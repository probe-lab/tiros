package main

import (
	"context"
	"embed"
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
	TracesRecHost string
	TracesRecPort int
	MaxIterations int
	TracesOut     string
	DownloadOnly  bool
	UploadOnly    bool

	TracesForwardHost string
	TracesForwardPort int
}{
	FileSizeMiB:       100,
	Interval:          10 * time.Second, // time.Minute,
	KuboHost:          "127.0.0.1",
	KuboAPIPort:       5001,
	TracesRecHost:     "127.0.0.1",
	TracesRecPort:     4317,
	MaxIterations:     0,
	TracesOut:         "",
	TracesForwardHost: "",
	TracesForwardPort: 0,
	DownloadOnly:      false,
	UploadOnly:        false,
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
	&cli.IntFlag{
		Name:        "maxIterations",
		Usage:       "The number of iterations to run. 0 means infinite.",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_MAX_ITERATIONS"),
		Value:       probeKuboConfig.MaxIterations,
		Destination: &probeKuboConfig.MaxIterations,
	},
	&cli.StringFlag{
		Name:        "traces.receiver.host",
		Usage:       "TODO",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_RECEIVER_HOST"),
		Value:       probeKuboConfig.TracesRecHost,
		Destination: &probeKuboConfig.TracesRecHost,
	},
	&cli.IntFlag{
		Name:        "traces.receiver.port",
		Usage:       "TODO",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_RECEIVER_PORT"),
		Value:       probeKuboConfig.TracesRecPort,
		Destination: &probeKuboConfig.TracesRecPort,
	},
	&cli.StringFlag{
		Name:        "traces.out",
		Usage:       "If set, where to write the traces to.",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_OUT"),
		Value:       probeKuboConfig.TracesOut,
		Destination: &probeKuboConfig.TracesOut,
	},
	&cli.StringFlag{
		Name:        "traces.forward.host",
		Usage:       "The host to forward Kubo's traces to.",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_FORWARD_HOST"),
		Value:       probeKuboConfig.TracesForwardHost,
		Destination: &probeKuboConfig.TracesForwardHost,
	},
	&cli.IntFlag{
		Name:        "traces.forward.port",
		Usage:       "The port to forward Kubo's traces to.",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_FORWARD_PORT"),
		Value:       probeKuboConfig.TracesForwardPort,
		Destination: &probeKuboConfig.TracesForwardPort,
	},
}

var probeKuboMuExFlags = []cli.MutuallyExclusiveFlags{
	{
		Flags: [][]cli.Flag{
			{
				&cli.BoolFlag{
					Name:        "download.only",
					Usage:       "Only download the file from Kubo",
					Sources:     cli.EnvVars("TIROS_PROBE_KUBO_DOWNLOAD_ONLY"),
					Value:       probeKuboConfig.DownloadOnly,
					Destination: &probeKuboConfig.DownloadOnly,
				},
				&cli.BoolFlag{
					Name:        "upload.only",
					Usage:       "Only download the file from Kubo",
					Sources:     cli.EnvVars("TIROS_PROBE_KUBO_DOWNLOAD_ONLY"),
					Value:       probeKuboConfig.DownloadOnly,
					Destination: &probeKuboConfig.DownloadOnly,
				},
			},
		},
	},
}

var probeKuboCmd = &cli.Command{
	Name:                   "kubo",
	Usage:                  "Start probing Kubo",
	Flags:                  probeKuboFlags,
	MutuallyExclusiveFlags: probeKuboMuExFlags,
	Action:                 probeKuboAction,
}

