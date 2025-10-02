package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

type provider struct {
	website      string
	path         string
	id           peer.ID
	addrs        []multiaddr.Multiaddr
	agent        string
	err          error
	isRelayed    null.Bool
	country      null.String
	continent    null.String
	asn          null.Int
	datacenterID null.Int
}

type nameResolveResponse struct {
	Path string
}

func (t *tiros) findAllProviders(c *cli.Context, websites []string, results chan<- *provider) {
	defer close(results)
	for _, website := range websites {
		for retry := 0; retry < 3; retry++ {
			err := t.findProviders(c.Context, website, results)
			if err != nil {
				log.WithError(err).WithField("retry", retry).WithField("website", website).Warnln("Couldn't find providers")
				if strings.Contains(err.Error(), "routing/findprovs") {
					continue
				}
			}
			break
		}
	}
}

func (t *tiros) findProviders(ctx context.Context, website string, results chan<- *provider) error {
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	nameResp, err := t.ipfs.Request("name/resolve").
		Option("arg", website).
		Option("nocache", "true").
		Option("dht-timeout", "30s").Send(ctx)
	if err != nil {
		return fmt.Errorf("name/resolve: %w", err)
	} else if nameResp.Error != nil {
		return fmt.Errorf("name/resolve: %w", nameResp.Error)
	} else if nameResp == nil {
		return fmt.Errorf("name/resolve no error but response nil")
	} else if nameResp.Output == nil {
		return fmt.Errorf("name/resolve no error but response output nil")
	}

	defer func() {
		if err = nameResp.Close(); err != nil {
			log.WithError(err).Warnln("Error closing name/resolve response")
		}
	}()

	dat, err := io.ReadAll(nameResp.Output)
	if err != nil {
		return fmt.Errorf("read name/resolve bytes: %w", err)
	}

	nrr := nameResolveResponse{}
	err = json.Unmarshal(dat, &nrr)
	if err != nil {
		return fmt.Errorf("unmarshal name/resolve response: %w", err)
	}

	findResp, err := t.ipfs.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "1000").
		Send(ctx)
	if err != nil {
		return fmt.Errorf("routing/findprovs: %w", err)
	} else if findResp.Error != nil {
		return fmt.Errorf("routing/findprovs: %w", findResp.Error)
	} else if findResp == nil {
		return fmt.Errorf("routing/findprovs no error but response nil")
	} else if findResp.Output == nil {
		return fmt.Errorf("routing/findprovs no error but response output nil")
	}
	defer func() {
		if err = findResp.Close(); err != nil {
			log.WithError(err).Warnln("Error closing name/resolve response")
		}
	}()

	var providerPeers []*peer.AddrInfo
	dec := json.NewDecoder(findResp.Output)
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

	for w := 0; w < 10; w++ {
		go t.idWorker(ctx, idJobs, idResults)
	}

	for _, providerPeer := range providerPeers {
		idJobs <- providerPeer
	}
	close(idJobs)

	for i := 0; i < numJobs; i++ {
		idr := <-idResults

		prov := &provider{
			website: website,
			path:    nrr.Path,
			id:      idr.peer.ID,
			addrs:   idr.peer.Addrs,
		}

		if idr.err != nil {
			prov.err = idr.err
		} else {
			prov.agent = idr.idOut.AgentVersion
			if len(idr.idOut.Addresses) != len(idr.peer.Addrs) && len(idr.idOut.Addresses) != 0 {
				newAddrs := make([]multiaddr.Multiaddr, len(idr.idOut.Addresses))
				for j, addr := range idr.idOut.Addresses {
					newAddrs[j] = multiaddr.StringCast(addr)
				}
				prov.addrs = newAddrs
			}
		}

		prov.isRelayed = isRelayed(prov.addrs)

		results <- prov
	}

	return nil
}

type idResult struct {
	peer  *peer.AddrInfo
	idOut *IdOutput
	err   error
}

type IdOutput struct { // nolint
	ID           string
	PublicKey    string
	Addresses    []string
	AgentVersion string
	Protocols    []protocol.ID
}

func (t *tiros) idWorker(ctx context.Context, jobs <-chan *peer.AddrInfo, idResults chan<- idResult) {
	for j := range jobs {
		var out IdOutput
		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := t.ipfs.Request("id", j.ID.String()).Exec(tCtx, &out)
		cancel()

		idResults <- idResult{
			peer:  j,
			idOut: &out,
			err:   err,
		}
	}
}

func isRelayed(maddrs []multiaddr.Multiaddr) null.Bool {
	if len(maddrs) == 0 {
		return null.NewBool(false, false)
	}
	for _, maddr := range maddrs {
		if manet.IsPrivateAddr(maddr) {
			continue
		}

		if _, err := maddr.ValueForProtocol(multiaddr.P_CIRCUIT); err != nil {
			return null.NewBool(false, true)
		}
	}
	return null.NewBool(true, true)
}
