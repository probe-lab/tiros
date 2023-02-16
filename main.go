package main

import (
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
)

var app *cli.App

// example invocation: ./tiros --region eu-north-1 --versions v0.17.0,v0.16.0,v0.15.0 --nodes-per-version 5 --settle 10s --urls /ipns/filecoin.io,/ipns/ipfs.io --times 5 --cluster aws
func main() {
	app = &cli.App{
		Name:    "tiros",
		Version: "0.1.0",
		Usage:   "measures the latency of making requests to the local gateway",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "versions",
				Usage:   "the kubo versions to test (comma-separated), e.g. 'v0.16.0,v0.17.0'.",
				Value:   cli.NewStringSlice("v0.17.0"),
				EnvVars: []string{"TIROS_VERSIONS"},
			},
			&cli.IntFlag{
				Name:    "nodes-per-version",
				Usage:   "the number of nodes per version to run",
				Value:   1,
				EnvVars: []string{"TIROS_NODES_PER_VERSION"},
			},
			&cli.StringSliceFlag{
				Name:    "regions",
				Usage:   "the AWS regions to use, if using an AWS cluster",
				EnvVars: []string{"TIROS_REGIONS"},
			},
			&cli.DurationFlag{
				Name:    "settle-short",
				Usage:   "the duration to wait after all daemons are online before starting the test",
				Value:   10 * time.Second,
				EnvVars: []string{"TIROS_SETTLE_SHORT"},
			},
			&cli.DurationFlag{
				Name:    "settle-long",
				Usage:   "the duration to wait after all daemons are online before starting the test",
				Value:   20 * time.Minute,
				EnvVars: []string{"TIROS_SETTLE_LONG"},
			},
			&cli.StringSliceFlag{
				Name:     "urls",
				Usage:    "URLs to test against, relative to the gateway URL. Example: '/ipns/ipfs.io'",
				Required: true,
				EnvVars:  []string{"TIROS_URLS"},
			},
			&cli.IntFlag{
				Name:    "times",
				Usage:   "number of times to test each URL",
				Value:   5,
				EnvVars: []string{"TIROS_TIMES"},
			},
			&cli.StringFlag{
				Name:    "nodeagent",
				Usage:   "path to the nodeagent binary",
				Value:   "/home/tiros/nodeagent", // correct if you use the default docker image
				EnvVars: []string{"TIROS_NODEAGENT_BIN"},
			},
			&cli.StringFlag{
				Name:    "instance-type",
				Usage:   "the EC2 instance type to run the experiment on",
				Value:   "t2.micro",
				EnvVars: []string{"TIROS_INSTANCE_TYPE"},
			},
			&cli.StringFlag{
				Name:    "db-host",
				Usage:   "On which host address can this clustertest reach the database",
				EnvVars: []string{"TIROS_DATABASE_HOST"},
			},
			&cli.IntFlag{
				Name:    "db-port",
				Usage:   "On which port can this clustertest reach the database",
				EnvVars: []string{"TIROS_DATABASE_PORT"},
			},
			&cli.StringFlag{
				Name:    "db-name",
				Usage:   "The name of the database to use",
				EnvVars: []string{"TIROS_DATABASE_NAME"},
			},
			&cli.StringFlag{
				Name:    "db-password",
				Usage:   "The password for the database to use",
				EnvVars: []string{"TIROS_DATABASE_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "db-user",
				Usage:   "The user with which to access the database to use",
				EnvVars: []string{"TIROS_DATABASE_USER"},
			},
			&cli.StringFlag{
				Name:    "db-sslmode",
				Usage:   "The sslmode to use when connecting the the database",
				EnvVars: []string{"TIROS_DATABASE_SSL_MODE"},
			},
			&cli.StringSliceFlag{
				Name:    "public-subnet-ids",
				Usage:   "The public subnet IDs to run the cluster in",
				EnvVars: []string{"TIROS_PUBLIC_SUBNET_IDS"},
			},
			&cli.StringSliceFlag{
				Name:    "instance-profile-arns",
				Usage:   "The instance profiles to run the Kubo nodes with",
				EnvVars: []string{"TIROS_INSTANCE_PROFILE_ARNS"},
			},
			&cli.StringSliceFlag{
				Name:    "instance-security-group-ids",
				Usage:   "The security groups of the Kubo instances",
				EnvVars: []string{"TIROS_SECURITY_GROUP_IDS"},
			},
			&cli.StringSliceFlag{
				Name:    "s3-bucket-arns",
				Usage:   "The S3 buckets where the nodeagent binaries are stored",
				EnvVars: []string{"TIROS_S3_BUCKET_ARNS"},
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "Whether to enable verbose logging",
				EnvVars: []string{"TIROS_VERBOSE"},
			},
		},
		Action: func(c *cli.Context) error {
			conf, err := configFromContext(c)
			if err != nil {
				return fmt.Errorf("parsing context to config: %w", err)
			}
			return Action(c.Context, conf)
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println("os-args", os.Args)
		fmt.Println(err)
		os.Exit(1)
	}
}
