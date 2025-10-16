package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
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
		Name:        "kubo.apiPort",
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
		Usage:       "The host that the trace receiver is binding to (this is where Kubo should send the traces to)",
		Sources:     cli.EnvVars("TIROS_PROBE_KUBO_TRACES_RECEIVER_HOST"),
		Value:       probeKuboConfig.TracesRecHost,
		Destination: &probeKuboConfig.TracesRecHost,
	},
	&cli.IntFlag{
		Name:        "traces.receiver.port",
		Usage:       "The port on which the trace receiver should listen on (this is where Kubo should send the traces to)",
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

	runID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("creating run id: %w", err)
	}

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

	// initializing the db client
	dbClient, err := newDBClient(ctx)
	if err != nil {
		return fmt.Errorf("creating database client: %w", err)
	}
	defer pllog.Defer(dbClient.Close, "Failed closing database client")

	var cidProvider CIDProvider
	cidProvider, err = NewBitswapSnifferClickhouseCIDProvider(dbClient)
	if err != nil {
		return fmt.Errorf("creating cid provider: %w", err)
	}

	kuboCfg := &KuboConfig{
		Host:     probeKuboConfig.KuboHost,
		APIPort:  probeKuboConfig.KuboAPIPort,
		Receiver: tr,
	}
	kubo, err := NewKubo(kuboCfg)
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

			ur, err := kubo.Upload(ctx, probeKuboConfig.FileSizeMiB)

			cidStr := ""
			if ur.CID.Defined() {
				cidStr = ur.CID.String()
			}

			dbUpload := &UploadModel{
				RunID:            runID.String(),
				Region:           rootConfig.AWSRegion,
				TirosVersion:     rootConfig.BuildInfo.ShortCommit(),
				KuboVersion:      kuboVersion.Version,
				KuboPeerID:       kuboID.ID,
				FileSizeB:        int32(probeKuboConfig.FileSizeMiB * 1024 * 1024),
				CID:              toPtr(cidStr),
				IPFSAddStart:     ur.IPFSAddStart,
				IPFSAddDurationS: ur.IPFSAddEnd.Sub(ur.IPFSAddStart).Seconds(),
				ProvideStart:     toPtr(ur.ProvideStart),
				ProvideDurationS: toPtr(ur.ProvideEnd.Sub(ur.ProvideStart).Seconds()),
				ProvideDelayS:    toPtr(ur.ProvideStart.Sub(ur.IPFSAddEnd).Seconds()),
				UploadDurationS:  toPtr(ur.ProvideEnd.Sub(ur.IPFSAddStart).Seconds()),
			}

			if err != nil {
				slog.With("err", err).Warn("Error uploading file to Kubo")
				dbUpload.Error = toPtr(err.Error())
			} else {
				slog.Info(fmt.Sprintf("Upload finished in %s", ur.ProvideEnd.Sub(ur.IPFSAddStart)))
			}
			if err := dbClient.InsertUpload(ctx, dbUpload); err != nil {
				return fmt.Errorf("inserting upload into database: %w", err)
			}
		}

		if !probeKuboConfig.UploadOnly {
			for _, origin := range []string{"bitswap", "dht"} {
				slog.Info(strings.Repeat("-", 80))

				slog.With("origin", origin).Info("Starting download measurement")
				ciid, err := cidProvider.SelectCID(ctx, origin)
				if errors.Is(err, sql.ErrNoRows) {
					slog.With("origin", origin).Info("No CID found in database")
					continue
				} else if err != nil {
					return fmt.Errorf("selecting cid from database: %w", err)
				}

				dr, err := kubo.Download(ctx, ciid)
				dbDownload := &DownloadModel{
					RunID:                runID.String(),
					Region:               rootConfig.AWSRegion,
					TirosVersion:         rootConfig.BuildInfo.ShortCommit(),
					KuboVersion:          kuboVersion.Version,
					KuboPeerID:           kuboID.ID,
					FileSizeB:            int32(dr.FileSize),
					CID:                  ciid.String(),
					IPFSCatStart:         dr.IPFSCatStart,
					IPFSCatDurationS:     dr.IPFSCatEnd.Sub(dr.IPFSCatStart).Seconds(),
					IPFSCatTTFBS:         toPtr(dr.IPFSCatTTFB.Seconds()),
					IdleBroadcastStart:   toPtr(dr.IdleBroadcastStartedAt),
					FoundProvCount:       dr.FoundProvidersCount,
					ConnProvCount:        dr.ConnectedProvidersCount,
					FirstConnProvFoundAt: toPtr(dr.FirstConnectedProviderFoundAt),
					FirstProvConnAt:      toPtr(dr.FirstProviderConnectedAt),
					FirstProvPeerID:      toPtr(dr.FirstConnectedProviderPeerID),
					IPNIStart:            toPtr(dr.IPNIStart),
					IPNIDurationS:        toPtr(dr.IPNIEnd.Sub(dr.IPNIStart).Seconds()),
					IPNIStatus:           toPtr(dr.IPNIStatus),
					FirstBlockReceivedAt: toPtr(dr.FirstBlockReceivedAt),
					DiscoveryMethod:      toPtr(dr.DiscoveryMethod),
					CIDSource:            "bitsniffer_" + origin,
				}
				if err != nil {
					slog.With("err", err).Warn("Error downloading file from Kubo")
					dbDownload.Error = toPtr(err.Error())
				} else {
					slog.With("discovery", dr.DiscoveryMethod).Info(fmt.Sprintf("Download finished in %s", dr.IPFSCatEnd.Sub(dr.IPFSCatStart)))
				}

				if err := dbClient.InsertDownload(ctx, dbDownload); err != nil {
					return fmt.Errorf("inserting upload into database: %w", err)
				}
			}
		}
	}

	return nil
}

func toPtr[T comparable](t T) *T {
	if t == *new(T) {
		return nil
	}
	return &t
}
