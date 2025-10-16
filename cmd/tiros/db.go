package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ipfs/go-cid"
	pldb "github.com/probe-lab/go-commons/db"
	pllog "github.com/probe-lab/go-commons/log"
	"golang.org/x/sync/errgroup"
)

type UploadModel struct {
	Region            string    `ch:"region"`
	TirosVersion      string    `ch:"tiros_version"`
	KuboVersion       string    `ch:"kubo_version"`
	KuboPeerID        string    `ch:"kubo_peer_id"`
	FileSizeB         int32     `ch:"file_size_b"`
	CID               string    `ch:"cid"`
	IPFSAddStart      time.Time `ch:"ipfs_add_start"`
	IPFSAddDurationMs int32     `ch:"ipfs_add_duration_ms"`
	ProvideStart      time.Time `ch:"provide_start"`
	ProvideDurationMs int32     `ch:"provide_duration_ms"`
	ProvideDelayMs    int32     `ch:"provide_delay_ms"`
	UploadDurationMs  int32     `ch:"upload_duration_ms"`
	Error             string    `ch:"error"`
}

type DownloadModel struct {
	Region               string    `ch:"region"`
	TirosVersion         string    `ch:"tiros_version"`
	KuboVersion          string    `ch:"kubo_version"`
	KuboPeerID           string    `ch:"kubo_peer_id"`
	FileSizeB            int32     `ch:"file_size_b"`
	CID                  string    `ch:"cid"`
	IPFSCatStart         time.Time `ch:"ipfs_cat_start"`
	IPFSCatTTFBMs        int32     `ch:"ipfs_cat_ttfb_ms"`
	IPFSCatDurationMs    int32     `ch:"ipfs_cat_duration_ms"`
	IdleBroadcastStart   time.Time `ch:"idle_broadcast_start"`
	FoundProvCount       int       `ch:"found_prov_count"`
	ConnProvCount        int       `ch:"conn_prov_count"`
	FirstConnProvFoundAt time.Time `ch:"first_conn_prov_found_at"`
	FirstProvConnAt      time.Time `ch:"first_prov_conn_at"`
	FirstProvPeerID      string    `ch:"first_prov_peer_id"`
	IPNIStart            time.Time `ch:"ipni_start"`
	IPNIDurationMs       int32     `ch:"ipni_duration_ms"`
	IPNIStatus           int       `ch:"ipni_status"`
	FirstBlockReceivedAt time.Time `ch:"first_block_rec_at"`
	DiscoveryMethod      string    `ch:"discovery_method"`
	CIDSource            string    `ch:"cid_source"`
	Error                string    `ch:"error"`
}

type DBClient interface {
	io.Closer
	InsertUpload(ctx context.Context, upload *UploadModel) error
	InsertDownload(ctx context.Context, download *DownloadModel) error
}

var defaultCid = cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")

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

func (c *ClickhouseClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO downloads")
	if err != nil {
		return fmt.Errorf("preparer batch: %w", err)
	}
	defer pllog.Defer(b.Close, "Failed closing batch")

	if err := b.AppendStruct(download); err != nil {
		return fmt.Errorf("append struct: %w", err)
	}

	return b.Send()
}

type NoopClient struct{}

var _ DBClient = (*NoopClient)(nil)

func NewNoopClient() *NoopClient {
	slog.Info("Skipping database interactions.")
	return &NoopClient{}
}

func (c *NoopClient) Close() error {
	return nil
}

func (c *NoopClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	return nil
}

func (c *NoopClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	return nil
}

func (c *NoopClient) SelectCID(ctx context.Context) (cid.Cid, error) {
	return cid.MustParse(""), nil
}

type LogClient struct{}

var _ DBClient = (*LogClient)(nil)

func (c *LogClient) Close() error {
	return nil
}

func (c *LogClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	panic("not implemented")
}

func (c *LogClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	panic("not implemented")
}

type JSONClient struct {
	uploadsFile   *os.File
	downloadsFile *os.File
}

var _ DBClient = (*JSONClient)(nil)

func NewJSONClient(dir string) (*JSONClient, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	prefix := time.Now().Format("2006-01-02T1504")
	uploadsFile, err := os.Create(path.Join(dir, prefix+"_uploads.ndjson"))
	if err != nil {
		return nil, err
	}

	downloadsFile, err := os.Create(path.Join(dir, prefix+"_downloads.ndjson"))
	if err != nil {
		return nil, err
	}

	slog.Info("Writing uploads to " + uploadsFile.Name())
	return &JSONClient{
		uploadsFile:   uploadsFile,
		downloadsFile: downloadsFile,
	}, nil
}

func (c *JSONClient) Close() error {
	errg := errgroup.Group{}
	errg.Go(c.downloadsFile.Close)
	errg.Go(c.uploadsFile.Close)
	return errg.Wait()
}

func (c *JSONClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	enc := json.NewEncoder(c.uploadsFile)
	return enc.Encode(upload)
}

func (c *JSONClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	enc := json.NewEncoder(c.downloadsFile)
	return enc.Encode(download)
}
