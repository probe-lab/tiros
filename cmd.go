package main

import (
	"log/slog"
	"os"

	plcli "github.com/probe-lab/go-commons/cli"
	"github.com/urfave/cli/v3"
)

var (
	rootCmd    *plcli.RootCommand
	rootConfig *plcli.RootCommandConfig
)

func main() {
	rootCmd, rootConfig = plcli.NewRootCommand(&cli.Command{
		Name: "tiros",
		Commands: []*cli.Command{
			probeCmd,
			plcli.NewHealthCommand(),
		},
	})

	if err := rootCmd.Run(); err != nil {
		slog.Error("terminated abnormally", "err", err)
		os.Exit(1)
	}
}
