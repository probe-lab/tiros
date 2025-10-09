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
	pldb "github.com/probe-lab/go-commons/db"
	pllog "github.com/probe-lab/go-commons/log"
	"golang.org/x/sync/errgroup"
)

type DBClient interface {
	io.Closer
	Websites(ctx context.Context) ([]string, error)
	InsertUpload(ctx context.Context, upload *UploadModel) error
	InsertDownload(ctx context.Context, download *DownloadModel) error
	InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error
	InsertProvider(ctx context.Context, provider *ProviderModel) error
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

func (c *ClickhouseClient) Websites(ctx context.Context) ([]string, error) {
	rows, err := c.conn.Query(ctx, `SELECT * FROM websites WHERE deactivated_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer pllog.Defer(rows.Close, "Failed closing rows")

	var websites []string
	for rows.Next() {
		var website string
		if err := rows.Scan(&website); err != nil {
			return nil, err
		}
		websites = append(websites, website)
	}

	return websites, nil
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

func (c *ClickhouseClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO website_probes")
	if err != nil {
		return fmt.Errorf("preparer batch: %w", err)
	}
	defer pllog.Defer(b.Close, "Failed closing batch")

	if err := b.AppendStruct(websiteProbe); err != nil {
		return fmt.Errorf("append struct: %w", err)
	}
	return b.Send()
}

func (c *ClickhouseClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO providers")
	if err != nil {
		return fmt.Errorf("preparer batch: %w", err)
	}
	defer pllog.Defer(b.Close, "Failed closing batch")

	if err := b.AppendStruct(provider); err != nil {
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

func (c *NoopClient) Websites(ctx context.Context) ([]string, error) {
	return []string{"protocol.ai"}, nil
}

func (c *NoopClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	return nil
}

func (c *NoopClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	return nil
}

func (c *NoopClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	return nil
}

func (c *NoopClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	return nil
}

type LogClient struct{}

var _ DBClient = (*LogClient)(nil)

func (c *LogClient) Close() error {
	return nil
}

func (c *LogClient) Websites(ctx context.Context) ([]string, error) {
	panic("not implemented")
}

func (c *LogClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	panic("not implemented")
}

func (c *LogClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	panic("not implemented")
}

func (c *LogClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	panic("implement me")
}

func (c *LogClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	panic("implement me")
}

type JSONClient struct {
	uploadsFile       *os.File
	downloadsFile     *os.File
	websiteProbesFile *os.File
	providersFile     *os.File
}

var _ DBClient = (*JSONClient)(nil)

func NewJSONClient(dir string) (*JSONClient, error) {
	dir = path.Join(dir, time.Now().Format("2006-01-02T15-04"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	uploadsFile, err := os.Create(path.Join(dir, "uploads.ndjson"))
	if err != nil {
		return nil, err
	}

	downloadsFile, err := os.Create(path.Join(dir, "downloads.ndjson"))
	if err != nil {
		return nil, err
	}

	websiteProbesFile, err := os.Create(path.Join(dir, "website_probes.ndjson"))
	if err != nil {
		return nil, err
	}

	providersFile, err := os.Create(path.Join(dir, "providers.ndjson"))
	if err != nil {
		return nil, err
	}

	slog.Info("Writing uploads to " + uploadsFile.Name())
	return &JSONClient{
		uploadsFile:       uploadsFile,
		downloadsFile:     downloadsFile,
		websiteProbesFile: websiteProbesFile,
		providersFile:     providersFile,
	}, nil
}

func (c *JSONClient) Close() error {
	errg := errgroup.Group{}
	errg.Go(c.downloadsFile.Close)
	errg.Go(c.uploadsFile.Close)
	return errg.Wait()
}

func (c *JSONClient) Websites(ctx context.Context) ([]string, error) {
	return []string{"protocol.ai"}, nil
}

func (c *JSONClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	enc := json.NewEncoder(c.uploadsFile)
	return enc.Encode(upload)
}

func (c *JSONClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	enc := json.NewEncoder(c.downloadsFile)
	return enc.Encode(download)
}

func (c *JSONClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	enc := json.NewEncoder(c.websiteProbesFile)
	return enc.Encode(websiteProbe)
}

func (c *JSONClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	enc := json.NewEncoder(c.providersFile)
	return enc.Encode(provider)
}
