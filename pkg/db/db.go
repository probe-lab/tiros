package db

import (
	"context"
	"embed"
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

//go:embed migrations
var migrations embed.FS

type Client interface {
	io.Closer
	Websites(ctx context.Context) ([]string, error)
	Gateways(ctx context.Context) ([]string, error)
	InsertUpload(ctx context.Context, upload *UploadModel) error
	InsertDownload(ctx context.Context, download *DownloadModel) error
	InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error
	InsertProvider(ctx context.Context, provider *ProviderModel) error
	InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error
	InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error
}

type ClickhouseClient struct {
	Conn driver.Conn

	biGroup         *pldb.BatchInserterGroup
	biUploads       *pldb.BatchInserter[UploadModel]
	biDownloads     *pldb.BatchInserter[DownloadModel]
	biWebsiteProbes *pldb.BatchInserter[WebsiteProbeModel]
	biProviders     *pldb.BatchInserter[ProviderModel]
	biGatewayProbes *pldb.BatchInserter[GatewayProbeModel]
	biSWProbes      *pldb.BatchInserter[ServiceWorkerProbeModel]
}

var _ Client = (*ClickhouseClient)(nil)

func NewClickhouseClient(
	ctx context.Context,
	chConfig *pldb.ClickHouseConfig,
	migrationsConfig *pldb.ClickHouseMigrationsConfig,
) (*ClickhouseClient, error) {
	// initializing the clickhouse db client

	conn, err := chConfig.OpenAndPing(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to clickhouse: %w", err)
	}

	if err = migrationsConfig.Apply(chConfig.Options(), migrations); err != nil {
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	biUploads, err := newBatchInserter[UploadModel](conn, "uploads")
	if err != nil {
		return nil, fmt.Errorf("creating uploads batch inserter: %w", err)
	}

	biDownloads, err := newBatchInserter[DownloadModel](conn, "downloads")
	if err != nil {
		return nil, fmt.Errorf("creating downloads batch inserter: %w", err)
	}

	biWebsiteProbes, err := newBatchInserter[WebsiteProbeModel](conn, "website_probes")
	if err != nil {
		return nil, fmt.Errorf("creating website_probes batch inserter: %w", err)
	}

	biProviders, err := newBatchInserter[ProviderModel](conn, "providers")
	if err != nil {
		return nil, fmt.Errorf("creating providers batch inserter: %w", err)
	}

	biGatewayProbes, err := newBatchInserter[GatewayProbeModel](conn, "gateway_probes")
	if err != nil {
		return nil, fmt.Errorf("creating gateway_probes batch inserter: %w", err)
	}

	biSWProbes, err := newBatchInserter[ServiceWorkerProbeModel](conn, "service_worker_probes")
	if err != nil {
		return nil, fmt.Errorf("creating service_worker_probes batch inserter: %w", err)
	}

	biGroup := &pldb.BatchInserterGroup{}
	biGroup.Add(biUploads)
	biGroup.Add(biDownloads)
	biGroup.Add(biWebsiteProbes)
	biGroup.Add(biProviders)
	biGroup.Add(biGatewayProbes)
	biGroup.Add(biSWProbes)
	biGroup.Start(context.Background())

	client := &ClickhouseClient{
		Conn: conn,

		biGroup:         biGroup,
		biUploads:       biUploads,
		biDownloads:     biDownloads,
		biWebsiteProbes: biWebsiteProbes,
		biProviders:     biProviders,
		biGatewayProbes: biGatewayProbes,
		biSWProbes:      biSWProbes,
	}

	return client, nil
}

func newBatchInserter[T any](
	conn driver.Conn,
	table string,
) (*pldb.BatchInserter[T], error) {
	cfg := pldb.DefaultBatchInserterConfig[T]()
	cfg.FlushInterval = 10 * time.Minute
	cfg.OnDroppedRows = func(rows []T, err error) {
		slog.Warn("Dropped rows in batch inserter", "table", table, "count", len(rows), "err", err)
	}

	bi, err := pldb.NewBatchInserter[T](conn, table, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating %s batch inserter: %w", table, err)
	}

	return bi, nil
}

func (c *ClickhouseClient) Close() error {
	timeout := 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := c.biGroup.Stop(ctx); err != nil {
		slog.Warn("Failed stopping batch inserter group in time", "err", err, "timeout", timeout)
	}

	return c.Conn.Close()
}

func (c *ClickhouseClient) Websites(ctx context.Context) ([]string, error) {
	rows, err := c.Conn.Query(ctx, `SELECT * FROM websites WHERE deactivated_at IS NULL`)
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

func (c *ClickhouseClient) Gateways(ctx context.Context) ([]string, error) {
	rows, err := c.Conn.Query(ctx, `SELECT domain FROM gateways WHERE deactivated_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer pllog.Defer(rows.Close, "Failed closing rows")

	var gateways []string
	for rows.Next() {
		var gateway string
		if err := rows.Scan(&gateway); err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

func (c *ClickhouseClient) InsertUpload(ctx context.Context, upload *UploadModel) error {
	return c.biUploads.Submit(ctx, *upload)
}

func (c *ClickhouseClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	return c.biDownloads.Submit(ctx, *download)
}

func (c *ClickhouseClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	return c.biWebsiteProbes.Submit(ctx, *websiteProbe)
}

func (c *ClickhouseClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	return c.biProviders.Submit(ctx, *provider)
}

func (c *ClickhouseClient) InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error {
	return c.biGatewayProbes.Submit(ctx, *gatewayProbe)
}

func (c *ClickhouseClient) InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error {
	return c.biSWProbes.Submit(ctx, *serviceWorkerProbe)
}

type NoopClient struct{}

var _ Client = (*NoopClient)(nil)

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

func (c *NoopClient) Gateways(ctx context.Context) ([]string, error) {
	return []string{"ipfs.io"}, nil
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

func (c *NoopClient) InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error {
	return nil
}

func (c *NoopClient) InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error {
	return nil
}

type LogClient struct{}

var _ Client = (*LogClient)(nil)

func (c *LogClient) Close() error {
	return nil
}

func (c *LogClient) Websites(ctx context.Context) ([]string, error) {
	panic("not implemented")
}

func (c *LogClient) Gateways(ctx context.Context) ([]string, error) {
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

func (c *LogClient) InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error {
	panic("implement me")
}

func (c *LogClient) InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error {
	panic("implement me")
}

type JSONClient struct {
	uploadsFile             *os.File
	downloadsFile           *os.File
	websiteProbesFile       *os.File
	providersFile           *os.File
	gatewayProbesFile       *os.File
	serviceWorkerProbesFile *os.File
}

var _ Client = (*JSONClient)(nil)

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

	gatewayProbesFile, err := os.Create(path.Join(dir, "gateway_probes.ndjson"))
	if err != nil {
		return nil, err
	}

	serviceWorkerProbesFile, err := os.Create(path.Join(dir, "service_worker_probes.ndjson"))
	if err != nil {
		return nil, err
	}

	slog.Info("Writing uploads to " + uploadsFile.Name())
	return &JSONClient{
		uploadsFile:             uploadsFile,
		downloadsFile:           downloadsFile,
		websiteProbesFile:       websiteProbesFile,
		providersFile:           providersFile,
		gatewayProbesFile:       gatewayProbesFile,
		serviceWorkerProbesFile: serviceWorkerProbesFile,
	}, nil
}

func (c *JSONClient) Close() error {
	errg := errgroup.Group{}
	errg.Go(c.downloadsFile.Close)
	errg.Go(c.uploadsFile.Close)
	errg.Go(c.websiteProbesFile.Close)
	errg.Go(c.providersFile.Close)
	errg.Go(c.gatewayProbesFile.Close)
	errg.Go(c.serviceWorkerProbesFile.Close)
	return errg.Wait()
}

func (c *JSONClient) Websites(ctx context.Context) ([]string, error) {
	return []string{"protocol.ai"}, nil
}

func (c *JSONClient) Gateways(ctx context.Context) ([]string, error) {
	return []string{"ipfs.io"}, nil
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

func (c *JSONClient) InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error {
	enc := json.NewEncoder(c.gatewayProbesFile)
	return enc.Encode(gatewayProbe)
}

func (c *JSONClient) InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error {
	enc := json.NewEncoder(c.serviceWorkerProbesFile)
	return enc.Encode(serviceWorkerProbe)
}
