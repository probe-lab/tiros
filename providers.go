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
	"github.com/urfave/cli/v2"
)

type provider struct {
	website string
	path    string
	id      peer.ID
	addrs   []multiaddr.Multiaddr
	agent   string
	err     error
}

type nameResolveResponse struct {
	Path string
}

func (t *Tiros) findAllProviders(c *cli.Context, websites []string, results chan<- *provider) {
	defer close(results)
	for _, website := range websites {
		providers, err := t.FindProviders(c.Context, website)
		if err != nil {
			log.WithError(err).WithField("website", website).Warnln("Couldn't find providers")
			if providers == nil {
				continue
			}
		}

		for _, provider := range providers {
			results <- provider
		}
	}
}

func (t *Tiros) FindProviders(ctx context.Context, website string) ([]*provider, error) {
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	var providers []*provider

	resp, err := t.Kubo.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("name/resolve: %w", err)
	}

	dat, err := io.ReadAll(resp.Output)
	if err != nil {
		return nil, fmt.Errorf("read name/resolve bytes: %w", err)
	}

	nrr := nameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return nil, fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	resp, err = t.Kubo.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "100").
		Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("routing/findprovs: %w", err)
	}

	var providerPeers []*peer.AddrInfo
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		evt := routing.QueryEvent{}
		if err = dec.Decode(&evt); err != nil {
			return nil, fmt.Errorf("decode routing/findprovs response: %w", err)
		}
		if evt.Type != routing.Provider {
			continue
		}

		if len(evt.Responses) != 1 {
			logEntry.Warnln("findprovs Providerquery event with != 1 responses:", len(evt.Responses))
			continue
		}

		providerPeers = append(providerPeers, evt.Responses[0])
	}

	for _, providerPeer := range providerPeers {
		provider := &provider{
			website: website,
			path:    nrr.Path,
			id:      providerPeer.ID,
			addrs:   providerPeer.Addrs,
		}

		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var out shell.IdOutput
		err = t.Kubo.Request("id", provider.id.String()).Exec(tCtx, &out)
		if err != nil {
			logEntry.WithError(err).Warnln("Couldn't identify provider", provider.id)
			provider.err = err
		} else {
			provider.agent = out.AgentVersion

			// if we found different addrs, overwrite from ID lookup
			if len(out.Addresses) != len(providerPeer.Addrs) {
				newAddrs := make([]multiaddr.Multiaddr, len(out.Addresses))
				for i, addr := range out.Addresses {
					newAddrs[i] = multiaddr.StringCast(addr)
				}
				provider.addrs = newAddrs
			}
		}
		cancel()

		providers = append(providers, provider)
	}

	return providers, nil
}
