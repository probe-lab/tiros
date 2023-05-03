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
		err := t.findProviders(c.Context, website, results)
		if err != nil {
			log.WithError(err).WithField("website", website).Warnln("Couldn't find providers")
		}
	}
}

func (t *Tiros) findProviders(ctx context.Context, website string, results chan<- *provider) error {
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	resp, err := t.Kubo.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return fmt.Errorf("name/resolve: %w", err)
	}

	dat, err := io.ReadAll(resp.Output)
	if err != nil {
		return fmt.Errorf("read name/resolve bytes: %w", err)
	}

	nrr := nameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	resp, err = t.Kubo.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "100").
		Send(ctx)
	if err != nil {
		return fmt.Errorf("routing/findprovs: %w", err)
	}

	var providerPeers []*peer.AddrInfo
	dec := json.NewDecoder(resp.Output)
	for dec.More() {
		evt := routing.QueryEvent{}
		if err = dec.Decode(&evt); err != nil {
			return fmt.Errorf("decode routing/findprovs response: %w", err)
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

	numJobs := len(providerPeers)
	idJobs := make(chan *peer.AddrInfo, numJobs)
	idResults := make(chan idResult, numJobs)

	for w := 0; w < 5; w++ {
		go t.idWorker(ctx, idJobs, idResults)
	}

	for _, providerPeer := range providerPeers {
		idJobs <- providerPeer
	}
	close(idJobs)

	for i := 0; i < numJobs; i++ {
		idResult := <-idResults

		provider := &provider{
			website: website,
			path:    nrr.Path,
			id:      idResult.peer.ID,
			addrs:   idResult.peer.Addrs,
		}

		if idResult.err != nil {
			provider.err = idResult.err
		} else {
			provider.agent = idResult.idOut.AgentVersion
			if len(idResult.idOut.Addresses) != len(idResult.peer.Addrs) && len(idResult.idOut.Addresses) != 0 {
				newAddrs := make([]multiaddr.Multiaddr, len(idResult.idOut.Addresses))
				for i, addr := range idResult.idOut.Addresses {
					newAddrs[i] = multiaddr.StringCast(addr)
				}
				provider.addrs = newAddrs
			}
		}

		results <- provider
	}

	return nil
}

type idResult struct {
	peer  *peer.AddrInfo
	idOut shell.IdOutput
	err   error
}

func (t *Tiros) idWorker(ctx context.Context, jobs <-chan *peer.AddrInfo, idResults chan<- idResult) {
	for j := range jobs {
		var out shell.IdOutput

		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := t.Kubo.Request("id", j.ID.String()).Exec(tCtx, &out)
		cancel()

		idResults <- idResult{
			peer:  j,
			idOut: out,
			err:   err,
		}
	}
}
