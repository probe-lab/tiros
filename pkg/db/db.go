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
	"reflect"
	"strings"
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
}

var _ Client = (*ClickhouseClient)(nil)

// buildInsertQuery builds an INSERT query and extracts values from a struct using reflection.
// It reads the `ch` struct tags to determine column names.
func buildInsertQuery(tableName string, model any) (query string, values []any) {
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	var columns []string
	var placeholders []string
	values = make([]any, 0, typ.NumField())

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		chTag := field.Tag.Get("ch")
		if chTag == "" || chTag == "-" {
			continue
		}

		columns = append(columns, chTag)
		placeholders = append(placeholders, "?")

		iface := val.Field(i).Interface()
		switch tiface := iface.(type) {
		case json.RawMessage:
			values = append(values, string(tiface))
		default:
			values = append(values, iface)
		}
	}

	query = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	return query, values
}

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
	defer pllog.Defer(conn.Close, "Failed closing clickhouse client")

	if err = migrationsConfig.Apply(chConfig.Options(), migrations); err != nil {
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	return &ClickhouseClient{Conn: conn}, nil
}

func (c *ClickhouseClient) Close() error {
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
	query, values := buildInsertQuery("uploads", upload)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert uploads: %w", err)
	}
	return nil
}

func (c *ClickhouseClient) InsertDownload(ctx context.Context, download *DownloadModel) error {
	query, values := buildInsertQuery("downloads", download)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert downloads: %w", err)
	}
	return nil
}

func (c *ClickhouseClient) InsertWebsiteProbe(ctx context.Context, websiteProbe *WebsiteProbeModel) error {
	query, values := buildInsertQuery("website_probes", websiteProbe)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert website_probes: %w", err)
	}
	return nil
}

func (c *ClickhouseClient) InsertProvider(ctx context.Context, provider *ProviderModel) error {
	query, values := buildInsertQuery("providers", provider)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert providers: %w", err)
	}
	return nil
}

func (c *ClickhouseClient) InsertGatewayProbe(ctx context.Context, gatewayProbe *GatewayProbeModel) error {
	query, values := buildInsertQuery("gateway_probes", gatewayProbe)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert gateway_probes: %w", err)
	}
	return nil
}

func (c *ClickhouseClient) InsertServiceWorkerProbe(ctx context.Context, serviceWorkerProbe *ServiceWorkerProbeModel) error {
	query, values := buildInsertQuery("service_worker_probes", serviceWorkerProbe)
	err := c.Conn.AsyncInsert(ctx, query, false, values...)
	if err != nil {
		return fmt.Errorf("async insert service_worker_probes: %w", err)
	}
	return nil
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
