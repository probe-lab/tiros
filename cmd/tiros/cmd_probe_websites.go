package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	pllog "github.com/probe-lab/go-commons/log"
	"github.com/probe-lab/go-commons/ptr"
	"github.com/probe-lab/tiros/pkg/db"
	"github.com/probe-lab/tiros/pkg/kubo"
	"github.com/probe-lab/tiros/pkg/ws"
	"github.com/urfave/cli/v3"
)

var probeWebsitesConfig = struct {
	Websites        []string
	Probes          int
	LookupProviders bool
	KuboHost        string
	KuboAPIPort     int
	KuboGatewayPort int
	ChromeCDPHost   string
	ChromeCDPPort   int
	ChromeKuboHost  string
}{
	Websites:        []string{},
	Probes:          3,
	LookupProviders: true,
	KuboHost:        "127.0.0.1",
	KuboAPIPort:     5001,
	KuboGatewayPort: 8080,
	ChromeCDPHost:   "127.0.0.1",
	ChromeCDPPort:   3000,
	ChromeKuboHost:  "",
}

var probeWebsitesCmd = &cli.Command{
	Name:  "websites",
	Usage: "Start probing website performance.",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:        "probes",
			Usage:       "number of times to probe each URL",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_PROBES"),
			Value:       probeWebsitesConfig.Probes,
			Destination: &probeWebsitesConfig.Probes,
		},
		&cli.StringSliceFlag{
			Name:        "websites",
			Usage:       "list of websites to probe",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_WEBSITES"),
			Value:       probeWebsitesConfig.Websites,
			Destination: &probeWebsitesConfig.Websites,
		},
		&cli.BoolFlag{
			Name:        "lookup.providers",
			Usage:       "Whether to lookup website providers",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_LOOKUP_PROVIDERS"),
			Value:       probeWebsitesConfig.LookupProviders,
			Destination: &probeWebsitesConfig.LookupProviders,
		},
		&cli.StringFlag{
			Name:        "kubo.host",
			Usage:       "Host at which to reach Kubo",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_KUBO_HOST"),
			Value:       probeWebsitesConfig.KuboHost,
			Destination: &probeWebsitesConfig.KuboHost,
		},
		&cli.IntFlag{
			Name:        "kubo.api.port",
			Usage:       "port to reach a Kubo-compatible RPC API",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_KUBO_API_PORT"),
			Value:       probeWebsitesConfig.KuboAPIPort,
			Destination: &probeWebsitesConfig.KuboAPIPort,
		},
		&cli.IntFlag{
			Name:        "kubo.gateway.port",
			Usage:       "port at which to reach Kubo's HTTP gateway",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_KUBO_GATEWAY_PORT"),
			Value:       probeWebsitesConfig.KuboGatewayPort,
			Destination: &probeWebsitesConfig.KuboGatewayPort,
		},
		&cli.StringFlag{
			Name:        "chrome.cdp.host",
			Usage:       "host at which the Chrome DevTools Protocol is reachable",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_CHROME_CDP_HOST"),
			Value:       probeWebsitesConfig.ChromeCDPHost,
			Destination: &probeWebsitesConfig.ChromeCDPHost,
		},
		&cli.IntFlag{
			Name:        "chrome.cdp.port",
			Usage:       "port to reach the Chrome DevTools Protocol port",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_CHROME_CDP_PORT"),
			Value:       probeWebsitesConfig.ChromeCDPPort,
			Destination: &probeWebsitesConfig.ChromeCDPPort,
		},
		&cli.StringFlag{
			Name:        "chrome.kubo.host",
			Usage:       "the kubo host from Chrome's perspective. This may be different from Tiros, especially if Chrome and Kubo are run with docker.",
			Sources:     cli.EnvVars("TIROS_PROBE_WEBSITES_CHROME_KUBO_HOST"),
			Value:       probeWebsitesConfig.ChromeKuboHost,
			Destination: &probeWebsitesConfig.ChromeKuboHost,
			DefaultText: "--kubo.host",
		},
	},
	Action: probeWebsitesAction,
}

