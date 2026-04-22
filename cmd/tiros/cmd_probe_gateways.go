package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/probe-lab/tiros/pkg"
	"github.com/probe-lab/tiros/pkg/db"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"
)

var probeGatewaysConfig = struct {
	Interval        time.Duration
	MaxIterations   int
	DownloadCIDs    []string
	Gateways        []string
	MaxDownloadMB   int
	Timeout         time.Duration
	RefreshInterval time.Duration
	Concurrency     int
	ControlledCIDs  bool
	ControlledShare float32
}{
	Interval:        10 * time.Second,
	MaxIterations:   0,
	DownloadCIDs:    []string{},
	Gateways:        []string{},
	MaxDownloadMB:   10,
	Timeout:         30 * time.Second,
	RefreshInterval: 5 * time.Minute,
	Concurrency:     10,
	ControlledCIDs:  true,
	ControlledShare: 0.2,
}

var probeGatewaysFlags = []cli.Flag{
	&cli.DurationFlag{
		Name:        "interval",
		Usage:       "How long to wait between each download iteration",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_INTERVAL"),
		Value:       probeGatewaysConfig.Interval,
		Destination: &probeGatewaysConfig.Interval,
	},
	&cli.IntFlag{
		Name:        "iterations.max",
		Usage:       "The number of iterations per concurrent worker to run. 0 means infinite.",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_ITERATIONS_MAX"),
		Value:       probeGatewaysConfig.MaxIterations,
		Destination: &probeGatewaysConfig.MaxIterations,
	},
	&cli.StringSliceFlag{
		Name:        "cids",
		Usage:       "A static list of CIDs to download from the Gateways.",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CIDS"),
		Value:       probeGatewaysConfig.DownloadCIDs,
		Destination: &probeGatewaysConfig.DownloadCIDs,
	},
	&cli.StringSliceFlag{
		Name:        "gateways",
		Usage:       "A static list of gateways to probe (takes precedence over database)",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_GATEWAYS"),
		Value:       probeGatewaysConfig.Gateways,
		Destination: &probeGatewaysConfig.Gateways,
	},
	&cli.IntFlag{
		Name:        "download.max.mb",
		Usage:       "Maximum download size in MiB before cancelling",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_DOWNLOAD_MAX_MB"),
		Value:       probeGatewaysConfig.MaxDownloadMB,
		Destination: &probeGatewaysConfig.MaxDownloadMB,
	},
	&cli.DurationFlag{
		Name:        "timeout",
		Usage:       "Timeout for each gateway request",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_TIMEOUT"),
		Value:       probeGatewaysConfig.Timeout,
		Destination: &probeGatewaysConfig.Timeout,
	},
	&cli.DurationFlag{
		Name:        "refresh.interval",
		Usage:       "How frequently to refresh the gateway list from the database",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_REFRESH_INTERVAL"),
		Value:       probeGatewaysConfig.RefreshInterval,
		Destination: &probeGatewaysConfig.RefreshInterval,
	},
	&cli.IntFlag{
		Name:        "concurrency",
		Usage:       "Number of gateways to probe concurrently",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CONCURRENCY"),
		Value:       probeGatewaysConfig.Concurrency,
		Destination: &probeGatewaysConfig.Concurrency,
	},
	&cli.BoolFlag{
		Name:        "controlled.cids",
		Usage:       "Whether to use the ControlledCIDProvider to select CIDs to probe",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CONTROLLED_CIDS"),
		Value:       probeGatewaysConfig.ControlledCIDs,
		Destination: &probeGatewaysConfig.ControlledCIDs,
	},
	&cli.Float32Flag{
		Name:        "controlled.share",
		Usage:       "What share of requests should be made for controlled CIDs",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CONTROLLED_SHARE"),
		Value:       probeGatewaysConfig.ControlledShare,
		Destination: &probeGatewaysConfig.ControlledShare,
	},
}

var probeGatewaysCmd = &cli.Command{
	Name:   "gateways",
	Usage:  "Start probing IPFS Gateways retrieval performance",
	Flags:  probeGatewaysFlags,
	Action: probeGatewaysAction,
}

