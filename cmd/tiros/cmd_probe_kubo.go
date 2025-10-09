package main

import (
	"context"
	"embed"
	"fmt"
	"net"
	"strconv"
	"time"

	kuboclient "github.com/ipfs/kubo/client/rpc"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

//go:embed migrations
var migrations embed.FS

var probeKuboConfig = struct {
	FileSizeMiB int
	Interval    time.Duration
	KuboHost    string
	KuboAPIPort int
}{
	FileSizeMiB: 100,
	Interval:    time.Minute,
	KuboHost:    "127.0.0.1",
	KuboAPIPort: 5001,
}

var probeKuboFlags = []cli.Flag{
	&cli.IntFlag{
		Name:        "filesize",
		Usage:       "File size in MiB to upload to kubo",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_UPLOAD_FILE_SIZE_MIB"),
		Value:       probeKuboConfig.FileSizeMiB,
		Destination: &probeKuboConfig.FileSizeMiB,
	},
	&cli.DurationFlag{
		Name:        "interval",
		Usage:       "How long to wait between each upload",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_UPLOAD_INTERVAL"),
		Value:       probeKuboConfig.Interval,
		Destination: &probeKuboConfig.Interval,
	},
	&cli.StringFlag{
		Name:        "kubo.host",
		Usage:       "Host at which to reach Kubo",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_KUBO_HOST"),
		Value:       probeKuboConfig.KuboHost,
		Destination: &probeKuboConfig.KuboHost,
	},
	&cli.IntFlag{
		Name:        "ipfs-api-port",
		Usage:       "port to reach a Kubo-compatible RPC API",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_KUBO_API_PORT"),
		Value:       probeKuboConfig.KuboAPIPort,
		Destination: &probeKuboConfig.KuboAPIPort,
	},
}

var probeKuboCmd = &cli.Command{
	Name:   "kubo",
	Usage:  "Start probing Kubo",
	Flags:  probeKuboFlags,
	Action: probeKuboAction,
}

func probeKuboAction(ctx context.Context, c *cli.Command) error {
	chOpts := probeConfig.Clickhouse.Options()
	chClient, err := probeConfig.Clickhouse.OpenAndPing(ctx)
	if err != nil {
		return fmt.Errorf("connecting to clickhouse: %w", err)
	}
	defer pllog.Defer(chClient.Close, "Failed closing clickhouse client")

	if err = probeConfig.Migrations.Apply(chOpts, migrations); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	kuboClient, err := kuboclient.NewURLApiWithClient(net.JoinHostPort(probeKuboConfig.KuboHost, strconv.Itoa(probeKuboConfig.KuboAPIPort)), otelhttp.DefaultClient)
	if err != nil {
		return fmt.Errorf("init kubo client: %w", err)
	}

	_ = kuboClient

	<-ctx.Done()

	return nil
}
