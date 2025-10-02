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

	"github.com/aarondl/sqlboiler/v4/boil"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

var app *cli.App

var rootConfig = struct {
	Debug         bool
	LogLevel      int
	TelemetryHost string
	TelemetryPort int
	EnableTracing bool
	Region        string

	tracerProvider *sdktrace.TracerProvider
}{
	Debug:         false,
	LogLevel:      int(log.InfoLevel),
	TelemetryHost: "localhost",
	TelemetryPort: 6666,
	EnableTracing: false,
	Region:        "local",
}

func main() {
	app = &cli.App{
		Name:    "tiros",
		Version: "0.3.0",
		Authors: []*cli.Author{
			{
				Name:  "Dennis Trautwein",
				Email: "dennis@probelab.io",
			},
		},
		Usage:  "measures the latency of making requests to the local gateway",
		Before: rootBefore,
		After:  rootAfter,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Set this flag to enable debug logging",
				EnvVars:     []string{"TIROS_DEBUG"},
				Value:       rootConfig.Debug,
				Destination: &rootConfig.Debug,
			},
			&cli.IntFlag{
				Name:        "log-level",
				Usage:       "Set this flag to a value from 0 (least verbose) to 6 (most verbose). Overrides the --debug flag",
				EnvVars:     []string{"TIROS_LOG_LEVEL"},
				Value:       rootConfig.LogLevel,
				Destination: &rootConfig.LogLevel,
			},
			&cli.StringFlag{
				Name:        "telemetry-host",
				Usage:       "To which network address should the telemetry (prometheus, pprof) server bind",
				EnvVars:     []string{"TIROS_TELEMETRY_HOST"},
				Value:       rootConfig.TelemetryHost,
				Destination: &rootConfig.TelemetryHost,
			},
			&cli.IntFlag{
				Name:        "telemetry-port",
				Usage:       "On which port should the telemetry (prometheus, pprof) server listen",
				EnvVars:     []string{"TIROS_TELEMETRY_PORT"},
				Value:       rootConfig.TelemetryPort,
				Destination: &rootConfig.TelemetryPort,
			},
			&cli.BoolFlag{
				Name:        "tracing",
				Usage:       "Set this flag to enable OTEL tracing",
				EnvVars:     []string{"TIROS_TRACING"},
				Value:       rootConfig.EnableTracing,
				Destination: &rootConfig.EnableTracing,
			},
			&cli.StringFlag{
				Name:        "region",
				Usage:       "In which region is this tiros instance running",
				EnvVars:     []string{"AWS_REGION", "TIROS_REGION"},
				Value:       rootConfig.Region,
				Destination: &rootConfig.Region,
			},
		},
		Commands: []*cli.Command{
			probeCmd,
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

func rootBefore(c *cli.Context) error {
	if rootConfig.Debug {
		log.SetLevel(log.DebugLevel)
	}

	if c.IsSet("log-level") {
		log.SetLevel(log.Level(rootConfig.LogLevel))
		if rootConfig.LogLevel == int(log.TraceLevel) {
			boil.DebugMode = true
		}
	}

	// Start prometheus metrics endpoint
	go metricsListenAndServe(rootConfig.TelemetryHost, rootConfig.TelemetryPort)

	tp, err := initTracing(c.Context)
	if err != nil {
		return err
	}

	rootConfig.tracerProvider = tp

	return nil
}

func rootAfter(c *cli.Context) error {
	if rootConfig.tracerProvider != nil {
		if err := rootConfig.tracerProvider.Shutdown(c.Context); err != nil {
			log.WithError(err).Warnln("Error shutting down tracer provider")
		}
	}

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

func initTracing(ctx context.Context) (*sdktrace.TracerProvider, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("tiros"),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return tp, nil
}
