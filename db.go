package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"

	"github.com/dennis-tra/tiros/models"
)

//go:embed migrations
var migrations embed.FS

type IDBClient interface {
	InsertRun(c *cli.Context, version string) (*models.Run, error)
	SaveMeasurement(c *cli.Context, dbRun *models.Run, pr *probeResult) (*models.Measurement, error)
	SaveProvider(c *cli.Context, dbRun *models.Run, provider *provider) (*models.Provider, error)
	SealRun(ctx context.Context, dbRun *models.Run) (*models.Run, error)
}

type DBClient struct {
	// Database handle
	handle *sql.DB
}

var _ IDBClient = (*DBClient)(nil)

// InitClient establishes a database connection with
// the provided configuration and applies any pending migrations
func InitClient(ctx context.Context, host string, port int, name string, user string, password string, ssl string) (IDBClient, error) {
	log.WithFields(log.Fields{
		"host": host,
		"port": port,
		"name": name,
		"user": user,
		"ssl":  ssl,
	}).Infoln("Initializing database client")

	driverName, err := ocsql.Register("postgres")
	if err != nil {
		return nil, fmt.Errorf("register ocsql: %w", err)
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		host, port, name, user, password, ssl,
	)

	// Open database handle
	db, err := sql.Open(driverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Ping database to verify connection.
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	client := &DBClient{handle: db}

	return client, client.applyMigrations(name)
}

func (db *DBClient) Close() error {
	return db.handle.Close()
}

func (db *DBClient) applyMigrations(name string) error {
	tmpDir, err := os.MkdirTemp("", "tiros")
	if err != nil {
		return fmt.Errorf("create migrations tmp dir: %w", err)
	}
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			log.WithField("tmpDir", tmpDir).WithError(err).Warnln("Could not clean up tmp directory")
		}
	}()
	log.WithField("dir", tmpDir).Debugln("Created temporary directory")

	err = fs.WalkDir(migrations, ".", func(path string, d fs.DirEntry, err error) error {
		join := filepath.Join(tmpDir, path)
		if d.IsDir() {
			return os.MkdirAll(join, 0o755)
		}

		data, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		return os.WriteFile(join, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("create migrations files: %w", err)
	}

	// Apply migrations
	driver, err := postgres.WithInstance(db.handle, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create driver instance: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(tmpDir, "migrations"), name, driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func (db *DBClient) SealRun(ctx context.Context, dbRun *models.Run) (*models.Run, error) {
	dbRun.FinishedAt = null.TimeFrom(time.Now())
	_, err := dbRun.Update(ctx, db.handle, boil.Infer())
	return dbRun, err
}

func (db *DBClient) InsertRun(c *cli.Context, version string) (*models.Run, error) {
	log.Infoln("Inserting Run...")

	websites := make([]string, len(c.StringSlice("websites")))
	for i, website := range c.StringSlice("websites") {
		websites[i] = website
	}
	sort.Strings(websites)

	r := &models.Run{
		Region:   c.String("region"),
		Websites: websites,
		Version:  version,
		Times:    int16(c.Int("times")),
		CPU:      c.Int("cpu"),
		Memory:   c.Int("memory"),
	}

	return r, r.Insert(c.Context, db.handle, boil.Infer())
}

func (db *DBClient) SaveProvider(c *cli.Context, dbRun *models.Run, provider *provider) (*models.Provider, error) {
	log.Infoln("Saving provider for", provider.website)

	maddrs := make([]string, len(provider.addrs))
	for i, maddr := range provider.addrs {
		maddrs[i] = maddr.String()
	}

	errMsg := ""

	if provider.err != nil {
		errMsg = provider.err.Error()
	}

	dbProvider := &models.Provider{
		RunID:          dbRun.ID,
		Website:        provider.website,
		Path:           provider.path,
		PeerID:         provider.id.String(),
		AgentVersion:   null.NewString(provider.agent, provider.agent != ""),
		MultiAddresses: maddrs,
		Error:          null.NewString(errMsg, errMsg != ""),
	}

	if err := dbProvider.Insert(c.Context, db.handle, boil.Infer()); err != nil {
		return nil, fmt.Errorf("insert provider: %w", err)
	}

	return dbProvider, nil
}

func (db *DBClient) SaveMeasurement(c *cli.Context, dbRun *models.Run, pr *probeResult) (*models.Measurement, error) {
	metrics, err := pr.NullJSON()
	if err != nil {
		return nil, fmt.Errorf("getting metrics json")
	}

	m := &models.Measurement{
		RunID:      dbRun.ID,
		Website:    pr.website,
		URL:        pr.url,
		Type:       pr.mType,
		Try:        int16(pr.try),
		TTFB:       intervalMs(pr.ttfb),
		FCP:        intervalMs(pr.fcp),
		LCP:        intervalMs(pr.lcp),
		CLS:        intervalMs(pr.cls),
		Tti:        intervalMs(pr.tti),
		TtiRating:  mapRating(pr.ttiRating),
		TTFBRating: mapRating(pr.ttfbRating),
		FCPRating:  mapRating(pr.fcpRating),
		LCPRating:  mapRating(pr.lcpRating),
		CLSRating:  mapRating(pr.clsRating),
		Metrics:    metrics,
		Error:      pr.NullError(),
	}

	if err := m.Insert(c.Context, db.handle, boil.Infer()); err != nil {
		return nil, fmt.Errorf("insert measurement: %w", err)
	}

	return m, nil
}

func intervalMs(val *float64) null.String {
	if val == nil {
		return null.NewString("", false)
	}
	return null.StringFrom(fmt.Sprintf("%f Milliseconds", *val))
}

func mapRating(rating *string) null.String {
	if rating == nil {
		return null.StringFromPtr(nil)
	}
	switch *rating {
	case "good":
		return null.StringFrom(models.RatingGOOD)
	case "needs-improvement":
		return null.StringFrom(models.RatingNEEDS_IMPROVEMENT)
	case "poor":
		return null.StringFrom(models.RatingPOOR)
	default:
		panic("unknown rating " + *rating)
	}
}

type DBDummyClient struct{}

var _ IDBClient = (*DBDummyClient)(nil)

func (D DBDummyClient) SaveMeasurement(c *cli.Context, dbRun *models.Run, pr *probeResult) (*models.Measurement, error) {
	return nil, nil
}

func (D DBDummyClient) SaveProvider(c *cli.Context, dbRun *models.Run, provider *provider) (*models.Provider, error) {
	return &models.Provider{}, nil
}

func (D DBDummyClient) SealRun(ctx context.Context, dbRun *models.Run) (*models.Run, error) {
	return nil, nil
}

func (D DBDummyClient) InsertRun(c *cli.Context, version string) (*models.Run, error) {
	return &models.Run{
		ID:     2,
		Region: "dummy",
	}, nil
}

func (D DBDummyClient) InsertMeasurement(ctx context.Context, m *models.Measurement) (*models.Measurement, error) {
	return nil, nil
}
