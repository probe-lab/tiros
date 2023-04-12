package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/tiros/models"
)

var RunCommand = &cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:     "websites",
			Usage:    "Websites to test against. Example: 'ipfs.io' or 'filecoin.io",
			EnvVars:  []string{"TIROS_RUN_WEBSITES"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "region",
			Usage:    "In which region does this tiros task run in",
			EnvVars:  []string{"TIROS_RUN_REGION"},
			Required: true,
		},
		&cli.IntSliceFlag{
			Name:    "settle-times",
			Usage:   "a list of times to settle in seconds",
			EnvVars: []string{"TIROS_RUN_SETTLE_TIMES"},
			Value:   cli.NewIntSlice(10, 1200),
		},
		&cli.IntFlag{
			Name:    "times",
			Usage:   "number of times to test each URL",
			EnvVars: []string{"TIROS_RUN_TIMES"},
			Value:   3,
		},
		&cli.BoolFlag{
			Name:    "dry-run",
			Usage:   "Whether to skip DB interactions",
			EnvVars: []string{"TIROS_RUN_DRY_RUN"},
		},
		&cli.StringFlag{
			Name:    "db-host",
			Usage:   "On which host address can this clustertest reach the database",
			EnvVars: []string{"TIROS_RUN_DATABASE_HOST"},
		},
		&cli.IntFlag{
			Name:    "db-port",
			Usage:   "On which port can this clustertest reach the database",
			EnvVars: []string{"TIROS_RUN_DATABASE_PORT"},
		},
		&cli.StringFlag{
			Name:    "db-name",
			Usage:   "The name of the database to use",
			EnvVars: []string{"TIROS_RUN_DATABASE_NAME"},
		},
		&cli.StringFlag{
			Name:    "db-password",
			Usage:   "The password for the database to use",
			EnvVars: []string{"TIROS_RUN_DATABASE_PASSWORD"},
		},
		&cli.StringFlag{
			Name:    "db-user",
			Usage:   "The user with which to access the database to use",
			EnvVars: []string{"TIROS_RUN_DATABASE_USER"},
		},
		&cli.StringFlag{
			Name:    "db-sslmode",
			Usage:   "The sslmode to use when connecting the the database",
			EnvVars: []string{"TIROS_RUN_DATABASE_SSL_MODE"},
		},
		&cli.IntFlag{
			Name:    "kubo-api-port",
			Usage:   "port to reach the Kubo API",
			EnvVars: []string{"TIROS_RUN_KUBO_API_PORT"},
			Value:   5001,
		},
		&cli.IntFlag{
			Name:    "kubo-gateway-port",
			Usage:   "port to reach the Kubo Gateway",
			EnvVars: []string{"TIROS_RUN_KUBO_GATEWAY_PORT"},
			Value:   8080,
		},
		&cli.IntFlag{
			Name:    "chrome-cdp-port",
			Usage:   "port to reach the Chrome DevTools Protocol port",
			EnvVars: []string{"TIROS_RUN_CHROME_CDP_PORT"},
			Value:   3000,
		},
		&cli.IntFlag{
			Name:    "cpu",
			Usage:   "CPU resources for this measurement run",
			EnvVars: []string{"TIROS_RUN_CPU"},
			Value:   2,
		},
		&cli.IntFlag{
			Name:    "memory",
			Usage:   "Memory resources for this measurement run",
			EnvVars: []string{"TIROS_RUN_MEMORY"},
			Value:   4096,
		},
	},
	Action: RunAction,
}

func RunAction(c *cli.Context) error {
	log.Infoln("Starting Tiros run...")
	defer log.Infoln("Stopped Tiros run.")

	var err error
	var dbClient IDBClient = DBDummyClient{}
	if !c.Bool("dry-run") {
		dbClient, err = InitClient(c.Context, c.String("db-host"), c.Int("db-port"), c.String("db-name"), c.String("db-user"), c.String("db-password"), c.String("db-sslmode"))
		if err != nil {
			return fmt.Errorf("init db connection: %w", err)
		}
	}

	kubo := shell.NewShell(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.Int("kubo-api-port")))

	t := Tiros{
		DBClient: dbClient,
		Kubo:     kubo,
	}

	if _, err := t.InitRun(c); err != nil {
		return fmt.Errorf("init run: %w", err)
	}
	defer func() {
		if _, err = t.DBClient.SealRun(context.Background(), t.DBRun); err != nil {
			log.WithError(err).Warnln("Couldn't seal run")
		}
	}()

	// shuffle websites
	websites := c.StringSlice("websites")
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(websites), func(i, j int) {
		websites[i], websites[j] = websites[j], websites[i]
	})

	log.WithFields(log.Fields{
		"websites":    websites,
		"settleTimes": c.IntSlice("settle-times"),
		"times":       c.Int("times"),
	}).Infoln("Starting run!")

	wpsChan := t.FindAllProvidersAsync(c.Context, websites)

	for _, settle := range c.IntSlice("settle-times") {

		sleepDur := time.Duration(settle) * time.Second

		log.Infof("Letting Kubo settle for %s\n", sleepDur)
		time.Sleep(sleepDur)

		for i := 0; i < c.Int("times"); i++ {
			for _, mType := range []string{models.MeasurementTypeKUBO, models.MeasurementTypeHTTP} {
				for _, website := range websites {
					pr, err := t.Probe(c, websiteURL(c, website, mType))
					if err != nil {
						return fmt.Errorf("probing %s: %w", website, err)
					}

					log.WithFields(log.Fields{
						"ttfb": p2f(pr.TimeToFirstByte),
						"lcp":  p2f(pr.LargestContentfulPaint),
						"fcp":  p2f(pr.FirstContentfulPaint),
						"tti":  p2f(pr.TimeToInteract),
					}).WithError(pr.Error).Infoln("Probed website", website)

					if _, err := t.DBClient.SaveMeasurement(c, t.DBRun, pr, website, mType, i); err != nil {
						return fmt.Errorf("save measurement: %w", err)
					}

					if mType == models.MeasurementTypeKUBO {
						if err = t.KuboGC(c.Context); err != nil {
							return fmt.Errorf("kubo gc: %w", err)
						}
					}
				}
			}
		}
	}

	wps := <-wpsChan
	for _, wp := range wps {
		_, err := t.DBClient.SaveProvider(c, t.DBRun, wp)
		if err != nil {
			log.WithError(err).WithField("website", wp.Website).Warnln("Couldn't save providers")
		}
	}

	return nil
}