type gatewayMetrics struct {
	dnsStart      time.Time
	dnsDuration   time.Duration
	connStart     time.Time
	connDuration  time.Duration
	tlsStart      time.Time
	tlsDuration   time.Duration
	reqStart      time.Time
	ttfb          time.Duration
	downloadEnd   time.Time
	bytesReceived int64
	statusCode    int
	headers       http.Header
	carValidated  *bool
	redirectCount int
	finalURL      string
	redirectChain []string
	err           error
}

func probeGatewaysAction(ctx context.Context, cmd *cli.Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	meter := otel.GetMeterProvider().Meter("tiros")

	downloadCounter, err := meter.Int64Counter("gateway_downloads")
	if err != nil {
		return err
	}

	runID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("creating run id: %w", err)
	}

	// Initialize the db client
	dbClient, err := newDBClient(ctx)
	if err != nil {
		return fmt.Errorf("creating database client: %w", err)
	}
	defer pllog.Defer(dbClient.Close, "Failed closing database client")

	// Initialize CID provider
	var cidProvider pkg.CIDProvider
	var cidProviderName string
	if len(probeGatewaysConfig.DownloadCIDs) > 0 {
		cidProvider, err = pkg.NewStaticCIDProvider(probeGatewaysConfig.DownloadCIDs)
		if err != nil {
			return fmt.Errorf("creating static cid provider: %w", err)
		}
		cidProviderName = "StaticCIDProvider"
	} else {
		cidProvider, err = pkg.NewBitswapSnifferClickhouseCIDProvider(dbClient)
		if err != nil {
			return fmt.Errorf("creating clickhouse cid provider: %w", err)
		}
		cidProviderName = "BitswapSnifferClickhouseCIDProvider"
	}
	slog.With("provider", cidProviderName).Info("Using CID provider for gateway probes")

	controlledCIDsProvider, err := pkg.NewControlledCIDProvider()
	if err != nil {
		return fmt.Errorf("creating controlled cid provider: %w", err)
	}

	// Gateway list management
	var gatewaysMu sync.RWMutex
	var gateways []string

	// Check if static gateways are provided via command line
	if len(probeGatewaysConfig.Gateways) > 0 {
		gateways = probeGatewaysConfig.Gateways
		slog.With("count", len(gateways), "gateways", gateways).Info("Using static gateway list from command line")
	} else {

		// Function to update gateways from database
		updateGateways := func() error {
			newGateways, err := dbClient.Gateways(ctx)
			if err != nil {
				return fmt.Errorf("fetching gateways: %w", err)
			}

			gatewaysMu.Lock()
			defer gatewaysMu.Unlock()

			// Calculate added and removed gateways
			oldSet := make(map[string]bool, len(gateways))
			for _, gw := range gateways {
				oldSet[gw] = true
			}

			newSet := make(map[string]bool, len(newGateways))
			for _, gw := range newGateways {
				newSet[gw] = true
			}

			var added, removed []string
			for _, gw := range newGateways {
				if !oldSet[gw] {
					added = append(added, gw)
				}
			}
			for _, gw := range gateways {
				if !newSet[gw] {
					removed = append(removed, gw)
				}
			}

			// Log changes
			if len(added) > 0 {
				slog.With("gateways", added).Info(fmt.Sprintf("Added %d gateway(s)", len(added)))
			}
			if len(removed) > 0 {
				slog.With("gateways", removed).Info(fmt.Sprintf("Removed %d gateway(s)", len(removed)))
			}

			gateways = newGateways
			return nil
		}

		// Initial gateway fetch
		if err := updateGateways(); err != nil {
			return err
		}

		if len(gateways) == 0 {
			return fmt.Errorf("no gateways found in database")
		}

		slog.Info(fmt.Sprintf("Using gateway list from database: initialized with %d gateways to probe", len(gateways)))

		// Start background gateway updater
		go func() {
			ticker := time.NewTicker(probeGatewaysConfig.RefreshInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := updateGateways(); err != nil {
						slog.With("err", err).Warn("Failed to update gateway list")
					}
				}
			}
		}()
	}

	// Create observable gauge for active gateways count
	_, err = meter.Int64ObservableGauge(
		"active_gateways",
		metric.WithDescription("Number of gateways currently being probed"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			gatewaysMu.RLock()
			count := len(gateways)
			gatewaysMu.RUnlock()
			observer.Observe(int64(count))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("creating active_gateways gauge: %w", err)
	}

	downloadCIDsCount := math.MaxInt
	if len(probeGatewaysConfig.DownloadCIDs) > 0 {
		downloadCIDsCount = len(probeGatewaysConfig.DownloadCIDs)
	}

	concurrency := min(probeGatewaysConfig.Concurrency, downloadCIDsCount)

	// Test each gateway in parallel (limit to N concurrent)
	g, gctx := errgroup.WithContext(ctx)

	slog.Info(fmt.Sprintf("Starting %d concurrent worker", concurrency))
	for worker := range concurrency {
		logEntry := slog.With("worker", worker)

		ticker := time.NewTimer(0)
		iterationStart := time.Now()
		maxIter := probeGatewaysConfig.MaxIterations

		g.Go(func() error {
			logEntry.Info("Started gateway probing worker...")

		mainLoop:
			for i := 0; maxIter == 0 || i < maxIter; i++ {

				// Wait for the next iteration
				waitTime := time.Until(iterationStart.Add(probeGatewaysConfig.Interval)).Truncate(time.Second)
				if i > 0 {
					ticker.Reset(waitTime)
					if waitTime > 0 {
						logEntry.With("iteration", i).Info(fmt.Sprintf("Waiting %s until the next iteration...", waitTime))
					}
				}

				select {
				case <-gctx.Done():
					return gctx.Err()
				case <-ticker.C:
					// pass
				}

				iterationStart = time.Now()

				logEntry.With("iteration", i).Info("Starting new gateway probing iteration...")

				// Get the current gateway list (thread-safe)
				gatewaysMu.RLock()
				currentGateways := make([]string, len(gateways))
				copy(currentGateways, gateways)
				gatewaysMu.RUnlock()

				var ciid cid.Cid
				var cidSource string
				if rand.Float32() < probeGatewaysConfig.ControlledShare {
					cidSource = "controlled"
					ciid, err = controlledCIDsProvider.SelectCID(gctx, "controlled")
				} else {
					// Get CID to download (origin doesn't matter for gateways, use "bitswap")
					ciid, err = cidProvider.SelectCID(gctx, "bitswap")

					cidSource = "bitsniffer_bitswap"
					if _, ok := cidProvider.(*pkg.StaticCIDProvider); ok {
						cidSource = "static"
					}
				}

				if errors.Is(err, sql.ErrNoRows) {
					logEntry.Warn("No CID available for gateway probing")
					continue mainLoop
				} else if err != nil {
					return fmt.Errorf("selecting cid from database: %w", err)
				}

				slog.Info(fmt.Sprintf("Worker %d will now start probing %s (%s)", worker, ciid.String(), cidSource))

				rand.Shuffle(len(currentGateways), func(i, j int) {
					currentGateways[i], currentGateways[j] = currentGateways[j], currentGateways[i]
				})

			gatewaysLoop:
				for _, gateway := range currentGateways {
					// Test both raw and trustless (CAR) formats
					formats := []db.GatewayProbeFormat{db.GatewayProbeFormatNone, db.GatewayProbeFormatCAR}

					for _, format := range formats {
						// first iteration uncached, second cached
						for j := 0; j < 2; j++ {
							logEntry.With("cid", ciid.String(), "gateway", gateway, "format", format).Debug("Probing gateway")

							metrics := probeGateway(gctx, gateway, ciid, format, int64(probeGatewaysConfig.MaxDownloadMB)*1024*1024, probeGatewaysConfig.Timeout)

							downloadCounter.Add(gctx, 1, metric.WithAttributes(
								attribute.String("source", cidSource),
								attribute.String("gateway", gateway),
								attribute.Bool("success", metrics.err == nil),
							))

							// Calculate download speed
							var downloadSpeedMbps *float64
							if metrics.bytesReceived > 0 && metrics.downloadEnd.Sub(metrics.reqStart) > 0 {
								durationS := metrics.downloadEnd.Sub(metrics.reqStart).Seconds()
								speedBps := float64(metrics.bytesReceived) / durationS
								speedMbps := (speedBps * 8) / (1024 * 1024)
								downloadSpeedMbps = &speedMbps
							}

							// Extract IPFS headers
							var ipfsPath, ipfsRoots, cacheStatus, contentType *string
							if metrics.headers != nil {
								if v := metrics.headers.Get("X-Ipfs-Path"); v != "" {
									ipfsPath = &v
								}
								if v := metrics.headers.Get("X-Ipfs-Roots"); v != "" {
									ipfsRoots = &v
								}

								// convert header type
								hdr := make(map[string]any, len(metrics.headers))
								for k, v := range metrics.headers {
									if len(v) > 0 {
										// drop multi-value header fields
										hdr[strings.ToLower(k)] = v[0]
									}
								}
								cacheStatus = pkg.ParseCacheStatus(hdr)

								if v := metrics.headers.Get("Content-Type"); v != "" {
									contentType = &v
								}
							}

							// Get Content-Length if present
							var contentLength *int64
							if metrics.headers != nil {
								if cl := metrics.headers.Get("Content-Length"); cl != "" {
									var clVal int64
									if _, err := fmt.Sscanf(cl, "%d", &clVal); err == nil {
										contentLength = &clVal
									}
								}
							}

							// Prepare database model
							dbGatewayProbe := &db.GatewayProbeModel{
								RunID:             runID.String(),
								Region:            rootConfig.AWSRegion,
								TirosVersion:      cmd.Root().Version,
								Gateway:           gateway,
								CID:               ciid.String(),
								CIDSource:         cidSource,
								Format:            string(format),
								RequestStart:      metrics.reqStart,
								DNSDurationS:      toPtr(metrics.dnsDuration.Seconds()),
								ConnDurationS:     toPtr(metrics.connDuration.Seconds()),
								TTFBS:             toPtr(metrics.ttfb.Seconds()),
								DownloadDurationS: metrics.downloadEnd.Sub(metrics.reqStart).Seconds(),
								BytesReceived:     metrics.bytesReceived,
								ContentLength:     contentLength,
								DownloadSpeedMbps: downloadSpeedMbps,
								StatusCode:        metrics.statusCode,
								IPFSPath:          ipfsPath,
								IPFSRoots:         ipfsRoots,
								CacheStatus:       cacheStatus,
								ContentType:       contentType,
								CARValidated:      metrics.carValidated,
								RedirectCount:     metrics.redirectCount,
								FinalURL:          toPtr(metrics.finalURL),
								CreatedAt:         time.Now(),
							}

							if metrics.err != nil {
								errStr := metrics.err.Error()
								dbGatewayProbe.Error = &errStr
								logEntry.With("cid", ciid.String(), "gateway", gateway, "err", metrics.err, "format", format).Info("Error downloading from gateway")
							} else {
								logEntry.With("cid", ciid.String(), "gateway", gateway, "format", format, "ttfb_s", metrics.ttfb.Seconds(), "cache", deref(cacheStatus)).Info("Gateway probe successful")
							}

							if err := dbClient.InsertGatewayProbe(gctx, dbGatewayProbe); err != nil {
								return fmt.Errorf("inserting gateway probe into database: %w", err)
							}

							if metrics.err != nil {
								// if we encountered an error, we're done with this gateway
								continue gatewaysLoop
							}
						}
					}
				}
			}
			return nil
		})
	}

	return g.Wait()
}

