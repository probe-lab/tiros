package main

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/tiros/pkg/config"
	"github.com/dennis-tra/tiros/pkg/tiros"
)

var RunCommand = &cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:        "versions",
			Usage:       "the kubo versions to test (comma-separated), e.g. 'v0.16.0,v0.17.0'.",
			EnvVars:     []string{"TIROS_RUN_VERSIONS"},
			Value:       cli.NewStringSlice(config.DefaultRunConfig.Versions...),
			DefaultText: strings.Join(config.DefaultRunConfig.Versions, ","),
		},
		&cli.IntFlag{
			Name:        "nodes-per-version",
			Usage:       "the number of nodes per version to run",
			EnvVars:     []string{"TIROS_RUN_NODES_PER_VERSION"},
			Value:       config.DefaultRunConfig.NodesPerVersion,
			DefaultText: strconv.Itoa(config.DefaultRunConfig.NodesPerVersion),
		},
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
			Name:        "nodeagent",
			Usage:       "path to the nodeagent binary",
			EnvVars:     []string{"TIROS_RUN_NODEAGENT_BIN"},
			Value:       config.DefaultRunConfig.NodeAgent,
			DefaultText: config.DefaultRunConfig.NodeAgent,
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
		&cli.StringFlag{
			Name:        "instance-type",
			Usage:       "the EC2 instance type to run the experiment on",
			EnvVars:     []string{"TIROS_RUN_INSTANCE_TYPE"},
			Value:       config.DefaultRunConfig.InstanceType,
			DefaultText: config.DefaultRunConfig.InstanceType,
		},
	},
	Subcommands: []*cli.Command{
		RunLocalCommand,
		RunAWSCommand,
	},
}

func RunAction(c *cli.Context, exp *tiros.Experiment) error {
	log.Infoln("Starting Tiros run...")

	if err := exp.Init(c.Context); err != nil {
		return fmt.Errorf("init experiment: %w", err)
	}

	if err := exp.Run(c.Context); err != nil {
		return fmt.Errorf("run experiment: %w", err)
	}

	return nil
}
