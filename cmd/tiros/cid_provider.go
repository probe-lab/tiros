package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
)

type CIDProvider interface {
	SelectCID(ctx context.Context, origin string) (cid.Cid, error)
}

type StaticCIDProvider struct {
	testCIDs   []cid.Cid
	testCIDIdx int
}

var _ CIDProvider = (*StaticCIDProvider)(nil)

func NewStaticCIDProvider() *StaticCIDProvider {
	testCIDStrs := []string{
		"bafkreigmes4fo2xnpixfk4syb5m27iok7rusrh6yziod4y5kunfhb6mf5e",
		"QmPrRV2DJHJCneS6Xyjg4y1FkoGidzAbSQxkwjcXi5rpiu",
		"bafkreihaa5hixintqqa2hdkgcoiczlpmv7dl4yammrg5f2ixovwkjc7peu",
		"bafybeiaprerug3p76ozy772iudrr5sqecs7wpxyrtwqzxmlyu7ri3unqae",
		"QmUZipvzKLssPTHxUnDwef3a8cPZGL8BwX7urzmNFNtTJ1",
		"QmfAxJ75ePH87jxh6K364P7ce2EFtz3KnU3xzLMmrv3eMN",
		"bafkreigemrgrxezrzyvt2jq7kj2v5m3aajjdyuemwkhgfuizbuudasrf6e",
		"bafkreie3trdds4kskfyxw3hzyxzjpwogwwbjpuja5l24avqrf4scwlusri",
		"bafybeiawuyyxivuxnaqe6iztn4op555flezzfug3h6zf4j2z3vdbit5vue",
		"bafkreia2gwddcggdprkn5t6wu4j5a3gv77ftgum7mdid53a6phxzpba5f4",
		"bafkreibon5tv5zuu4lt2re6yfhkuo3ojtfbfdn6t5zna4excyewazmofca",
		"bafkreiha5ukwhn6ytl73w4tp3v7h2zayvnpoe64uobhb3f3gf36ng3aa4q",
		"bafkreias4o6xfoitigzponn5zb7oqifj4fysmk6fjtogp5zmrizr7ijeja",
		"bafkreig2ltiutlf35ioab6cceorgbjbfu7pkvfdoduhugrvwl5by5yw2di",
		"bafkreia73jrngvxgmanbyozjmln5f6roqbjzt6yugqzja3s4exqrmwvvkq",
		"bafkreihtf7ckw7kapkwkcp6vh7vsnqxdxii5gi5fodj3k5rxbithfvxfem",
		"bafkreifxturkxlmfanavfv2amr63dowtrhegkqpxqposigda2uono2bvyy",
		"bafkreihnyskm5bpa47xsi2wde3vephyewfupawrqebraqmnm3bhk673caa",
		"bafkreibocixhj5ln3k37anfdwqp4rfxqlf2ffs7em2kokptq2pgvmohr2a",
	}

	testCIDs := make([]cid.Cid, 0, len(testCIDStrs))
	for _, c := range testCIDStrs {
		parse := cid.MustParse(c)
		testCIDs = append(testCIDs, parse)

	}

	return &StaticCIDProvider{testCIDs: testCIDs}
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
	defer rows.Close()

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