func probeWebsitesAction(ctx context.Context, cmd *cli.Command) error {
	runID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("creating run id: %w", err)
	}

	// initializing the clickhouse db client
	dbClient, err := newDBClient(ctx)
	if err != nil {
		return fmt.Errorf("creating database client: %w", err)
	}
	defer pllog.Defer(dbClient.Close, "Failed closing database client")

	if probeWebsitesConfig.ChromeKuboHost == "" {
		probeWebsitesConfig.ChromeKuboHost = probeWebsitesConfig.KuboHost
	}

	kuboCfg := &kubo.KuboConfig{
		Host:           probeWebsitesConfig.KuboHost,
		APIPort:        probeWebsitesConfig.KuboAPIPort,
		GWPort:         probeWebsitesConfig.KuboGatewayPort,
		ChromeKuboHost: probeWebsitesConfig.ChromeKuboHost,
	}
	kuboClient, err := kubo.NewKubo(kuboCfg)
	if err != nil {
		return fmt.Errorf("creating kubo client: %w", err)
	}

	if err := kuboClient.WaitAvailable(ctx, time.Minute); err != nil {
		return err
	}

	kuboVersion, err := kuboClient.Version(ctx)
	if err != nil {
		return err
	}

	kuboID, err := kuboClient.ID(ctx)
	if err != nil {
		return err
	}

	var websites []string
	if len(probeWebsitesConfig.Websites) == 0 {
		if websites, err = dbClient.Websites(ctx); err != nil {
			return fmt.Errorf("getting websites from database: %w", err)
		}
	} else {
		slog.Info("Using static list of websites.")
		websites = probeWebsitesConfig.Websites
	}

	// shuffle websites, so that we have a different order in which we request the websites.
	// If we didn't do this a single website would always be requested with a comparatively "cold" ipfs node.
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(websites), func(i, j int) {
		websites[i], websites[j] = websites[j], websites[i]
	})

	slog.With("probes", probeWebsitesConfig.Probes).Info(fmt.Sprintf("Queried %d websites:", len(websites)))
	for _, w := range websites {
		slog.Info("  " + w)
	}

	providerResults := make(chan *kubo.Provider)
	probeResults := make(chan *ws.WebsiteProbeResult)

	go measureWebsites(ctx, kuboClient, websites, probeResults)
	go findAllProviders(ctx, kuboClient, websites, providerResults)

	for {
		slog.Info("Awaiting Provider or Probe result...")
		select {
		case pr, more := <-probeResults:
			if !more {
				slog.Info("Probing websites done!")
				probeResults = nil
			} else {
				slog.With("url", pr.URL).Info("Handling probe result")

				var errStr string
				if pr.Err != nil {
					errStr = pr.Err.Error()
				}

				metricsJSON, err := pr.MetricsJSON()
				metricsJSONStr := string(metricsJSON)
				if err != nil {
					slog.With("err", err).Warn("Error marshalling metrics")
					metricsJSONStr = "{}"
				}

				wpm := &db.WebsiteProbeModel{
					RunID:        runID.String(),
					Region:       rootConfig.AWSRegion,
					TirosVersion: cmd.Root().Version,
					KuboVersion:  kuboVersion.Version,
					KuboPeerID:   kuboID.ID,
					Website:      pr.Website,
					URL:          pr.URL,
					Protocol:     string(pr.Protocol),
					IPFSImpl:     "KUBO",
					Try:          pr.Try,
					TTFB:         pr.TTFB,
					FCP:          pr.FCP,
					LCP:          pr.LCP,
					TTI:          pr.TTI,
					CLS:          pr.CLS,
					TTFBRating:   pr.TTFBRating,
					CLSRating:    pr.CLSRating,
					FCPRating:    pr.FCPRating,
					LCPRating:    pr.LCPRating,
					StatusCode:   pr.HTTPStatus,
					Body:         pr.HTTPBody,
					Metrics:      metricsJSONStr,
					Error:        toPtr(errStr),
					CreatedAt:    time.Now(),
				}
				if err = dbClient.InsertWebsiteProbe(ctx, wpm); err != nil {
					return fmt.Errorf("save measurement: %w", err)
				}
			}

		case pr, more := <-providerResults:
			if !more {
				slog.Info("Searching for providers done!")
				providerResults = nil
			} else {
				if errors.Is(pr.Err, context.DeadlineExceeded) {
					pr.Err = context.DeadlineExceeded
				}
				slog.With("err", pr.Err).
					With("peerID", pr.ID.String()[:16]).
					With("website", pr.Website).
					Info("Handling provider result")

				maddrs := make([]string, 0, len(pr.Maddrs))
				for _, addr := range pr.Maddrs {
					maddrs = append(maddrs, addr.String())
				}

				pm := &db.ProviderModel{
					RunID:          runID.String(),
					Region:         rootConfig.AWSRegion,
					TirosVersion:   cmd.Root().Version,
					KuboVersion:    kuboVersion.Version,
					KuboPeerID:     kuboID.ID,
					Website:        pr.Website,
					Path:           pr.Path,
					ProviderID:     pr.ID.String(),
					AgentVersion:   pr.Agent,
					MultiAddresses: maddrs,
					IsRelayed:      pr.IsRelayed,
					Error:          pr.Err,
					CreatedAt:      time.Now(),
				}
				if err = dbClient.InsertProvider(ctx, pm); err != nil {
					return fmt.Errorf("save provider: %w", err)
				}
			}
		}

		if probeResults == nil && providerResults == nil {
			break
		}
	}

	return nil
}

