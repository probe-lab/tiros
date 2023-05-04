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
	manet "github.com/multiformats/go-multiaddr/net"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/null/v8"
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
		err := t.findProviders(c.Context, website, results)
		if err != nil {
			log.WithError(err).WithField("website", website).Warnln("Couldn't find providers")
		}
	}
}

func (t *tiros) findProviders(ctx context.Context, website string, results chan<- *provider) error {
	logEntry := log.WithField("website", website)
	logEntry.Infoln("Finding providers for", website)

	resp, err := t.kubo.Request("name/resolve").
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

	resp, err = t.kubo.
		Request("routing/findprovs").
		Option("arg", nrr.Path).
		Option("num-providers", "1000").
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
		prov.country, prov.continent, prov.asn, prov.datacenterID = t.addrInfos(ctx, prov.addrs)

		results <- prov
	}

	return nil
}

type idResult struct {
	peer  *peer.AddrInfo
	idOut shell.IdOutput
	err   error
}

func (t *tiros) idWorker(ctx context.Context, jobs <-chan *peer.AddrInfo, idResults chan<- idResult) {
	for j := range jobs {
		var out shell.IdOutput

		tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := t.kubo.Request("id", j.ID.String()).Exec(tCtx, &out)
		cancel()

		idResults <- idResult{
			peer:  j,
			idOut: out,
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

func (t *tiros) addrInfos(ctx context.Context, maddrs []multiaddr.Multiaddr) (null.String, null.String, null.Int, null.Int) {
	var countries []string
	var continents []string
	var asns []int
	var datacenters []int

	countriesMap := map[string]struct{}{}
	continentsMap := map[string]struct{}{}
	asnsMap := map[int]struct{}{}
	datacentersMap := map[int]struct{}{}

	for _, maddr := range maddrs {
		infos, err := t.mmClient.MaddrInfo(ctx, maddr)
		if err != nil {
			continue
		}

		for addr, info := range infos {
			if _, found := countriesMap[info.Country]; !found {
				countriesMap[info.Country] = struct{}{}
				countries = append(countries, info.Country)
			}

			if _, found := continentsMap[info.Continent]; !found {
				continentsMap[info.Continent] = struct{}{}
				continents = append(continents, info.Continent)
			}

			if _, found := asnsMap[int(info.ASN)]; !found {
				asnsMap[int(info.ASN)] = struct{}{}
				asns = append(asns, int(info.ASN))
			}

			if t.uClient != nil {
				datacenter, err := t.uClient.Datacenter(addr)
				if err != nil {
					continue
				}

				if _, found := datacentersMap[datacenter]; !found {
					datacentersMap[datacenter] = struct{}{}
					datacenters = append(datacenters, datacenter)
				}
			}
		}
	}

	nullCountry := null.NewString("", false)
	if len(countries) == 1 {
		nullCountry = null.NewString(countries[0], true)
	}

	nullContinent := null.NewString("", false)
	if len(continents) == 1 {
		nullContinent = null.NewString(continents[0], true)
	}

	nullASN := null.NewInt(0, false)
	if len(asns) == 1 {
		nullASN = null.NewInt(asns[0], true)
	}

	nullDatacenters := null.NewInt(0, false)
	if len(datacenters) == 1 {
		nullDatacenters = null.NewInt(datacenters[0], true)
	}

	return nullCountry, nullContinent, nullASN, nullDatacenters
}
