package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	plcli "github.com/probe-lab/go-commons/cli"
	pldb "github.com/probe-lab/go-commons/db"
	"github.com/urfave/cli/v3"
)

var probeConfig = struct {
	Clickhouse *pldb.ClickHouseConfig
	Migrations *pldb.ClickHouseMigrationsConfig
}{
	Clickhouse: pldb.DefaultClickHouseConfig("tiros"),
	Migrations: pldb.DefaultClickHouseMigrationsConfig(),
}

var probeCmd = &cli.Command{
	Name:  "probe",
	Usage: "Start probing Kubo",
	Flags: slices.Concat(
		plcli.ClickHouseFlags("TIROS_PROBE_", probeConfig.Clickhouse),
		plcli.ClickHouseMigrationsFlags("TIROS_PROBE_", probeConfig.Migrations),
	),
	Before: probeBefore,
	After:  probeAfter,
	Commands: []*cli.Command{
		probeKuboCmd,
	},
}

func probeBefore(ctx context.Context, c *cli.Command) (context.Context, error) {
	slog.Info("Start probing Kubo...")

	if err := probeConfig.Clickhouse.Validate(); err != nil {
		return ctx, fmt.Errorf("invalid clickhouse config: %w", err)
	}

	return ctx, nil
}

func probeAfter(ctx context.Context, c *cli.Command) error {
	slog.Info("Stopped probing Kubo.")
	return nil
}
