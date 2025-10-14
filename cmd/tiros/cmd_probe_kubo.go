package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pllog "github.com/probe-lab/go-commons/log"
	"github.com/urfave/cli/v3"
)

//go:embed migrations
var migrations embed.FS

var probeKuboConfig = struct {
	FileSizeMiB   int
	Interval      time.Duration
	KuboHost      string
	KuboAPIPort   int
	TraceRecHost  string
	TraceRecPort  int
	MaxIterations int
	TracesOut     string
}{
	FileSizeMiB:   100,
	Interval:      10 * time.Second, // time.Minute,
	KuboHost:      "127.0.0.1",
	KuboAPIPort:   5001,
	TraceRecHost:  "127.0.0.1",
	TraceRecPort:  4317,
	MaxIterations: 0,
	TracesOut:     "",
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
	&cli.StringFlag{
		Name:        "traceReceiver.host",
		Usage:       "TODO",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACE_RECEIVER_HOST"),
		Value:       probeKuboConfig.TraceRecHost,
		Destination: &probeKuboConfig.TraceRecHost,
	},
	&cli.IntFlag{
		Name:        "traceReceiver.port",
		Usage:       "TODO",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACE_RECEIVER_PORT"),
		Value:       probeKuboConfig.TraceRecPort,
		Destination: &probeKuboConfig.TraceRecPort,
	},
	&cli.IntFlag{
		Name:        "maxIterations",
		Usage:       "TODO",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_MAX_ITERATIONS"),
		Value:       probeKuboConfig.MaxIterations,
		Destination: &probeKuboConfig.MaxIterations,
	},
	&cli.StringFlag{
		Name:        "traces.out",
		Usage:       "If set, where to write the traces to.",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_OUT"),
		Value:       probeKuboConfig.TracesOut,
		Destination: &probeKuboConfig.TracesOut,
	},
}

var probeKuboCmd = &cli.Command{
	Name:   "kubo",
	Usage:  "Start probing Kubo",
	Flags:  probeKuboFlags,
	Action: probeKuboAction,
}

func probeKuboAction(ctx context.Context, c *cli.Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// initializing the trace receiver
	tr, err := NewTraceReceiver(probeKuboConfig.TraceRecHost, probeKuboConfig.TraceRecPort, probeKuboConfig.TracesOut)
	if err != nil {
		return fmt.Errorf("creating trace receiver gRPC server: %w", err)
	}
	defer tr.Shutdown()

	// start to listen for incoming gRPC requests in a separate goroutine
	go func() {
		if err := tr.server.ListenAndServe(); err != nil {
			slog.Error("Failed to start trace receiver gRPC server", "err", err)
			// cancel the root context to stop the main
			// loop if the server fails to start
			cancel()
		}
	}()

	// initializing the clickhouse db client
	var dbClient DBClient
	if probeConfig.DryRun {
		dbClient = NewNoopClient()
	} else if probeConfig.JSONOut != "" {
		dbClient, err = NewJSONClient(probeConfig.JSONOut)
		if err != nil {
			return fmt.Errorf("connecting to json client: %w", err)
		}
	} else {
		dbClient, err = NewClickhouseClient(ctx, probeConfig.Clickhouse, probeConfig.Migrations)
		if err != nil {
			return fmt.Errorf("connecting to clickhouse: %w", err)
		}
	}
	defer pllog.Defer(dbClient.Close, "Failed closing database client")

	kuboCfg := &KuboConfig{
		Host: probeKuboConfig.KuboHost,
		Port: probeKuboConfig.KuboAPIPort,
	}
	kubo, err := NewKubo(tr, kuboCfg)
	if err != nil {
		return fmt.Errorf("creating kubo client: %w", err)
	}

	if err := kubo.WaitAvailable(ctx, time.Minute); err != nil {
		return err
	}

	kuboVersion, err := kubo.Version(ctx)
	if err != nil {
		return err
	}

	// ticker to control the interval between iterations
	ticker := time.NewTimer(0)

	// start of the respective iteration
	iterationStart := time.Now()

	maxIter := probeKuboConfig.MaxIterations
	for i := 0; maxIter == 0 || i < maxIter; i++ {
		slog.Info(strings.Repeat("-", 40))

		// remove all pins and run a repo garbage collection
		kubo.Reset(ctx)

		// log the time until the next iteration
		waitTime := time.Until(iterationStart.Add(probeKuboConfig.Interval)).Truncate(time.Second)
		if i > 0 {
			ticker.Reset(waitTime)
			if waitTime > 0 {
				slog.With("iteration", i).Info(fmt.Sprintf("Waiting %s until the next iteration...", waitTime))
			}
		}

		// wait for the next iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// pass
		}

		// start of the respective iteration and keep track of the start timestamp
		iterationStart = time.Now()

		ur, err := kubo.Upload(ctx, probeKuboConfig.FileSizeMiB)
		if errors.Is(err, context.Canceled) {
			return err
		} else if err != nil {
			slog.With("err", err).Warn("Error uploading file to Kubo")
			continue
		}

		// reset ticker interval after operation

		slog.Info(fmt.Sprintf("Upload finished in %s", ur.ProvideEnd.Sub(ur.IPFSAddStart)))

		ipfsAddDuration := ur.IPFSAddEnd.Sub(ur.IPFSAddStart)
		provideDuration := ur.ProvideEnd.Sub(ur.ProvideStart)
		provideDelay := ur.ProvideStart.Sub(ur.IPFSAddEnd)
		uploadDuration := ur.ProvideEnd.Sub(ur.IPFSAddStart)

		dbUpload := &UploadModel{
			Region:            rootConfig.AWSRegion,
			TirosVersion:      rootConfig.BuildInfo.ShortCommit(),
			KuboVersion:       kuboVersion.Version,
			FileSizeMiB:       int32(probeKuboConfig.FileSizeMiB),
			IPFSAddStart:      ur.IPFSAddStart,
			IPFSAddDurationMs: int32(ipfsAddDuration.Milliseconds()),
			ProvideStart:      ur.ProvideStart,
			ProvideDurationMs: int32(provideDuration.Milliseconds()),
			ProvideDelayMs:    int32(provideDelay.Milliseconds()),
			UploadDurationMs:  int32(uploadDuration.Milliseconds()),
		}
		if err := dbClient.InsertUpload(ctx, dbUpload); err != nil {
			return fmt.Errorf("inserting upload into database: %w", err)
		}

		////////////////// DOWNLOAD ///////////////////////////
		//download(ctx, kubo, tracer, tr)

		//ciid, err := dbClient.SelectCID(ctx)
		//if err != nil {
		//	return fmt.Errorf("selecting cid from database: %w", err)
		//}
		//
		//res, err := kubo.Download(ctx, ciid)
		//if err != nil {
		//	slog.With("err", err).Warn("Error downloading file from Kubo")
		//	continue
		//}
		//
		//slog.With(
		//	"fileSizeMiB", res.fileSize/1024.0/1024.0,
		//	"ipniStatusCode", res.clientMetrics,
		//).Info("Download Successful")

	}

	return nil
}
