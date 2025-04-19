package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/nebula-crawler/pkg/maxmind"
	"github.com/dennis-tra/nebula-crawler/pkg/udger"
	"github.com/probe-lab/tiros/models"
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
			Name:    "lookup-providers",
			Usage:   "Whether to lookup website providers",
			EnvVars: []string{"TIROS_RUN_LOOKUP_PROVIDERS"},
			Value:   true,
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
			Name:    "ipfs-host",
			Usage:   "host at which to reach the IPFS Gateway",
			EnvVars: []string{"TIROS_RUN_IPFS_HOST", "TIROS_RUN_KUBO_HOST" /* <- legacy */},
			Value:   "localhost",
		},
		&cli.StringFlag{
			Name:    "ipfs-api-host",
			Usage:   "port to reach a Kubo-compatible RPC API",
			EnvVars: []string{"TIROS_RUN_IPFS_API_HOST"},
			Value:   "127.0.0.1",
		},
		&cli.IntFlag{
			Name:    "ipfs-api-port",
			Usage:   "port to reach a Kubo-compatible RPC API",
			EnvVars: []string{"TIROS_RUN_IPFS_API_PORT", "TIROS_RUN_KUBO_API_PORT" /* <- legacy */},
			Value:   5001,
		},
		&cli.IntFlag{
			Name:    "ipfs-gateway-port",
			Usage:   "port to reach the IPFS Gateway",
			EnvVars: []string{"TIROS_RUN_IPFS_GATEWAY_PORT", "TIROS_RUN_KUBO_GATEWAY_PORT" /* <- legacy */},
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
		&cli.StringFlag{
			Name:    "udger-db",
			Usage:   "Path to the Udger DB",
			EnvVars: []string{"TIROS_UDGER_DB_PATH"},
		},
		&cli.StringFlag{
			Name:    "ipfs-implementation",
			Usage:   "Which implementation are we testing (KUBO, HELIA)",
			EnvVars: []string{"TIROS_IPFS_IMPLEMENTATION"},
			Value:   "KUBO",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Usage:   "The maximum allowed time for this experiment to run (0 no timeout)",
			EnvVars: []string{"TIROS_RUN_TIMEOUT"},
			Value:   time.Duration(0),
		},
	},
	Action: RunAction,
}

type tiros struct {
	dbClient IDBClient
	ipfs     *shell.Shell
	dbRun    *models.Run
	mmClient *maxmind.Client
	uClient  *udger.Client
}

func RunAction(c *cli.Context) error {
	log.Infoln("Starting Tiros run...")
	defer log.Infoln("Stopped Tiros run.")

	// create global timeout context
	timeout := c.Duration("timeout")
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(c.Context, timeout)
		defer cancel()
		c.Context = ctx
	}

	// Initialize database client
	var err error
	var dbClient IDBClient = DBDummyClient{}
	if !c.Bool("dry-run") {
		dbClient, err = InitClient(c.Context, c.String("db-host"), c.Int("db-port"), c.String("db-name"), c.String("db-user"), c.String("db-password"), c.String("db-sslmode"))
		if err != nil {
			return fmt.Errorf("init db connection: %w", err)
		}
	}

	// Initialize ipfs client
	var ipfsClient *shell.Shell
	if c.Int("ipfs-api-port") != 0 {
		ipfsClient = shell.NewShell(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.Int("ipfs-api-port")))
	}

	// Initialize maxmind client
	mmClient, err := maxmind.NewClient()
	if err != nil {
		return fmt.Errorf("new maxmind client: %w", err)
	}

	// Initialize udger db client
	uClient, err := udger.NewClient(c.String("udger-db"))
	if err != nil {
		return fmt.Errorf("new udger db client: %w", err)
	}

	// configure tiros struct
	t := tiros{
		dbClient: dbClient,
		ipfs:     ipfsClient,
		mmClient: mmClient,
		uClient:  uClient,
	}

	// Create a measurement run entry in the database. This entry will
	// contain information about the measurement configuration.
	if _, err := t.InitRun(c); err != nil {
		return fmt.Errorf("init run: %w", err)
	}
	// Before we're completely exiting we "seal" the run entry. Right now, this only means we're setting the
	// finished_at timestamp.
	defer func() {
		if _, err = t.dbClient.SealRun(context.Background(), t.dbRun); err != nil {
			log.WithError(err).Warnln("Couldn't seal run")
		}
	}()

	// shuffle websites, so that we have a different order in which we request the websites.
	// If we didn't do this a single website would always be requested with a comparatively "cold" ipfs node.
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

	if c.Bool("lookup-providers") {
		go t.findAllProviders(c, websites, providerResults)
	} else {
		close(providerResults)
	}

	for {
		log.Infoln("Awaiting Provider or Probe result...")
		select {
		case pr, more := <-probeResults:
			if !more {
				log.Infoln("Probing websites done!")
				probeResults = nil
			} else {
				log.WithField("url", pr.url).Infoln("Handling probe result")
				if _, err := t.dbClient.SaveMeasurement(c, t.dbRun, pr); err != nil {
					return fmt.Errorf("save measurement: %w", err)
				}
			}

		case pr, more := <-providerResults:
			if !more {
				log.Infoln("Searching for providers done!")
				providerResults = nil
			} else {
				if errors.Is(pr.err, context.DeadlineExceeded) {
					pr.err = context.DeadlineExceeded
				}
				log.WithError(pr.err).
					WithField("peerID", pr.id.String()[:16]).
					WithField("website", pr.website).
					Infoln("Handling provider result")
				_, err := t.dbClient.SaveProvider(c, t.dbRun, pr)
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

// GetIpfsVersion tries to retrieve the IPFS version using the IPFS API client.
// If it fails, it falls back to an HTTP request to the specified endpoint, using host and port from the context.
func (t *tiros) GetIpfsVersion(c *cli.Context) (version string, sha string, err error) {
	version, sha, err = t.ipfs.Version()
	if err == nil {
		return version, sha, nil
	}

	// Use host and port from the context for the fallback HTTP request
	apiHost := c.String("ipfs-api-host")
	apiPort := c.Int("ipfs-api-port")
	apiUrl := fmt.Sprintf("http://%s:%d/api/v0/version", apiHost, apiPort)
	// log that we're falling back to HTTP url for version
	log.WithField("url", apiUrl).Debugln("Falling back to HTTP request for version")

	resp, httpErr := http.Get(apiUrl)
	if httpErr != nil {
		return "", "", fmt.Errorf("ipfs api and http fallback both failed: %v, fallback error: %w", err, httpErr)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", "", fmt.Errorf("error reading version endpoint response: %w", readErr)
	}
	// log the entire body of the response
	log.WithField("body", string(body)).Debugln("HTTP response body")

	var data struct {
		Version string `json:"Version"`
		Commit  string `json:"Commit"`
	}

	jsonErr := json.Unmarshal(body, &data)
	if jsonErr != nil {
		return "", "", fmt.Errorf("error parsing version endpoint JSON: %w", jsonErr)
	}

	return data.Version, data.Commit, nil
}

func (t *tiros) InitRun(c *cli.Context) (*models.Run, error) {
	version, sha, err := t.GetIpfsVersion(c)
	if err != nil {
		return nil, fmt.Errorf("ipfs api offline: %w", err)
	}

	ipfsImpl := c.String("ipfs-implementation")
	dbRun, err := t.dbClient.InsertRun(c, ipfsImpl, fmt.Sprintf("%s-%s", version, sha))
	if err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	t.dbRun = dbRun

	return t.dbRun, nil
}