func websiteURL(c *cli.Context, website string, mType string) string {
	switch mType {
	case models.MeasurementTypeKUBO:
		return fmt.Sprintf("http://localhost:%d/ipns/%s", c.Int("kubo-gateway-port"), website)
	case models.MeasurementTypeHTTP:
		return fmt.Sprintf("https://%s", website)
	default:
		panic(fmt.Sprintf("unknown measurement type: %s", mType))
	}
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func (t *Tiros) KuboGC(ctx context.Context) error {
	return t.Kubo.Request("repo/gc").Exec(ctx, nil)
}

type WebsiteProviders struct {
	Website   string
	Path      string
	Providers []WebsiteProvider
}

type WebsiteProvider struct {
	ID           peer.ID
	Addrs        []multiaddr.Multiaddr
	AgentVersion string
}

type NameResolveResponse struct {
	Path string
}

func (t *Tiros) FindAllProvidersAsync(ctx context.Context, websites []string) <-chan []*WebsiteProviders {
	results := make(chan []*WebsiteProviders)
	go func() {
		wps := []*WebsiteProviders{}
		for _, website := range websites {
			wp, err := t.FindProviders(ctx, website)
			if err != nil {
				log.WithError(err).WithField("website", website).Warnln("Couldn't find providers")
				if wp == nil {
					continue
				}
			}
			wps = append(wps, wp)
		}
		results <- wps
		close(results)
	}()

	return results
}

func (t *Tiros) FindProviders(ctx context.Context, website string) (*WebsiteProviders, error) {
	wps := &WebsiteProviders{
		Website:   website,
		Providers: []WebsiteProvider{},
	}
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	resp, err := t.Kubo.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return wps, fmt.Errorf("name/resolve: %w", err)
	}

	dat, err := io.ReadAll(resp.Output)
	if err != nil {
		return wps, fmt.Errorf("read name/resolve bytes: %w", err)
	}

	nrr := NameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return wps, fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	wps.Path = nrr.Path

	resp, err = t.Kubo.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Send(ctx)
	if err != nil {
		return wps, fmt.Errorf("routing/findprovs: %w", err)
	}

	providers := []*peer.AddrInfo{}
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		evt := routing.QueryEvent{}
		if err = dec.Decode(&evt); err != nil {
			return wps, fmt.Errorf("decode routing/findprovs response: %w", err)
		}
		if evt.Type != routing.Provider {
			continue
		}

		if len(evt.Responses) != 1 {
			logEntry.Warnln("findprovs Providerquery event with != 1 responses:", len(evt.Responses))
			continue
		}

		providers = append(providers, evt.Responses[0])
	}

	for _, provider := range providers {
		wp := WebsiteProvider{
			ID:    provider.ID,
			Addrs: provider.Addrs,
		}

		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var out shell.IdOutput
		err = t.Kubo.Request("id", provider.ID.String()).Exec(tCtx, &out)
		if err != nil {
			cancel()
			wps.Providers = append(wps.Providers, wp)
			logEntry.WithError(err).Warnln("Couldn't identify provider", provider.ID)
			continue
		}
		cancel()

		wp.AgentVersion = out.AgentVersion
		if len(out.Addresses) >= len(wp.Addrs) {
			newAddrs := make([]multiaddr.Multiaddr, len(out.Addresses))
			for i, addr := range out.Addresses {
				newAddrs[i] = multiaddr.StringCast(addr)
			}
			wp.Addrs = newAddrs
		}

		wps.Providers = append(wps.Providers, wp)
	}

	return wps, nil
}
