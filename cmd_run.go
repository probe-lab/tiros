package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
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
		&cli.StringFlag{
			Name:    "kubo-host",
			Usage:   "port to reach the Kubo Gateway",
			EnvVars: []string{"TIROS_RUN_KUBO_HOST"},
			Value:   "localhost",
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

	// Initialize database client
	var err error
	var dbClient IDBClient = DBDummyClient{}
	if !c.Bool("dry-run") {
		dbClient, err = InitClient(c.Context, c.String("db-host"), c.Int("db-port"), c.String("db-name"), c.String("db-user"), c.String("db-password"), c.String("db-sslmode"))
		if err != nil {
			return fmt.Errorf("init db connection: %w", err)
		}
	}

	// Initialize kubo client
	kubo := shell.NewShell(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.Int("kubo-api-port")))

	// configure Tiros struct
	t := Tiros{
		DBClient: dbClient,
		Kubo:     kubo,
	}

	// Create a measurement run entry in the database. This entry will
	// contain information about the measurement configuration.
	if _, err := t.InitRun(c); err != nil {
		return fmt.Errorf("init run: %w", err)
	}
	// Before we're completely exiting we "seal" the run entry. Right now, this only means we're setting the
	// finished_at timestamp.
	defer func() {
		if _, err = t.DBClient.SealRun(context.Background(), t.DBRun); err != nil {
			log.WithError(err).Warnln("Couldn't seal run")
		}
	}()

	// shuffle websites, so that we have a different order in which we request the websites.
	// If we didn't do this a single website would always be requested with a comparatively "cold" kubo node.
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

	providerResults := make(chan *provider)
	probeResults := make(chan *probeResult)

	go t.measureWebsites(c, websites, probeResults)
	go t.findAllProviders(c, websites, providerResults)

	for {
		select {
		case pr, more := <-probeResults:
			if !more {
				probeResults = nil
			} else {
				log.Infoln("Handling probe result")
				if _, err := t.DBClient.SaveMeasurement(c, t.DBRun, pr); err != nil {
					return fmt.Errorf("save measurement: %w", err)
				}
			}

		case pr, more := <-providerResults:
			if !more {
				providerResults = nil
			} else {
				log.Infoln("Handling provider result")
				_, err := t.DBClient.SaveProvider(c, t.DBRun, pr)
				if err != nil {
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

func (t *Tiros) measureWebsites(c *cli.Context, websites []string, results chan<- *probeResult) {
	defer close(results)

	for _, settle := range c.IntSlice("settle-times") {

		sleepDur := time.Duration(settle) * time.Second

		log.Infof("Letting Kubo settle for %s\n", sleepDur)
		time.Sleep(sleepDur)

		for i := 0; i < c.Int("times"); i++ {
			for _, mType := range []string{models.MeasurementTypeKUBO, models.MeasurementTypeHTTP} {
				for _, website := range websites {
					pr, err := t.probeWebsite(c, websiteURL(c, website, mType))
					if err != nil {
						log.WithError(err).WithField("website", website).Warnln("error probing website")
						continue
					}

					pr.website = website
					pr.mType = mType
					pr.try = i

					log.WithFields(log.Fields{
						"ttfb": p2f(pr.ttfb),
						"lcp":  p2f(pr.lcp),
						"fcp":  p2f(pr.fcp),
						"tti":  p2f(pr.tti),
					}).WithError(pr.err).Infoln("Probed website", website)

					results <- pr

					if mType == models.MeasurementTypeKUBO {
						if err = t.KuboGC(c.Context); err != nil {
							log.WithError(err).Warnln("error running kubo gc")
							continue
						}
					}
				}
			}
		}
	}
}

func websiteURL(c *cli.Context, website string, mType string) string {
	switch mType {
	case models.MeasurementTypeKUBO:
		return fmt.Sprintf("http://%s:%d/ipns/%s", c.String("kubo-host"), c.Int("kubo-gateway-port"), website)
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
