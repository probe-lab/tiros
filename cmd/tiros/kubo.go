package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	ipfs "github.com/ipfs/kubo"
	kuboclient "github.com/ipfs/kubo/client/rpc"
	"github.com/ipfs/kubo/core/commands"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/multiformats/go-multicodec"
	pllog "github.com/probe-lab/go-commons/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type KuboConfig struct {
	Host string
	Port int
}

type Kubo struct {
	*kuboclient.HttpApi
	cfg      *KuboConfig
	addr     string
	receiver *TraceReceiver
	tracer   trace.Tracer
}

func NewKubo(receiver *TraceReceiver, cfg *KuboConfig) (*Kubo, error) {
	provider := sdktrace.NewTracerProvider()
	tracer := provider.Tracer("Tiros")

	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithPropagators(propagator),
		),
	}

	// initializing the kubo client
	kuboAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	kuboClient, err := kuboclient.NewURLApiWithClient(kuboAddr, httpClient)
	if err != nil {
		return nil, fmt.Errorf("init kubo client: %w", err)
	}

	return &Kubo{HttpApi: kuboClient, addr: kuboAddr, receiver: receiver, tracer: tracer}, nil
}

func (k *Kubo) WaitAvailable(ctx context.Context, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for {
		select {
		case <-timeoutCtx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timeout waiting for Kubo to be ready: %w", ctx.Err())
			}
			return ctx.Err()
		case <-time.After(time.Second):
			slog.With("addr", k.addr).Info("Testing Kubo availability...")
			v, err := k.Version(ctx)
			if err != nil {
				continue
			}
			slog.Info("Kubo is online!", "version", v.Version)
			return nil
		}
	}
}

func (k *Kubo) Version(ctx context.Context) (*ipfs.VersionInfo, error) {
	res, err := k.Request("version").Send(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := io.ReadAll(res.Output)
	if err != nil {
		return nil, err
	}

	info := &ipfs.VersionInfo{}
	return info, json.Unmarshal(data, info)
}

func (k *Kubo) ID(ctx context.Context) (*commands.IdOutput, error) {
	var out commands.IdOutput
	if err := k.Request("id").Exec(ctx, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (k *Kubo) Reset(ctx context.Context) {
	pinsChan := make(chan iface.Pin)

	go func() {
		if err := k.Pin().Ls(ctx, pinsChan); err != nil {
			slog.With("err", err).Warn("Error getting pins")
		}
	}()

	for pin := range pinsChan {
		if pin.Type() != "recursive" && pin.Type() != "direct" {
			continue
		}

		slog.With("pin", pin.Path()).Info("Unpinning file from Kubo")
		if err := k.Pin().Rm(ctx, pin.Path()); err != nil {
			slog.With("err", err, "pin", pin.Path()).Warn("Error unpinning file from Kubo")
		}
	}

	slog.Info("Running repo garbage collection")
	if _, err := k.Request("repo/gc").Send(ctx); err != nil {
		slog.With("err", err).Warn("Error running ipfs gc")
	}
}

func (k *Kubo) Upload(ctx context.Context, fileSizeMiB int) (*UploadResult, error) {
	// Generate random data
	size := probeKuboConfig.FileSizeMiB * 1024 * 1024
	data := make([]byte, size)
	rand.Read(data)

	uploadCtx, uploadCancel := context.WithTimeout(ctx, time.Minute)
	uploadCtx, uploadSpan := k.tracer.Start(uploadCtx, "Upload")
	logEntry := slog.With(
		"size", size,
		"traceID", uploadSpan.SpanContext().TraceID().String(),
	)

	// take the trace receiver lock before uploading the file
	// so that we won't miss any trace data
	k.receiver.mu.Lock()

	logEntry.Info("Adding file to Kubo")
	imPath, err := k.Unixfs().Add(
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
	k.receiver.traceMatchers = []TraceMatcher{
		traceIDMatcher(uploadSpan.SpanContext().TraceID()),
		strAttrMatcher("key", rawCID.String()),
	}
	k.receiver.mu.Unlock()

	// if an error occurred, log it and continue with the next iteration
	if err != nil {
		return nil, fmt.Errorf("add file to kubo: %w", err)
	}

	result := &UploadResult{
		CID:            imPath.RootCid(),
		RawCID:         rawCID,
		IPFSAddTraceID: uploadSpan.SpanContext().TraceID(),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	logEntry.Info("Waiting for trace data...")
	parseTimeout := time.NewTimer(time.Minute)
loop:
	for {
		select {
		case <-parseTimeout.C:
			break loop
		case <-ctx.Done():
			return nil, ctx.Err()
		case req, more := <-k.receiver.traceMatchChan:
			if !more {
				return nil, errors.New("trace receiver closed")
			}
			result.parse(req)
			if result.isPopulated() {
				break loop
			}
		}
	}
	parseTimeout.Stop()

	k.receiver.Reset()

	return result, nil
}

func (k *Kubo) Download(ctx context.Context, c cid.Cid) (*DownloadResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)

	ctx, downloadSpan := k.tracer.Start(ctx, "Download")
	defer downloadSpan.End()

	traceID := downloadSpan.SpanContext().TraceID()

	logEntry := slog.With(
		"cid", c.String(),
		"traceID", traceID.String(),
	)

	k.receiver.mu.Lock()
	k.receiver.traceMatchers = []TraceMatcher{
		traceIDMatcher(traceID),
		nameMatcher("ProviderQueryManager.FindProvidersAsync"),
		nameMatcher("DelegatedHTTPClient.FindProviders"),
	}
	k.receiver.mu.Unlock()
	defer k.receiver.Reset()

	result := &DownloadResult{
		CID:            c,
		IPFSCatStart:   time.Now(),
		IPFSCatTraceID: traceID,
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	parseTimeout := time.NewTimer(45 * time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		logEntry.With("timeout", 45*time.Second).Info("Subscribing to trace data...")
		for {
			select {
			case <-parseTimeout.C:
				return
			case <-ctx.Done():
				return
			case req, more := <-k.receiver.traceMatchChan:
				if !more {
					return
				}
				result.parse(req)
				if result.isPopulated() {
					return
				}
			}
		}
	}()

	defer func() {
		cancel()
		<-done
		if result.IPFSCatEnd.IsZero() {
			result.IPFSCatEnd = time.Now()
		}
	}()

	logEntry.Info("Downloading file from Kubo")
	resp, err := k.Request("cat", c.String()).Send(ctx)
	if err != nil {
		return result, err
	} else if resp.Error != nil {
		return result, resp.Error
	}

	defer pllog.Defer(resp.Output.Close, "Failed closing response output")

	var buf [1]byte
	_, err = resp.Output.Read(buf[:])
	if err != nil {
		return result, err
	}
	logEntry.Info("Read first byte")
	ttfb := time.Since(result.IPFSCatStart)

	r := io.LimitReader(resp.Output, 100*1024*1024) // read at most 20 MiB
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	downloadEnd := time.Now()

	data = append(data, buf[:]...)
	downloadSpan.End()

	logEntry.With("size", len(data)).Info("Read all data")

	logEntry.Info("Waiting for trace data...")
	parseTimeout.Reset(12 * time.Second) // traces are submitted every 10 seconds, we wait a little longer

	<-done // will be closed when context is canceled or timeout is reached

	result.IPFSCatEnd = downloadEnd
	result.IPFSCatTTFB = ttfb
	result.FileSize = len(data)

	return result, nil
}
