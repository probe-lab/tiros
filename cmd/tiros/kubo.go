package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	ipfs "github.com/ipfs/kubo"
	kuboclient "github.com/ipfs/kubo/client/rpc"
	iface "github.com/ipfs/kubo/core/coreiface"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

type Kubo struct {
	*kuboclient.HttpApi
}

func NewKubo(host string, port int) (*Kubo, error) {
	// ....
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithPropagators(propagator),
		),
	}

	// initializing the kubo client
	kuboAddr := net.JoinHostPort(host, strconv.Itoa(port))
	kuboClient, err := kuboclient.NewURLApiWithClient(kuboAddr, httpClient)
	if err != nil {
		return nil, fmt.Errorf("init kubo client: %w", err)
	}

	return &Kubo{HttpApi: kuboClient}, nil
}

func (k *Kubo) WaitAvailable(ctx context.Context, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for {
		select {
		case <-timeoutCtx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timeout waiting for Kubo to be ready: %w", ctx.Err())
			}
			return ctx.Err()
		case <-time.After(time.Second):
			slog.Info("Testing Kubo availability...")
			v, err := k.Version(ctx)
			if err != nil {
				continue
			}
			slog.Info("Kubo is online!", "version", v.Version)
			return nil
		}
	}
}

func (k *Kubo) Version(ctx context.Context) (*ipfs.VersionInfo, error) {
	res, err := k.Request("version").Send(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	data, err := io.ReadAll(res.Output)
	if err != nil {
		return nil, err
	}

	info := &ipfs.VersionInfo{}
	return info, json.Unmarshal(data, info)
}

func (k *Kubo) Reset(ctx context.Context) {
	pinsChan := make(chan iface.Pin)

	go func() {
		if err := k.Pin().Ls(ctx, pinsChan); err != nil {
			slog.With("err", err).Warn("Error getting pins")
		}
	}()

	for pin := range pinsChan {
		if pin.Type() != "recursive" && pin.Type() != "direct" {
			continue
		}

		slog.With("pin", pin.Path()).Info("Unpinning file from Kubo")
		if err := k.Pin().Rm(ctx, pin.Path()); err != nil {
			slog.With("err", err, "pin", pin.Path()).Warn("Error unpinning file from Kubo")
		}
	}

	slog.Info("Running repo garbage collection")
	if _, err := k.Request("repo/gc").Send(ctx); err != nil {
		slog.With("err", err).Warn("Error running ipfs gc")
	}
}