func probeKuboAction(ctx context.Context, c *cli.Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	trCfg := &TraceReceiverConfig{
		Host:        probeKuboConfig.TracesRecHost,
		Port:        probeKuboConfig.TracesRecPort,
		TraceOut:    probeKuboConfig.TracesOut,
		ForwardHost: probeKuboConfig.TracesForwardHost,
		ForwardPort: probeKuboConfig.TracesForwardPort,
	}

	// initializing the trace receiver
	tr, err := NewTraceReceiver(trCfg)
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

	var cidProvider CIDProvider
	cidProvider, err = NewBitswapSnifferClickhouseCIDProvider(dbClient)
	if err != nil {
		return fmt.Errorf("creating cid provider: %w", err)
	}

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

	kuboID, err := kubo.ID(ctx)
	if err != nil {
		return err
	}

	// ticker to control the interval between iterations
	ticker := time.NewTimer(0)

	// start of the respective iteration
	iterationStart := time.Now()

	maxIter := probeKuboConfig.MaxIterations
	for i := 0; maxIter == 0 || i < maxIter; i++ {
		slog.Info(strings.Repeat("-", 80))

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

		if !probeKuboConfig.DownloadOnly {
			slog.Info("Starting upload measurement")

			var (
				uploadStart = time.Now()
				errStr      string
			)
			ur, err := kubo.Upload(ctx, probeKuboConfig.FileSizeMiB)
			if err != nil {
				errStr = err.Error()
				slog.With("err", err).Warn("Error uploading file to Kubo")

				dbUpload := &UploadModel{
					Region:       rootConfig.AWSRegion,
					TirosVersion: rootConfig.BuildInfo.ShortCommit(),
					KuboVersion:  kuboVersion.Version,
					KuboPeerID:   kuboID.ID,
					FileSizeB:    int32(probeKuboConfig.FileSizeMiB * 1024 * 1024),
					CID:          ur.CID.String(),
					IPFSAddStart: uploadStart,
					Error:        errStr,
				}
				if err := dbClient.InsertUpload(ctx, dbUpload); err != nil {
					return fmt.Errorf("inserting upload into database: %w", err)
				}
			} else {
				slog.Info(fmt.Sprintf("Upload finished in %s", ur.ProvideEnd.Sub(ur.IPFSAddStart)))

				ipfsAddDuration := ur.IPFSAddEnd.Sub(ur.IPFSAddStart)
				provideDuration := ur.ProvideEnd.Sub(ur.ProvideStart)
				provideDelay := ur.ProvideStart.Sub(ur.IPFSAddEnd)
				uploadDuration := ur.ProvideEnd.Sub(ur.IPFSAddStart)

				dbUpload := &UploadModel{
					Region:            rootConfig.AWSRegion,
					TirosVersion:      rootConfig.BuildInfo.ShortCommit(),
					KuboVersion:       kuboVersion.Version,
					KuboPeerID:        kuboID.ID,
					FileSizeB:         int32(probeKuboConfig.FileSizeMiB * 1024 * 1024),
					CID:               ur.CID.String(),
					IPFSAddStart:      ur.IPFSAddStart,
					IPFSAddDurationMs: int32(ipfsAddDuration.Milliseconds()),
					ProvideStart:      ur.ProvideStart,
					ProvideDurationMs: int32(provideDuration.Milliseconds()),
					ProvideDelayMs:    int32(provideDelay.Milliseconds()),
					UploadDurationMs:  int32(uploadDuration.Milliseconds()),
					Error:             errStr,
				}
				if err := dbClient.InsertUpload(ctx, dbUpload); err != nil {
					return fmt.Errorf("inserting upload into database: %w", err)
				}
			}
		}

		if !probeKuboConfig.UploadOnly {
			for _, origin := range []string{"bitswap", "dht"} {
				slog.Info(strings.Repeat("-", 80))

				slog.With("origin", origin).Info("Starting download measurement")
				ciid, err := cidProvider.SelectCID(ctx, origin)
				if err != nil {
					return fmt.Errorf("selecting cid from database: %w", err)
				}

				var errStr string
				dr, err := kubo.Download(ctx, ciid)
				if err != nil {
					errStr = err.Error()
					slog.With("err", err).Warn("Error downloading file from Kubo")
				} else {
					slog.With("discovery", dr.DiscoveryMethod).Info(fmt.Sprintf("Download finished in %s", dr.IPFSCatEnd.Sub(dr.IPFSCatStart)))
				}

				ipfsCatDuration := dr.IPFSCatEnd.Sub(dr.IPFSCatStart)
				ipniDuration := dr.IPNIEnd.Sub(dr.IPNIStart)

				dbDownload := &DownloadModel{
					Region:               rootConfig.AWSRegion,
					TirosVersion:         rootConfig.BuildInfo.ShortCommit(),
					KuboVersion:          kuboVersion.Version,
					KuboPeerID:           kuboID.ID,
					FileSizeB:            int32(dr.FileSize),
					CID:                  ciid.String(),
					IPFSCatStart:         dr.IPFSCatStart,
					IPFSCatTTFBMs:        int32(dr.IPFSCatTTFB.Milliseconds()),
					IPFSCatDurationMs:    int32(ipfsCatDuration.Milliseconds()),
					IdleBroadcastStart:   dr.IdleBroadcastStartedAt,
					FoundProvCount:       dr.FoundProvidersCount,
					ConnProvCount:        dr.ConnectedProvidersCount,
					FirstConnProvFoundAt: dr.FirstConnectedProviderFoundAt,
					FirstProvConnAt:      dr.FirstProviderConnectedAt,
					FirstProvPeerID:      dr.FirstConnectedProviderPeerID,
					IPNIStart:            dr.IPNIStart,
					IPNIDurationMs:       int32(ipniDuration.Milliseconds()),
					IPNIStatus:           dr.IPNIStatus,
					FirstBlockReceivedAt: dr.FirstBlockReceivedAt,
					DiscoveryMethod:      dr.DiscoveryMethod,
					Error:                errStr,
				}
				if err := dbClient.InsertDownload(ctx, dbDownload); err != nil {
					return fmt.Errorf("inserting upload into database: %w", err)
				}
			}
		}

	}

	return nil
}