func probeGateway(ctx context.Context, gateway string, ciid cid.Cid, format db.GatewayProbeFormat, maxBytes int64, timeout time.Duration) *gatewayMetrics {
	metrics := &gatewayMetrics{
		reqStart: time.Now(),
	}

	// Create request context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Construct URL based on format
	var url string
	if !strings.HasPrefix(gateway, "http://") && !strings.HasPrefix(gateway, "https://") {
		gateway = "https://" + gateway
	}
	gateway = strings.TrimSuffix(gateway, "/")

	switch format {
	case db.GatewayProbeFormatNone:
		url = fmt.Sprintf("%s/ipfs/%s", gateway, ciid.String())
	case db.GatewayProbeFormatRaw:
		url = fmt.Sprintf("%s/ipfs/%s?format=raw", gateway, ciid.String())
	case db.GatewayProbeFormatCAR:
		url = fmt.Sprintf("%s/ipfs/%s?format=car", gateway, ciid.String())
	default:
		panic(fmt.Sprintf("unknown gateway probe format: %s", format))
	}

	// Create HTTP client with detailed tracing
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			metrics.dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			metrics.dnsDuration = time.Since(metrics.dnsStart)
		},
		ConnectStart: func(_, _ string) {
			metrics.connStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			metrics.connDuration = time.Since(metrics.connStart)
		},
		TLSHandshakeStart: func() {
			metrics.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			metrics.tlsDuration = time.Since(metrics.tlsStart)
		},
		GotFirstResponseByte: func() {
			metrics.ttfb = time.Since(metrics.reqStart)
		},
	}

	reqCtx = httptrace.WithClientTrace(reqCtx, trace)

	// Create HTTP client with redirect tracking
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Track redirect chain
			metrics.redirectCount = len(via)
			if req.URL != nil {
				metrics.redirectChain = append(metrics.redirectChain, req.URL.String())
			}
			// Allow up to 10 redirects (Go's default)
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	// Create request
	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		metrics.err = fmt.Errorf("creating request: %w", err)
		metrics.downloadEnd = time.Now()
		return metrics
	}

	// According to: https://probelab-analytics.slack.com/archives/C08MY2YENG3/p1776796611945589
	req.Header.Set("X-Requested-With", "tiros")

	// Set appropriate Accept header for CAR format
	switch format {
	case db.GatewayProbeFormatNone:
		// none
	case db.GatewayProbeFormatRaw:
		req.Header.Set("Accept", "application/vnd.ipld.raw")
	case db.GatewayProbeFormatCAR:
		req.Header.Set("Accept", "application/vnd.ipld.car")
	default:
		panic(fmt.Sprintf("unknown gateway probe format: %s", format))
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		metrics.err = fmt.Errorf("executing request: %w", err)
		metrics.downloadEnd = time.Now()
		return metrics
	}
	defer resp.Body.Close()

	metrics.statusCode = resp.StatusCode
	metrics.headers = resp.Header

	// Store final URL (may differ from initial if redirected)
	if resp.Request != nil && resp.Request.URL != nil {
		metrics.finalURL = resp.Request.URL.String()
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		metrics.err = fmt.Errorf("HTTP %d", resp.StatusCode)
		metrics.downloadEnd = time.Now()
		return metrics
	}

	// Read response body up to maxBytes, always capturing bytes in buffer
	var buf bytes.Buffer
	teeReader := io.TeeReader(io.LimitReader(resp.Body, maxBytes), &buf)

	bytesRead, err := io.Copy(io.Discard, teeReader)
	metrics.bytesReceived = bytesRead
	metrics.downloadEnd = time.Now()

	if err != nil && err != io.EOF {
		metrics.err = fmt.Errorf("reading response: %w", err)
		return metrics
	}

	// Validate CAR format if applicable
	if format != db.GatewayProbeFormatCAR || resp.StatusCode != 200 {
		return metrics
	}

	// set to false by default
	metrics.carValidated = toPtr(false)

	if buf.Len() == 0 {
		return metrics
	}

	// Try to parse the CAR file
	carReader, err := carv2.NewBlockReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return metrics
	}

	// Check if requested CID is in the roots
	for _, root := range carReader.Roots {
		if root.Equals(ciid) {
			metrics.carValidated = toPtr(true)
			break
		}
	}

	return metrics
}

func deref[T any](p *T) T {
	if p == nil {
		return *new(T)
	}
	return *p
}
