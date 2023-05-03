package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
)

type WebsiteProviders struct {
	Website   string
	Path      string
	Providers []WebsiteProvider
}

type WebsiteProvider struct {
	ID           peer.ID
	Addrs        []multiaddr.Multiaddr
	AgentVersion string
}

type NameResolveResponse struct {
	Path string
}

func (t *Tiros) FindAllProvidersAsync(ctx context.Context, websites []string) <-chan []*WebsiteProviders {
	results := make(chan []*WebsiteProviders)
	go func() {
		wps := []*WebsiteProviders{}
		for _, website := range websites {
			wp, err := t.FindProviders(ctx, website)
			if err != nil {
				log.WithError(err).WithField("website", website).Warnln("Couldn't find providers")
				if wp == nil {
					continue
				}
			}
			wps = append(wps, wp)
		}
		results <- wps
		close(results)
	}()

	return results
}

func (t *Tiros) FindProviders(ctx context.Context, website string) (*WebsiteProviders, error) {
	wps := &WebsiteProviders{
		Website:   website,
		Providers: []WebsiteProvider{},
	}
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	resp, err := t.Kubo.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return wps, fmt.Errorf("name/resolve: %w", err)
	}

	dat, err := io.ReadAll(resp.Output)
	if err != nil {
		return wps, fmt.Errorf("read name/resolve bytes: %w", err)
	}

	nrr := NameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return wps, fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	wps.Path = nrr.Path

	resp, err = t.Kubo.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "100").
		Send(ctx)
	if err != nil {
		return wps, fmt.Errorf("routing/findprovs: %w", err)
	}

	providers := []*peer.AddrInfo{}
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		evt := routing.QueryEvent{}
		if err = dec.Decode(&evt); err != nil {
			return wps, fmt.Errorf("decode routing/findprovs response: %w", err)
		}
		if evt.Type != routing.Provider {
			continue
		}

		if len(evt.Responses) != 1 {
			logEntry.Warnln("findprovs Providerquery event with != 1 responses:", len(evt.Responses))
			continue
		}

		providers = append(providers, evt.Responses[0])
	}

	for _, provider := range providers {
		wp := WebsiteProvider{
			ID:    provider.ID,
			Addrs: provider.Addrs,
		}

		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var out shell.IdOutput
		err = t.Kubo.Request("id", provider.ID.String()).Exec(tCtx, &out)
		if err != nil {
			cancel()
			wps.Providers = append(wps.Providers, wp)
			logEntry.WithError(err).Warnln("Couldn't identify provider", provider.ID)
			continue
		}
		cancel()

		wp.AgentVersion = out.AgentVersion
		if len(out.Addresses) >= len(wp.Addrs) {
			newAddrs := make([]multiaddr.Multiaddr, len(out.Addresses))
			for i, addr := range out.Addresses {
				newAddrs[i] = multiaddr.StringCast(addr)
			}
			wp.Addrs = newAddrs
		}

		wps.Providers = append(wps.Providers, wp)
	}

	return wps, nil
}
