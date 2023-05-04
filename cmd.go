package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

var app *cli.App

func main() {
	app = &cli.App{
		Name:    "tiros",
		Version: "0.1.0",
		Authors: []*cli.Author{
			{
				Name:  "Dennis Trautwein",
				Email: "dennis@protocol.ai",
			},
		},
		Usage:  "measures the latency of making requests to the local gateway",
		Before: Before,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Set this flag to enable debug logging",
				EnvVars: []string{"TIROS_DEBUG"},
				Value:   false,
			},
			&cli.IntFlag{
				Name:    "log-level",
				Usage:   "Set this flag to a value from 0 (least verbose) to 6 (most verbose). Overrides the --debug flag",
				EnvVars: []string{"TIROS_LOG_LEVEL"},
				Value:   4,
			},
			&cli.StringFlag{
				Name:    "telemetry-host",
				Usage:   "To which network address should the telemetry (prometheus, pprof) server bind",
				EnvVars: []string{"TIROS_TELEMETRY_HOST"},
				Value:   "localhost",
			},
			&cli.IntFlag{
				Name:    "telemetry-port",
				Usage:   "On which port should the telemetry (prometheus, pprof) server listen",
				EnvVars: []string{"TIROS_TELEMETRY_PORT"},
				Value:   6666,
			},
		},
		Commands: []*cli.Command{
			RunCommand,
		},
	}

	sigs := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	go func() {
		sig := <-sigs
		log.Printf("Received %s signal - Stopping...\n", sig.String())
		signal.Stop(sigs)
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil && !errors.Is(err, context.Canceled) {
		log.Errorf("error: %v\n", err)
		os.Exit(1)
	}

	log.Infoln("Tiros stopped.")
}

func Before(c *cli.Context) error {
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	if c.IsSet("log-level") {
		ll := c.Int("log-level")
		log.SetLevel(log.Level(ll))
		if ll == int(log.TraceLevel) {
			boil.DebugMode = true
		}
	}

	// Start prometheus metrics endpoint
	go metricsListenAndServe(c.String("telemetry-host"), c.Int("telemetry-port"))

	return nil
}

func metricsListenAndServe(host string, port int) {
	addr := fmt.Sprintf("%s:%d", host, port)
	log.WithField("addr", addr).Debugln("Starting telemetry endpoint")
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.WithError(err).Warnln("Error serving prometheus")
	}
}
