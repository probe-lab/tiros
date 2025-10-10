package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/multiformats/go-multicodec"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/urfave/cli/v3"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	p "golang.org/x/exp/apidiff/testdata"
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
}{
	FileSizeMiB:   100,
	Interval:      time.Minute,
	KuboHost:      "127.0.0.1",
	KuboAPIPort:   5001,
	TraceRecHost:  "127.0.0.1",
	TraceRecPort:  4317,
	MaxIterations: 0,
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
	tr, err := NewTraceReceiver(probeKuboConfig.TraceRecHost, probeKuboConfig.TraceRecPort)
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

	kubo, err := NewKubo(probeKuboConfig.KuboHost, probeKuboConfig.KuboAPIPort)
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

	provider := sdktrace.NewTracerProvider()
	tracer := provider.Tracer("Tiros")

	// ticker to control the interval between iterations
	ticker := time.NewTimer(0)

	// start of the respective iteration
	iterationStart := time.Now()

	maxIter := probeKuboConfig.MaxIterations
	for i := 0; maxIter == 0 || i < maxIter; i++ {
		slog.Info("")

		// remove all pins and run a repo garbage collection
		kubo.Reset(ctx)

		// log the time until the next iteration
		waitTime := time.Until(iterationStart.Add(probeKuboConfig.Interval)).Truncate(time.Second)
		if i > 0 && waitTime > 0 {
			slog.With("iteration", i).Info(fmt.Sprintf("Waiting %s until the next iteration...", waitTime))
		}

		// wait for the next iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// continue
		}

		// start of the respective iteration and keep track of the start timestamp
		iterationStart = time.Now()

		// Generate random data
		size := probeKuboConfig.FileSizeMiB * 1024 * 1024
		data := make([]byte, size)
		rand.Read(data)

		uploadCtx, uploadCancel := context.WithTimeout(ctx, time.Minute)

		uploadCtx, uploadSpan := tracer.Start(uploadCtx, "Upload")
		logEntry := slog.With(
			"iteration", i,
			"size", size,
			"traceID", uploadSpan.SpanContext().TraceID().String(),
		)

		// take the trace receiver lock before uploading the file
		// so that we won't miss any trace data
		tr.matchersMu.Lock()

		logEntry.Info("Adding file to Kubo")
		imPath, err := kubo.Unixfs().Add(
			uploadCtx,
			files.NewBytesFile(data),
			options.Unixfs.Pin(true, uuid.NewString()),
			options.Unixfs.FsCache(false),
		)
		uploadSpan.RecordError(err)
		uploadSpan.End()
		uploadCancel()
		logEntry = logEntry.With("cid", imPath.RootCid().String())
		logEntry.Info("Done adding file to Kubo")

		// register trace ID as well as the CID of the uploaded file

		rawCID := cid.NewCidV1(uint64(multicodec.Raw), imPath.RootCid().Hash())
		tr.traceMatchers = []TraceMatcher{
			traceIDMatcher(uploadSpan.SpanContext().TraceID()),
			strAttrMatcher("key", rawCID.String()),
		}
		tr.matchersMu.Unlock()

		// reset ticker interval after operation
		ticker.Reset(probeKuboConfig.Interval)

		// if an error occurred, log it and continue with the next iteration
		if err != nil {
			logEntry.With("err", err).Warn("Error adding file to Kubo")
			continue
		}

		var (
			ipfsAddMetrics *ipfsAddMetrics
			provideMetrics *provideMetrics
		)

		logEntry.Info("Waiting for trace data...")
		parseTimeout := time.NewTimer(time.Minute)
	loop:
		for {
			var matchRes *TraceMatch
			select {
			case <-parseTimeout.C:
				break loop
			case <-ctx.Done():
				return nil
			case matchRes = <-tr.traceMatchChan:
				// continue
			}

			if matchRes.matcherIdx == 0 {
				logEntry.Info("Received `ipfs add` trace data")
				ipfsAddMetrics = parseIPFSAddTrace(matchRes.req)
			} else if matchRes.matcherIdx == 1 {
				logEntry.Info("Received provide trace data")
				provideMetrics = parseProvideTrace(matchRes.req)
			} else {
				panic("invalid matcher index")
			}

			if ipfsAddMetrics != nil && provideMetrics != nil {
				break
			}
		}
		parseTimeout.Stop()

		tr.Reset()

		if ipfsAddMetrics == nil && provideMetrics == nil {
			logEntry.Warn("Failed to parse trace data")
			continue
		}

		logEntry.Info(fmt.Sprintf("Upload finished in %s", provideMetrics.end.Sub(ipfsAddMetrics.start)))

		provideDelay := provideMetrics.start.Sub(ipfsAddMetrics.end)
		uploadDuration := provideMetrics.end.Sub(ipfsAddMetrics.start)

		dbUpload := &UploadModel{
			Region:            rootConfig.AWSRegion,
			TirosVersion:      rootConfig.BuildInfo.ShortCommit(),
			KuboVersion:       kuboVersion.Version,
			FileSizeMiB:       int32(probeKuboConfig.FileSizeMiB),
			IPFSAddStart:      ipfsAddMetrics.start,
			IPFSAddDurationMs: int32(ipfsAddMetrics.duration.Milliseconds()),
			ProvideStart:      provideMetrics.start,
			ProvideDurationMs: int32(provideMetrics.duration.Milliseconds()),
			ProvideDelayMs:    int32(provideDelay.Milliseconds()),
			UploadDurationMs:  int32(uploadDuration.Milliseconds()),
		}
		if err := dbClient.InsertUpload(ctx, dbUpload); err != nil {
			return fmt.Errorf("inserting upload into database: %w", err)
		}

		////////////////// DOWNLOAD ///////////////////////////
		download(ctx, kubo, tracer, tr)

	}

	return nil
}

type downloadData struct{}

func download(ctx context.Context, kubo *Kubo, tracer trace.Tracer, tr *TraceReceiver) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ctx, downloadSpan := tracer.Start(ctx, "Download")
	defer downloadSpan.End()

	tr.matchersMu.Lock()
	tr.traceMatchers = []TraceMatcher{traceIDMatcher(downloadSpan.SpanContext().TraceID())}
	tr.matchersMu.Unlock()

	downloadStart := time.Now()
	resp, err := kubo.Request("cat", p.String()).Send(ctx)
	if err != nil {
		return err
	}
	defer pllog.Defer(resp.Output.Close, "Failed closing response output")

	var buf [1]byte
	_, err = resp.Output.Read(buf[:])
	if err != nil {
		return err
	}
	downloadTTFB := time.Since(downloadStart)

	data, err := io.ReadAll(resp.Output)
	if err != nil {
		return err
	}

	data = append(data, buf[:]...)

	return nil
}
