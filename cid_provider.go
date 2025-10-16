package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	pllog "github.com/probe-lab/go-commons/log"
)

type CIDProvider interface {
	SelectCID(ctx context.Context, origin string) (cid.Cid, error)
}

type StaticCIDProvider struct {
	testCIDs   []cid.Cid
	testCIDIdx int
}

var _ CIDProvider = (*StaticCIDProvider)(nil)

func NewStaticCIDProvider(cids []string) (*StaticCIDProvider, error) {
	testCIDs := make([]cid.Cid, 0, len(cids))
	for _, c := range cids {
		parse, err := cid.Parse(c)
		if err != nil {
			return nil, fmt.Errorf("parsing cid: %w", err)
		}
		testCIDs = append(testCIDs, parse)
	}

	return &StaticCIDProvider{testCIDs: testCIDs}, nil
}

func (p *StaticCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	testCID := p.testCIDs[p.testCIDIdx]
	p.testCIDIdx += 1
	p.testCIDIdx %= len(p.testCIDs)
	return testCID, nil
}

type BitswapSnifferClickhouseCIDProvider struct {
	conn driver.Conn
}

var _ CIDProvider = (*BitswapSnifferClickhouseCIDProvider)(nil)

func NewBitswapSnifferClickhouseCIDProvider(dbClient DBClient) (*BitswapSnifferClickhouseCIDProvider, error) {
	chClient, ok := dbClient.(*ClickhouseClient)
	if !ok {
		return nil, fmt.Errorf("expected clickhouse client, got: %T", dbClient)
	}

	return &BitswapSnifferClickhouseCIDProvider{conn: chClient.conn}, nil
}

func (p *BitswapSnifferClickhouseCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	msgType := "%"
	if origin == "dht" {
		msgType = "add-provider-records"
	}
	rows, err := p.conn.Query(ctx, `SELECT cid FROM bitswap_sniffer_ipfs.shared_cids WHERE origin = $1 AND msg_type like $2 ORDER BY timestamp DESC LIMIT 1`, origin, msgType)
	if err != nil {
		return cid.Cid{}, err
	}
	defer pllog.Defer(rows.Close, "Failed closing rows")

	if !rows.Next() {
		return cid.Cid{}, sql.ErrNoRows
	}

	var cidStr string
	if err := rows.Scan(&cidStr); err != nil {
		return cid.Cid{}, err
	}

	c, err := cid.Parse(cidStr)
	if err != nil {
		return cid.Cid{}, err
	}

	// tmp fix until https://github.com/probe-lab/bitswap-sniffer/pull/11 is merged
	if c.Prefix().Codec != uint64(multicodec.DagPb) {
		return cid.NewCidV1(uint64(multicodec.Raw), c.Hash()), nil
	}

	return c, nil
}
