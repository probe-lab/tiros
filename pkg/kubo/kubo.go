package kubo

import (
	"bytes"
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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	ipfs "github.com/ipfs/kubo"
	kuboclient "github.com/ipfs/kubo/client/rpc"
	"github.com/ipfs/kubo/core/commands"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multicodec"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/probe-lab/go-commons/ptr"
	"github.com/probe-lab/tiros/pkg/db"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/sync/errgroup"
)

type KuboConfig struct {
	Host           string
	APIPort        int
	GWPort         int
	Receiver       *TraceReceiver
	ChromeKuboHost string
	FileSizeMiB    int
}

type Kubo struct {
	*kuboclient.HttpApi
	cfg    *KuboConfig
	addr   string
	tracer trace.Tracer
}

func NewKubo(cfg *KuboConfig) (*Kubo, error) {
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
	kuboAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.APIPort))
	kuboClient, err := kuboclient.NewURLApiWithClient(kuboAddr, httpClient)
	if err != nil {
		return nil, fmt.Errorf("init kubo client: %w", err)
	}

	return &Kubo{HttpApi: kuboClient, cfg: cfg, addr: kuboAddr, tracer: tracer}, nil
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
	res, err := k.Request("repo/gc").Send(ctx)
	if err != nil {
		slog.With("err", err).Warn("Error running ipfs gc")
	} else {
		defer pllog.Defer(res.Close, "Failed closing repo garbage collection")
	}
}

func (k *Kubo) Upload(ctx context.Context) (*UploadResult, error) {
	slog.Info(fmt.Sprintf("Uploading %dMiB to Kubo", k.cfg.FileSizeMiB))

	// Generate random data
	size := k.cfg.FileSizeMiB * 1024 * 1024
	data := make([]byte, size)
	rand.Read(data)

	// Determine root CID of the random data blob
	rootCID, err := k.GetCID(ctx, files.NewBytesFile(data))
	if err != nil {
		return nil, fmt.Errorf("determine root CID: %w", err)
	}

	// acquire lock here to prevent that the upload start is delayed by the lock
	// acquisition (which is measured by the trace start below.
	k.cfg.Receiver.mu.Lock()

	// Start a trace span to get a traceID to match traces for
	uploadCtx, uploadCancel := context.WithTimeout(ctx, time.Minute)
	defer uploadCancel()

	uploadCtx, uploadSpan := k.tracer.Start(uploadCtx, "Upload")
	slog.Info("Determined root CID of random data: " + rootCID.String())

	// take the trace receiver lock before uploading the file
	// so that we won't miss any trace data
	k.cfg.Receiver.traceMatchers = []TraceMatcher{
		traceIDMatcher(uploadSpan.SpanContext().TraceID()),
	}
	k.cfg.Receiver.mu.Unlock()
	defer k.cfg.Receiver.Reset()

	// initialize the upload result
	result := &UploadResult{
		CID:            rootCID,
		RawCID:         cid.NewCidV1(uint64(multicodec.Raw), rootCID.Hash()),
		IPFSAddTraceID: uploadSpan.SpanContext().TraceID(),
	}

	// start listening for trace events

	parseTimeout := time.NewTimer(time.Minute)
	defer parseTimeout.Stop()
	errg, ectx := errgroup.WithContext(ctx)
	errg.Go(func() error {
		slog.Info("Waiting for trace data...")
		for {
			select {
			case <-parseTimeout.C:
				return context.DeadlineExceeded
			case <-ectx.Done():
				return ectx.Err()
			case req, more := <-k.cfg.Receiver.traceMatchChan:
				if !more {
					return errors.New("trace receiver closed")
				}

				slog.Info("Received relevant traces from Kubo...")
				result.parse(req)
				if result.isPopulated() {
					return nil
				}
			}
		}
	})

	slog.With("sizeMiB", k.cfg.FileSizeMiB, "traceID", uploadSpan.SpanContext().TraceID().String()).Info("Adding file to Kubo")

	uploadStart := time.Now()
	rootCID, err = k.Add(uploadCtx, files.NewBytesFile(data))
	uploadEnd := time.Now()

	uploadSpan.RecordError(err) // noop if err is nil
	uploadSpan.End()

	slog.Info("Done adding file to Kubo")

	// if an error occurred, log it and continue with the next iteration
	if err != nil {
		parseTimeout.Stop()
		if err2 := errg.Wait(); !errors.Is(err2, context.DeadlineExceeded) {
			slog.Warn("Failed to wait for traces", "err", err2)
		}

		result.UploadStart = uploadStart
		result.UploadEnd = uploadEnd

		return result, fmt.Errorf("add file to kubo: %w", err)
	}

	// after the upload has finished, wait the most 30s for all traces to arrive
	parseTimeout.Reset(30 * time.Second)

	errgErr := errg.Wait()

	result.UploadStart = uploadStart
	result.UploadEnd = uploadEnd

	// wait for all traces to have been parsed
	return result, errgErr
}

