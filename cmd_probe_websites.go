package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"time"

	kuboclient "github.com/ipfs/kubo/client/rpc"
	"github.com/probe-lab/tiros/models"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var probeWebsitesConfig = struct {
	Websites        *cli.StringSlice
	SettleTimes     *cli.IntSlice
	Probes          int
	LookupProviders bool
	RunTimeout      time.Duration
}{
	Websites:        cli.NewStringSlice(),
	SettleTimes:     cli.NewIntSlice(10, 1200),
	Probes:          3,
	LookupProviders: true,
	RunTimeout:      0,
}

var probeWebsitesCmd = &cli.Command{
	Name: "websites",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:        "websites",
			Usage:       "Websites to test against. Example: 'ipfs.io' or 'filecoin.io",
			EnvVars:     []string{"TIROS_PROBE_WEBSITES_WEBSITES"},
			Value:       probeWebsitesConfig.Websites,
			Destination: probeWebsitesConfig.Websites,
		},
		&cli.IntSliceFlag{
			Name:        "settle-times",
			Usage:       "a list of times to settle in seconds",
			EnvVars:     []string{"TIROS_PROBE_WEBSITES_SETTLE_TIMES"},
			Value:       probeWebsitesConfig.SettleTimes,
			Destination: probeWebsitesConfig.SettleTimes,
		},
		&cli.IntFlag{
			Name:        "times",
			Usage:       "number of times to test each URL",
			EnvVars:     []string{"TIROS_PROBE_WEBSITES_TIMES"},
			Value:       probeWebsitesConfig.Probes,
			Destination: &probeWebsitesConfig.Probes,
		},
		&cli.BoolFlag{
			Name:        "lookup-providers",
			Usage:       "Whether to lookup website providers",
			EnvVars:     []string{"TIROS_PROBE_WEBSITES_LOOKUP_PROVIDERS"},
			Value:       probeWebsitesConfig.LookupProviders,
			Destination: &probeWebsitesConfig.LookupProviders,
		},
		&cli.DurationFlag{
			Name:        "timeout",
			Usage:       "The maximum allowed time for this experiment to run (0 no timeout)",
			EnvVars:     []string{"TIROS_PROBE_WEBSITES_TIMEOUT"},
			Value:       probeWebsitesConfig.RunTimeout,
			Destination: &probeWebsitesConfig.RunTimeout,
		},
	},
	Action: RunAction,
}

type tiros struct {
	dbClient IDBClient
	ipfs     *kuboclient.HttpApi
	dbRun    *models.Run
}

func RunAction(c *cli.Context) error {
	log.Infoln("Starting Tiros run...")
	defer log.Infoln("Stopped Tiros run.")

	// create global timeout context
	if probeWebsitesConfig.RunTimeout > 0 {
		ctx, cancel := context.WithTimeout(c.Context, probeWebsitesConfig.RunTimeout)
		defer cancel()
		c.Context = ctx
	}

	// Initialize database client
	dbClient, err := newDBClient(c.Context)
	if err != nil {
		return fmt.Errorf("init database client: %w", err)
	}

	ipfsClient, err := newKuboClient()
	if err != nil {
		return fmt.Errorf("init ipfs client: %w", err)
	}

	// configure tiros struct
	t := tiros{
		dbClient: dbClient,
		ipfs:     ipfsClient,
	}

	// Create a measurement run entry in the database. This entry will
	// contain information about the measurement configuration.
	if _, err := t.InitRun(c); err != nil {
		return fmt.Errorf("init run: %w", err)
	}
	// rootBefore we're completely exiting we "seal" the run entry. Right now, this only means we're setting the
	// finished_at timestamp.
	defer func() {
		if _, err = t.dbClient.SealRun(context.Background(), t.dbRun); err != nil {
			log.WithError(err).Warnln("Couldn't seal run")
		}
	}()

	// shuffle websites, so that we have a different order in which we request the websites.
	// If we didn't do this a single website would always be requested with a comparatively "cold" ipfs node.
	websites := probeWebsitesConfig.Websites.Value()
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(websites), func(i, j int) {
		websites[i], websites[j] = websites[j], websites[i]
	})

	log.WithFields(log.Fields{
		"websites":    websites,
		"settleTimes": probeWebsitesConfig.SettleTimes.Value(),
		"times":       probeWebsitesConfig.Probes,
	}).Infoln("Starting run!")

	providerResults := make(chan *provider)
	probeResults := make(chan *probeResult)

	go t.measureWebsites(c, websites, probeResults)

	if probeWebsitesConfig.LookupProviders {
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

func (t *tiros) InitRun(c *cli.Context) (*models.Run, error) {
	vinfo, err := kuboVersion(c.Context, t.ipfs)
	if err != nil {
		return nil, fmt.Errorf("ipfs api offline: %w", err)
	}

	ipfsImpl := probeConfig.IPFS.Implementation
	dbRun, err := t.dbClient.InsertRun(c, ipfsImpl, fmt.Sprintf("%s-%s", vinfo.Version, vinfo.Commit))
	if err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	t.dbRun = dbRun

	return t.dbRun, nil
}

type versionInfo struct {
	Version string
	Commit  string
	Repo    string
	System  string
	Golang  string
}

func kuboVersion(ctx context.Context, client *kuboclient.HttpApi) (*versionInfo, error) {
	res, err := client.Request("version").Send(ctx)
	if err != nil {
		panic(err)
	}
	defer res.Close()

	data, err := io.ReadAll(res.Output)
	if err != nil {
		panic(err)
	}

	info := &versionInfo{}
	return info, json.Unmarshal(data, info)
}
