package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	pldb "github.com/probe-lab/go-commons/db"
	pllog "github.com/probe-lab/go-commons/log"
)

type UploadModel struct {
	Region            string    `ch:"region"`
	TirosVersion      string    `ch:"tiros_version"`
	KuboVersion       string    `ch:"kubo_version"`
	FileSizeMiB       int32     `ch:"file_size_mib"`
	IPFSAddStart      time.Time `ch:"ipfs_add_start"`
	IPFSAddDurationMs int32     `ch:"ipfs_add_duration_ms"`
	ProvideStart      time.Time `ch:"provide_start"`
	ProvideDurationMs int32     `ch:"provide_duration_ms"`
	ProvideDelayMs    int32     `ch:"provide_delay_ms"`
	UploadDurationMs  int32     `ch:"upload_duration_ms"`
}

type DBClient interface {
	io.Closer
	InsertUpload(ctx context.Context, upload *UploadModel) error
}

type ClickhouseClient struct {
	conn driver.Conn
}

var _ DBClient = (*ClickhouseClient)(nil)

func NewClickhouseClient(
	ctx context.Context,
	chConfig *pldb.ClickHouseConfig,
	migrationsConfig *pldb.ClickHouseMigrationsConfig,
) (*ClickhouseClient, error) {
	// initializing the clickhouse db client
	chOpts := probeConfig.Clickhouse.Options()
	conn, err := probeConfig.Clickhouse.OpenAndPing(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to clickhouse: %w", err)
	}
	defer pllog.Defer(conn.Close, "Failed closing clickhouse client")

	if err = probeConfig.Migrations.Apply(chOpts, migrations); err != nil {
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	return &ClickhouseClient{conn: conn}, nil
}

func (c *ClickhouseClient) Close() error {
	return c.conn.Close()
}

func (c *ClickhouseClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO uploads")
	if err != nil {
		return fmt.Errorf("preparer batch: %w", err)
	}
	defer pllog.Defer(b.Close, "Failed closing batch")

	if err := b.AppendStruct(upload); err != nil {
		return fmt.Errorf("append struct: %w", err)
	}

	return b.Send()
}

type NoopClient struct{}

var _ DBClient = (*NoopClient)(nil)

func NewNoopClient() *NoopClient {
	return &NoopClient{}
}

func (n NoopClient) Close() error {
	return nil
}

func (n NoopClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	return nil
}

type LogClient struct{}

var _ DBClient = (*LogClient)(nil)

func (n *LogClient) Close() error {
	// TODO implement me
	panic("implement me")
}

func (n *LogClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	panic("not implemented")
}

type JSONClient struct{}

var _ DBClient = (*JSONClient)(nil)

func (n *JSONClient) Close() error {
	// TODO implement me
	panic("implement me")
}

func (n *JSONClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	panic("not implemented")
}