func (k *Kubo) Download(ctx context.Context, c cid.Cid) (*DownloadResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)

	ctx, downloadSpan := k.tracer.Start(ctx, "Download")
	defer downloadSpan.End()

	traceID := downloadSpan.SpanContext().TraceID()

	logEntry := slog.With(
		"cid", c.String(),
		"traceID", traceID.String(),
	)

	k.cfg.Receiver.mu.Lock()
	k.cfg.Receiver.traceMatchers = []TraceMatcher{
		traceIDMatcher(traceID),
	}
	k.cfg.Receiver.mu.Unlock()
	defer k.cfg.Receiver.Reset()

	result := &DownloadResult{
		CID:             c,
		IPFSCatStart:    time.Now(),
		IPFSCatTraceID:  traceID,
		DiscoveryMethod: "",
		spansByTraceID:  map[trace.TraceID][]*v1.Span{},
	}

	parseTimeout := time.NewTimer(30 * time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		logEntry.With("timeout", 30*time.Second).Info("Subscribing to trace data...")
		for {
			select {
			case <-parseTimeout.C:
				return
			case <-ctx.Done():
				return
			case req, more := <-k.cfg.Receiver.traceMatchChan:
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
	catCtx, catCancel := context.WithTimeout(ctx, 10*time.Second)
	defer catCancel()
	resp, err := k.Request("cat", c.String()).Send(catCtx)
	if err != nil {
		return result, err
	} else if resp.Error != nil {
		return result, resp.Error
	}

	defer pllog.Defer(resp.Close, "Failed closing response output")

	var buf [1]byte
	_, err = resp.Output.Read(buf[:])
	if err != nil {
		return result, err
	}
	logEntry.Info("Read first byte")
	ttfb := time.Since(result.IPFSCatStart)

	r := io.LimitReader(resp.Output, 100*1024*1024) // read at most 100 MiB
	data, err := io.ReadAll(r)
	if err != nil {
		return result, err
	}
	downloadEnd := time.Now()

	data = append(buf[:], data...)
	downloadSpan.End()

	logEntry.With("size", len(data)).Info("Read all data")

	logEntry.Info("Waiting for trace data...")
	parseTimeout.Reset(12 * time.Second) // traces are submitted every 10 seconds, we wait a little longer

	<-done // will be closed when context is canceled or timeout is reached

	result.IPFSCatEnd = downloadEnd
	result.IPFSCatTTFB = ttfb
	result.FileSize = len(data)
	result.MIMEType = mimetype.Detect(data).String()

	// the FirstBlockReceivedAt field is only used to determine
	// the discovery method. This field will only be set though,
	// when https://github.com/ipfs/boxo/pull/1053 is merged.
	// Until then, we assume ttfb == FirstBlockReceivedAt.
	if result.DiscoveryMethod == "unknown" {
		if result.IPFSCatStart.Add(ttfb).Before(result.FirstProviderConnectedAt) || result.FirstProviderConnectedAt.IsZero() {
			result.DiscoveryMethod = "bitswap"
		}
	}

	return result, nil
}

type Provider struct {
	Website   string
	Path      string
	ID        peer.ID
	Maddrs    []multiaddr.Multiaddr
	Agent     *string
	Err       error
	IsRelayed *bool
}

func (k *Kubo) FindProviders(ctx context.Context, website string, results chan<- *Provider) error {
	logEntry := slog.With("website", website)
	logEntry.Info("Finding providers for " + website)

	nameResp, err := k.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return fmt.Errorf("name/resolve: %w", err)
	} else if nameResp.Error != nil {
		return fmt.Errorf("name/resolve: %w", nameResp.Error)
	} else if nameResp == nil {
		return fmt.Errorf("name/resolve no error but response nil")
	} else if nameResp.Output == nil {
		return fmt.Errorf("name/resolve no error but response output nil")
	}
	defer pllog.Defer(nameResp.Output.Close, "Failed closing name/resolve response output")

	dat, err := io.ReadAll(nameResp.Output)
	if err != nil {
		return fmt.Errorf("read name/resolve bytes: %w", err)
	}

	type nameResolveResponse struct {
		Path string
	}

	nrr := nameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	findResp, err := k.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "1000").
		Send(ctx)
	if err != nil {
		return fmt.Errorf("routing/findprovs: %w", err)
	} else if findResp.Error != nil {
		return fmt.Errorf("routing/findprovs: %w", findResp.Error)
	} else if findResp == nil {
		return fmt.Errorf("routing/findprovs no error but response nil")
	} else if findResp.Output == nil {
		return fmt.Errorf("routing/findprovs no error but response output nil")
	}
	defer pllog.Defer(findResp.Output.Close, "Error closing name/resolve response")

	var providerPeers []*peer.AddrInfo
	dec := json.NewDecoder(findResp.Output)
	for dec.More() {
		evt := routing.QueryEvent{}
		if err = dec.Decode(&evt); err != nil {
			return fmt.Errorf("decode routing/findprovs response: %w", err)
		}

		if evt.Type != routing.Provider {
			continue
		}

		if len(evt.Responses) != 1 {
			logEntry.Warn("findprovs Providerquery event with != 1 responses", "actual", len(evt.Responses))
			continue
		}

		providerPeers = append(providerPeers, evt.Responses[0])
	}

	type idResult struct {
		peer *peer.AddrInfo
		id   *commands.IdOutput
		err  error
	}

	numJobs := len(providerPeers)
	idJobs := make(chan *peer.AddrInfo, numJobs)
	idResults := make(chan idResult, numJobs)

	for w := 0; w < 10; w++ {
		go func() {
			for j := range idJobs {
				tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				id, err := k.ID(tCtx)
				cancel()

				idResults <- idResult{
					peer: j,
					id:   id,
					err:  err,
				}
			}
		}()
	}

	for _, providerPeer := range providerPeers {
		idJobs <- providerPeer
	}
	close(idJobs)

	for i := 0; i < numJobs; i++ {
		idr := <-idResults

		prov := &Provider{
			Website: website,
			Path:    nrr.Path,
			ID:      idr.peer.ID,
			Maddrs:  idr.peer.Addrs,
		}

		if idr.err != nil {
			prov.Err = idr.err
		} else {
			prov.Agent = ptr.From(idr.id.AgentVersion)
			if len(idr.id.Addresses) != len(idr.peer.Addrs) && len(idr.id.Addresses) != 0 {
				newAddrs := make([]multiaddr.Multiaddr, len(idr.id.Addresses))
				for j, addr := range idr.id.Addresses {
					newAddrs[j] = multiaddr.StringCast(addr)
				}
				prov.Maddrs = newAddrs
			}
		}

		prov.IsRelayed = isRelayed(prov.Maddrs)

		results <- prov
	}

	return nil
}