func measureWebsites(ctx context.Context, k *kubo.Kubo, websites []string, results chan<- *ws.WebsiteProbeResult) {
	defer close(results)

	if !probeWebsitesConfig.LookupProviders {
		return
	}

	for i := 0; i < probeWebsitesConfig.Probes; i++ {
		for _, protocol := range []db.WebsiteProbeProtocol{db.WebsiteProbeProtocolIPFS, db.WebsiteProbeProtocolHTTP} {
			for _, website := range websites {
				slog.Info("Start probing", "website", website, "protocol", protocol)
				wp := &ws.WebsiteProbe{
					URL:       k.WebsiteURL(website, protocol),
					Website:   website,
					ProbeType: protocol,
					CDPPort:   probeWebsitesConfig.ChromeCDPPort,
					Result: &ws.WebsiteProbeResult{
						URL:      k.WebsiteURL(website, protocol),
						Website:  website,
						Protocol: protocol,
					},
				}

				pr, err := wp.Run(ctx)
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				} else if err != nil {
					slog.With("err", err, "website", website).Warn("error probing website")
					continue
				}

				pr.Website = website
				pr.Protocol = protocol
				pr.Try = i

				slog.With(
					"ttfb", ptr.From(pr.TTFB),
					"lcp", ptr.From(pr.LCP),
					"fcp", ptr.From(pr.FCP),
					"tti", ptr.From(pr.TTI),
					"status", pr.HTTPStatus,
					"err", pr.Err,
				).Info("Probed website " + website)

				results <- pr

				if protocol == db.WebsiteProbeProtocolIPFS {
					if k.Reset(ctx); err != nil {
						slog.With("err", err).Warn("error running ipfs gc")
						continue
					}
				}
			}
		}
	}
}

func findAllProviders(ctx context.Context, k *kubo.Kubo, websites []string, results chan<- *kubo.Provider) {
	defer close(results)
	for _, website := range websites {
		for retry := 0; retry < 3; retry++ {
			err := k.FindProviders(ctx, website, results)
			if err != nil {
				slog.With("err", err, "retry", retry, "website", website).Warn("Couldn't find providers")
				if strings.Contains(err.Error(), "routing/findprovs") {
					continue
				}
			}
			break
		}
	}
}
