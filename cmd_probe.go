package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	plcli "github.com/probe-lab/go-commons/cli"
	pldb "github.com/probe-lab/go-commons/db"
	"github.com/urfave/cli/v3"
)

var probeConfig = struct {
	DryRun     bool
	JSONOut    string
	Timeout    time.Duration
	Clickhouse *pldb.ClickHouseConfig
	Migrations *pldb.ClickHouseMigrationsConfig
}{
	DryRun:     false,
	JSONOut:    "",
	Timeout:    0,
	Clickhouse: pldb.DefaultClickHouseConfig("tiros"),
	Migrations: pldb.DefaultClickHouseMigrationsConfig(),
}

var probeCmd = &cli.Command{
	Name:  "probe",
	Usage: "Start probing Kubo",
	Flags: slices.Concat(
		probeFlags,
		plcli.ClickHouseFlags("TIROS_PROBE_", probeConfig.Clickhouse),
		plcli.ClickHouseMigrationsFlags("TIROS_PROBE_", probeConfig.Migrations),
	),
	Before: probeBefore,
	After:  probeAfter,
	Commands: []*cli.Command{
		probeKuboCmd,
	},
}

var probeFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:        "dry.run",
		Usage:       "Whether to skip DB interactions",
		Sources:     cli.EnvVars("TIROS_PROBE_DRY_RUN"),
		Value:       probeConfig.DryRun,
		Destination: &probeConfig.DryRun,
	},
	&cli.StringFlag{
		Name:        "json.out",
		Usage:       "Write measurements to JSON files in the given directory",
		Sources:     cli.EnvVars("TIROS_PROBE_JSON_OUT"),
		Value:       probeConfig.JSONOut,
		Destination: &probeConfig.JSONOut,
	},
	&cli.DurationFlag{
		Name:        "timeout",
		Usage:       "The maximum allowed time for this experiment to run (0 no timeout)",
		Sources:     cli.EnvVars("TIROS_PROBE_TIMEOUT"),
		Value:       probeConfig.Timeout,
		Destination: &probeConfig.Timeout,
	},
}

func probeBefore(ctx context.Context, c *cli.Command) (context.Context, error) {
	defer slog.Info("Start probing Kubo...")

	if err := probeConfig.Clickhouse.Validate(); err != nil {
		return ctx, fmt.Errorf("invalid clickhouse config: %w", err)
	}

	if probeConfig.Timeout > 0 {
		ctx, _ = context.WithTimeout(ctx, probeConfig.Timeout) // would be cleaner to call `cancel` eventually
	}

	return ctx, nil
}

func newDBClient(ctx context.Context) (DBClient, error) {
	var (
		dbClient DBClient
		err      error
	)
	if probeConfig.DryRun {
		dbClient = NewNoopClient()
	} else if probeConfig.JSONOut != "" {
		dbClient, err = NewJSONClient(probeConfig.JSONOut)
		if err != nil {
			return nil, fmt.Errorf("connecting to json client: %w", err)
		}
	} else {
		dbClient, err = NewClickhouseClient(ctx, probeConfig.Clickhouse, probeConfig.Migrations)
		if err != nil {
			return nil, fmt.Errorf("connecting to clickhouse: %w", err)
		}
	}
	return dbClient, nil
}

func probeAfter(ctx context.Context, c *cli.Command) error {
	slog.Info("Stopped probing Kubo.")
	return nil
}
