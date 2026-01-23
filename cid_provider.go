package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ipfs/go-cid"
	kuboclient "github.com/ipfs/kubo/client/rpc"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/multiformats/go-multicodec"
	pllog "github.com/probe-lab/go-commons/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	conn             driver.Conn
	cidSelectCounter metric.Int64Counter
}

var _ CIDProvider = (*BitswapSnifferClickhouseCIDProvider)(nil)

func NewBitswapSnifferClickhouseCIDProvider(dbClient DBClient) (*BitswapSnifferClickhouseCIDProvider, error) {
	chClient, ok := dbClient.(*ClickhouseClient)
	if !ok {
		return nil, fmt.Errorf("expected clickhouse client, got: %T", dbClient)
	}

	meter := otel.GetMeterProvider().Meter("tiros")
	cidSelectCounter, err := meter.Int64Counter("cid_select")
	if err != nil {
		return nil, fmt.Errorf("creating cid select counter: %w", err)
	}

	return &BitswapSnifferClickhouseCIDProvider{
		conn:             chClient.conn,
		cidSelectCounter: cidSelectCounter,
	}, nil
}

func (p *BitswapSnifferClickhouseCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	msgType := "%"
	limit := 100

	if origin == "dht" {
		msgType = "add-provider-records"
		limit = 1
	}

	rows, err := p.conn.Query(ctx, `
		WITH cte AS (
			SELECT
				cid
			FROM bitswap_sniffer_ipfs.shared_cids
			WHERE origin = $1
			  AND msg_type LIKE $2
			ORDER BY timestamp DESC
			LIMIT $3
		) SELECT cid FROM cte ORDER BY RAND() LIMIT 1
	`, origin, msgType, limit)

	p.cidSelectCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("origin", origin),
		attribute.Bool("success", err == nil && rows.Err() == nil),
	))

	var c cid.Cid
	if err != nil {
		return c, err
	}
	defer pllog.Defer(rows.Close, "Failed closing rows")

	if rows.Next() {
		var cidStr string
		if err := rows.Scan(&cidStr); err != nil {
			return c, err
		}

		c, err = cid.Parse(cidStr)
		if err != nil {
			return c, err
		}

		// tmp fix until https://github.com/probe-lab/bitswap-sniffer/pull/11 is merged
		if c.Prefix().Codec != uint64(multicodec.DagPb) {
			c = cid.NewCidV1(uint64(multicodec.Raw), c.Hash())
		}
	}

	return c, rows.Err()
}

type KuboCIDProvider struct {
	client *kuboclient.HttpApi

	cidsMu       sync.Mutex
	cids         []cid.Cid
	cidsLoadedAt time.Time
}

var _ CIDProvider = (*KuboCIDProvider)(nil)

func NewKuboCIDProvider(host string, port int) (*KuboCIDProvider, error) {
	kuboAddr := net.JoinHostPort(host, strconv.Itoa(port))
	c, err := kuboclient.NewURLApiWithClient(kuboAddr, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("init kubo client: %w", err)
	}
	return &KuboCIDProvider{client: c, cids: []cid.Cid{}}, nil
}

func (p *KuboCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	p.cidsMu.Lock()
	defer p.cidsMu.Unlock()

	// refresh every 12 hours
	if len(p.cids) > 0 && time.Since(p.cidsLoadedAt) < 12*time.Hour {
		idx := rand.IntN(len(p.cids))
		return p.cids[idx], nil
	}

	pins := make(chan iface.Pin)
	err := p.client.Pin().Ls(ctx, pins, options.Pin.Ls.Recursive())
	if err != nil {
		return cid.Undef, err
	}

	for pin := range pins {
		p.cids = append(p.cids, pin.Path().RootCid())
	}

	p.cidsLoadedAt = time.Now()
	idx := rand.IntN(len(p.cids))
	return p.cids[idx], nil
}

//go:embed controlledcids
var controlledCIDs string

type ControlledCIDProvider struct {
	cids []cid.Cid
	idx  atomic.Int32
}

var _ CIDProvider = (*ControlledCIDProvider)(nil)

func NewControlledCIDProvider() (*ControlledCIDProvider, error) {
	var cids []cid.Cid
	for _, line := range strings.Split(controlledCIDs, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		c, err := cid.Parse(strings.TrimSpace(line))
		if err != nil {
			slog.Warn("failed to parse CID from controlledcids file: %v", err)
			continue
		}
		cids = append(cids, c)
	}

	// shuffle once and then iterate in order with every call to SelectCID
	rand.Shuffle(len(cids), func(i, j int) { cids[i], cids[j] = cids[j], cids[i] })

	return &ControlledCIDProvider{cids: cids}, nil
}

func (p *ControlledCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	idx := p.idx.Add(1)
	idx %= int32(len(p.cids))
	return p.cids[idx], nil
}

type NoopCIDProvider struct{}

var _ CIDProvider = (*NoopCIDProvider)(nil)

func (p *NoopCIDProvider) SelectCID(ctx context.Context, origin string) (cid.Cid, error) {
	return cid.Undef, nil
}