func (k *Kubo) WebsiteURL(website string, protocol db.WebsiteProbeProtocol) string {
	switch protocol {
	case db.WebsiteProbeProtocolIPFS:
		return fmt.Sprintf("http://%s:%d/ipns/%s", k.cfg.ChromeKuboHost, k.cfg.GWPort, website)
	case db.WebsiteProbeProtocolHTTP:
		return fmt.Sprintf("https://%s", website)
	default:
		panic(fmt.Sprintf("unknown probe type: %s", protocol))
	}
}

func isRelayed(maddrs []multiaddr.Multiaddr) *bool {
	if len(maddrs) == 0 {
		return nil
	}

	for _, maddr := range maddrs {
		if manet.IsPrivateAddr(maddr) {
			continue
		}

		if _, err := maddr.ValueForProtocol(multiaddr.P_CIRCUIT); err != nil {
			out := false
			return &out
		}
	}
	out := true
	return &out
}

func (k *Kubo) Add(ctx context.Context, body io.Reader) (cid.Cid, error) {
	resp, err := k.Request("add").
		Option("pin", true).
		Option("fast-provide-wait", true).
		Option("fast-provide-root", true).
		Option("fscache", false).
		FileBody(body).
		Send(ctx)
	if err != nil {
		return cid.Undef, err
	}

	if resp.Error != nil {
		return cid.Undef, err
	}
	defer pllog.Defer(resp.Close, "Failed closing response output")

	var evt commands.AddEvent
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		if err := dec.Decode(&evt); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return cid.Undef, err
		}
	}

	c, err := cid.Decode(evt.Hash)
	if err != nil {
		return cid.Undef, err
	}

	return c, ctx.Err()
}

func (k *Kubo) GetCID(ctx context.Context, body io.Reader) (cid.Cid, error) {
	resp, err := k.Request("add").
		Option("only-hash", true).
		FileBody(body).
		Send(ctx)
	if err != nil {
		return cid.Undef, err
	}

	if resp.Error != nil {
		return cid.Undef, err
	}
	defer pllog.Defer(resp.Close, "Failed closing response output")

	var evt commands.AddEvent
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		if err := dec.Decode(&evt); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return cid.Undef, err
		}
	}

	return cid.Decode(evt.Hash)
}

func looksLikeJSON(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}

	if !utf8.Valid(data) {
		return false
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	s := strings.TrimSpace(string(data))
	if s == "" {
		return false
	}

	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}

	return true
}
