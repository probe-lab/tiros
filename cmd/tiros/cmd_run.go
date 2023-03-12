package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/tiros/pkg/config"
	"github.com/dennis-tra/tiros/pkg/db"
)

var RunCommand = &cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.DurationFlag{
			Name:        "settle-short",
			Usage:       "the duration to wait after all daemons are online before starting the test",
			EnvVars:     []string{"TIROS_RUN_SETTLE_SHORT"},
			Value:       config.DefaultRunConfig.SettleShort,
			DefaultText: config.DefaultRunConfig.SettleShort.String(),
		},
		&cli.DurationFlag{
			Name:        "settle-long",
			Usage:       "the duration to wait after all daemons are online before starting the test",
			EnvVars:     []string{"TIROS_RUN_SETTLE_LONG"},
			Value:       config.DefaultRunConfig.SettleLong,
			DefaultText: config.DefaultRunConfig.SettleLong.String(),
		},
		&cli.StringSliceFlag{
			Name:        "websites",
			Usage:       "Websites to test against. Example: 'ipfs.io' or 'filecoin.io",
			EnvVars:     []string{"TIROS_RUN_WEBSITES"},
			Value:       cli.NewStringSlice(config.DefaultRunConfig.Websites...),
			DefaultText: strings.Join(config.DefaultRunConfig.Websites, ","),
		},
		&cli.IntFlag{
			Name:        "times",
			Usage:       "number of times to test each URL",
			EnvVars:     []string{"TIROS_RUN_TIMES"},
			Value:       config.DefaultRunConfig.Times,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.Times),
		},
		&cli.StringFlag{
			Name:        "db-host",
			Usage:       "On which host address can this clustertest reach the database",
			EnvVars:     []string{"TIROS_RUN_DATABASE_HOST"},
			Value:       config.DefaultRunConfig.DatabaseHost,
			DefaultText: config.DefaultRunConfig.DatabaseHost,
		},
		&cli.IntFlag{
			Name:        "db-port",
			Usage:       "On which port can this clustertest reach the database",
			EnvVars:     []string{"TIROS_RUN_DATABASE_PORT"},
			Value:       config.DefaultRunConfig.DatabasePort,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.DatabasePort),
		},
		&cli.StringFlag{
			Name:        "db-name",
			Usage:       "The name of the database to use",
			EnvVars:     []string{"TIROS_RUN_DATABASE_NAME"},
			Value:       config.DefaultRunConfig.DatabaseName,
			DefaultText: config.DefaultRunConfig.DatabaseName,
		},
		&cli.StringFlag{
			Name:        "db-password",
			Usage:       "The password for the database to use",
			EnvVars:     []string{"TIROS_RUN_DATABASE_PASSWORD"},
			Value:       config.DefaultRunConfig.DatabasePassword,
			DefaultText: config.DefaultRunConfig.DatabasePassword,
		},
		&cli.StringFlag{
			Name:        "db-user",
			Usage:       "The user with which to access the database to use",
			EnvVars:     []string{"TIROS_RUN_DATABASE_USER"},
			Value:       config.DefaultRunConfig.DatabaseUser,
			DefaultText: config.DefaultRunConfig.DatabaseUser,
		},
		&cli.StringFlag{
			Name:        "db-sslmode",
			Usage:       "The sslmode to use when connecting the the database",
			EnvVars:     []string{"TIROS_RUN_DATABASE_SSL_MODE"},
			Value:       config.DefaultRunConfig.DatabaseSSLMode,
			DefaultText: config.DefaultRunConfig.DatabaseSSLMode,
		},
		&cli.IntFlag{
			Name:        "kubo-api-port",
			Usage:       "port to reach the Kubo API",
			EnvVars:     []string{"TIROS_RUN_KUBO_API_PORT"},
			Value:       config.DefaultRunConfig.KuboApiPort,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.KuboApiPort),
		},
		&cli.IntFlag{
			Name:        "kubo-gateway-port",
			Usage:       "port to reach the Kubo Gateway",
			EnvVars:     []string{"TIROS_RUN_KUBO_GATEWAY_PORT"},
			Value:       config.DefaultRunConfig.KuboGatewayPort,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.KuboGatewayPort),
		},
		&cli.IntFlag{
			Name:        "chrome-cdp-port",
			Usage:       "port to reach the Chrome DevTools Protocol port",
			EnvVars:     []string{"TIROS_RUN_CHROME_CDP_PORT"},
			Value:       config.DefaultRunConfig.ChromeCDPPort,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.ChromeCDPPort),
		},
	},
	Action: RunAction,
}

func RunAction(c *cli.Context) error {
	log.Infoln("Starting Tiros standalone...")
	defer log.Infoln("Stopped Tiros standalone.")

	dbClient, err := db.InitClient(c.Context, c.String("db-host"), c.Int("db-port"), c.String("db-name"), c.String("db-user"), c.String("db-password"), c.String("db-sslmode"))
	if err != nil {
		return fmt.Errorf("init db connection: %w", err)
	}

	// verify Kubo API connection
	kubo := shell.NewShell(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.Int("kubo-api-port")))
	version, _, err := kubo.Version()
	if err != nil {
		return fmt.Errorf("kubo api offline: %w", err)
	}

	browser := rod.New().ControlURL(fmt.Sprintf("ws://127.0.0.1:%d", c.Int("chrome-cdp-port")))
	if err = browser.Connect(); err != nil {
		return fmt.Errorf("chrome cdp api offline: %w", err)
	}

	return nil
}
