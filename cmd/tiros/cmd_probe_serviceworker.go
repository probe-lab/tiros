package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/probe-lab/tiros/pkg"
	"github.com/probe-lab/tiros/pkg/db"
	"github.com/probe-lab/tiros/pkg/sw"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var probeServiceWorkerConfig = struct {
	Interval        time.Duration
	MaxIterations   int
	DownloadCIDs    []string
	Gateways        []string
	MaxDownloadMB   int
	Timeout         time.Duration
	ChromeCDPHost   string
	ChromeCDPPort   int
	ControlledCIDs  bool
	ControlledShare float32
}{
	Interval:        10 * time.Second,
	MaxIterations:   0,
	DownloadCIDs:    []string{},
	Gateways:        []string{"inbrowser.link"},
	MaxDownloadMB:   10,
	Timeout:         2 * time.Minute,
	ChromeCDPHost:   "127.0.0.1",
	ChromeCDPPort:   3000,
	ControlledCIDs:  true,
	ControlledShare: 0.2,
}

var probeServiceWorkerFlags = []cli.Flag{
	&cli.DurationFlag{
		Name:        "interval",
		Usage:       "How long to wait between each download iteration",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_INTERVAL"),
		Value:       probeServiceWorkerConfig.Interval,
		Destination: &probeServiceWorkerConfig.Interval,
	},
	&cli.IntFlag{
		Name:        "iterations.max",
		Usage:       "The number of iterations per concurrent worker to run. 0 means infinite.",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_ITERATIONS_MAX"),
		Value:       probeServiceWorkerConfig.MaxIterations,
		Destination: &probeServiceWorkerConfig.MaxIterations,
	},
	&cli.StringSliceFlag{
		Name:        "cids",
		Usage:       "A static list of CIDs to download from the Gateways.",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_DOWNLOAD_CIDS"),
		Value:       probeServiceWorkerConfig.DownloadCIDs,
		Destination: &probeServiceWorkerConfig.DownloadCIDs,
	},
	&cli.StringSliceFlag{
		Name:        "gateways",
		Usage:       "A static list of gateways to probe (takes precedence over database)",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_GATEWAYS"),
		Value:       probeServiceWorkerConfig.Gateways,
		Destination: &probeServiceWorkerConfig.Gateways,
	},
	&cli.IntFlag{
		Name:        "download.max.mb",
		Usage:       "Maximum download size in MiB before cancelling",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_DOWNLOAD_MAX_MB"),
		Value:       probeServiceWorkerConfig.MaxDownloadMB,
		Destination: &probeServiceWorkerConfig.MaxDownloadMB,
	},
	&cli.DurationFlag{
		Name:        "timeout",
		Usage:       "Timeout for each gateway request",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_TIMEOUT"),
		Value:       probeServiceWorkerConfig.Timeout,
		Destination: &probeServiceWorkerConfig.Timeout,
	},
	&cli.StringFlag{
		Name:        "chrome.cdp.host",
		Usage:       "host at which the Chrome DevTools Protocol is reachable",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CHROME_CDP_HOST"),
		Value:       probeServiceWorkerConfig.ChromeCDPHost,
		Destination: &probeServiceWorkerConfig.ChromeCDPHost,
	},
	&cli.IntFlag{
		Name:        "chrome.cdp.port",
		Usage:       "port to reach the Chrome DevTools Protocol port",
		Sources:     cli.EnvVars("TIROS_PROBE_GATEWAYS_CHROME_CDP_PORT"),
		Value:       probeServiceWorkerConfig.ChromeCDPPort,
		Destination: &probeServiceWorkerConfig.ChromeCDPPort,
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

var probeServiceWorkerCmd = &cli.Command{
	Name:   "serviceworker",
	Usage:  "Start probing IPFS Service Worker Gateway retrieval performance",
	Flags:  probeServiceWorkerFlags,
	Action: probeServiceWorkerAction,
}

func probeServiceWorkerAction(ctx context.Context, cmd *cli.Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	meter := otel.GetMeterProvider().Meter("tiros")

	probeCounter, err := meter.Int64Counter("probes")
	if err != nil {
		return fmt.Errorf("creating probe counter: %w", err)
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
	if len(probeServiceWorkerConfig.DownloadCIDs) > 0 {
		cidProvider, err = pkg.NewStaticCIDProvider(probeServiceWorkerConfig.DownloadCIDs)
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
	slog.With("provider", cidProviderName).Info("Using CID provider for service worker probes")

	controlledCIDsProvider, err := pkg.NewControlledCIDProvider()
	if err != nil {
		return fmt.Errorf("creating controlled cid provider: %w", err)
	}

	// Use configured gateways (defaults to inbrowser.link)
	gateways := probeServiceWorkerConfig.Gateways
	slog.With("count", len(gateways), "gateways", gateways).Info("Using service worker gateways")

	browserURL := url.URL{
		Scheme: "ws",
		Host: net.JoinHostPort(
			probeServiceWorkerConfig.ChromeCDPHost,
			strconv.Itoa(probeServiceWorkerConfig.ChromeCDPPort),
		),
	}

	slog.With("url", browserURL.String()).Debug("Connecting to browser...")
	ctx, cancel = chromedp.NewRemoteAllocator(ctx, browserURL.String())
	defer cancel()

	ticker := time.NewTimer(0)
	iterationStart := time.Now()
	maxIter := probeServiceWorkerConfig.MaxIterations

	for i := 0; maxIter == 0 || i < maxIter; i++ {
		// Wait for next iteration
		waitTime := time.Until(iterationStart.Add(probeServiceWorkerConfig.Interval)).Truncate(time.Second)
		if i > 0 {
			ticker.Reset(waitTime)
			if waitTime > 0 {
				slog.With("iteration", i).Info(fmt.Sprintf("Waiting %s until the next iteration...", waitTime))
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// pass
		}

		iterationStart = time.Now()

		slog.With("iteration", i).Info("Starting new service worker probing iteration...")

		var ciid cid.Cid
		var cidSource string
		if rand.Float32() < probeGatewaysConfig.ControlledShare {
			cidSource = "controlled"
			ciid, err = controlledCIDsProvider.SelectCID(ctx, "controlled")
		} else {
			ciid, err = cidProvider.SelectCID(ctx, "dht")

			cidSource = "bitsniffer_bitswap"
			if _, ok := cidProvider.(*pkg.StaticCIDProvider); ok {
				cidSource = "static"
			}
		}

		if errors.Is(err, sql.ErrNoRows) {
			slog.With("err", err).Warn("No CID available for service worker probing")
			continue
		} else if err != nil {
			return fmt.Errorf("selecting cid from database: %w", err)
		}

		// Probe each gateway
		for _, gateway := range gateways {

			navURL := url.URL{
				Scheme: "https",
				Host:   gateway,
				Path:   "/ipfs/" + ciid.String(),
			}

			// Create and run probe
			slog.With("gateway", gateway).Info("Probing service worker gateway")
			probe := sw.NewSwProbe(ciid, navURL.String())

			probeCtx, probeCancel := context.WithTimeout(ctx, probeServiceWorkerConfig.Timeout)
			result, err := probe.Run(probeCtx)
			probeCancel()

			status := 0
			if result != nil {
				status = result.FinalStatusCode
			}
			probeCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("gateway", gateway),
				attribute.Int("status", status),
				attribute.String("source", cidSource),
			))

			var errStr *string
			if err != nil {
				slog.Warn("Error running service worker probe", "url", navURL.String(), "err", err)
				errMsg := err.Error()
				errStr = &errMsg
			}

			dbModel := &db.ServiceWorkerProbeModel{
				RunID:         runID.String(),
				Region:        rootConfig.AWSRegion,
				TirosVersion:  cmd.Root().Version,
				Gateway:       gateway,
				CID:           ciid.String(),
				CIDSource:     cidSource,
				URL:           navURL.String(),
				Error:         errStr,
				CreatedAt:     time.Now(),
				ServerTimings: "{}",
			}

			// Populate fields from result if successful
			if result != nil {
				// Core timing metrics
				dbModel.TotalTTFBS = toPtr(result.TotalTTFB.Seconds())
				dbModel.FinalTTFBS = toPtr(result.FinalTTFB.Seconds())
				dbModel.TimeToFinalRedirectS = toPtr(result.TimeToFinalRedirect.Seconds())
				dbModel.ServiceWorkerVersion = toPtr(result.ServiceWorkerVersion)
				dbModel.StatusCode = result.FinalStatusCode
				dbModel.ContentType = toPtr(result.ContentType)
				dbModel.ContentLength = toPtr(result.ContentLength)
				dbModel.IPFSPath = toPtr(result.IPFSPath)
				dbModel.IPFSRoots = toPtr(result.IPFSRoots)
				dbModel.FoundProviders = result.FoundProviders
				dbModel.ServedFromGateway = result.ServedFromGateway
				dbModel.GatewayCacheStatus = result.GatewayCacheStatus
				dbModel.DelegatedRouterTTFBS = toPtr(result.DelegatedRouterTTFB.Seconds())
				dbModel.TrustlessGatewayTTFBS = toPtr(result.TrustlessGatewayTTFB.Seconds())

				// Parse server timings into parallel arrays for the Nested column
				// and compute hot-path scalar projections.
				stRow := sw.ParseServerTimings(result.ServerTimings)
				dbModel.ServerTimingName = stRow.NameArr
				dbModel.ServerTimingDurS = stRow.DurSArr
				dbModel.ServerTimingSystem = stRow.SystemArr
				dbModel.ServerTimingProviderID = stRow.ProviderIDArr
				dbModel.ServerTimingTransport = stRow.TransportArr
				dbModel.ServerTimingExtra = stRow.ExtraArr
				dbModel.STIPFSResolveS = stRow.IPFSResolveS
				dbModel.STDNSLinkResolveS = stRow.DNSLinkResolveS
				dbModel.STIPNSResolveS = stRow.IPNSResolveS
				dbModel.STFirstConnectS = stRow.FirstConnectS
				dbModel.STFirstBlockS = stRow.FirstBlockS
				dbModel.STProviderCountHTTPGateway = stRow.ProviderCountHTTPGateway
				dbModel.STProviderCountLibp2p = stRow.ProviderCountLibp2p
				dbModel.STFastestBlockSystem = stRow.FastestBlockSystem
			}

			slog.With(
				"gateway", gateway,
				"status", dbModel.StatusCode,
				"totalTTFBs", deref(dbModel.TotalTTFBS),
				"finalTTFBs", deref(dbModel.FinalTTFBS),
				"stFirstBlockS", deref(dbModel.STFirstBlockS),
				"serverTimingCount", len(dbModel.ServerTimingName),
				"cid", ciid.String(),
			).Info("Inserting service worker probe into database")

			if err := dbClient.InsertServiceWorkerProbe(ctx, dbModel); err != nil {
				return fmt.Errorf("inserting service worker probe into database: %w", err)
			}

		}
	}

	return nil
}
