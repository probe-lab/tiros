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

	"github.com/dennis-tra/tiros/models"

	"github.com/urfave/cli/v2"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

//go:embed migrations
var migrations embed.FS

type DBClient struct {
	// Database handle
	handle *sql.DB
}

// InitClient establishes a database connection with
// the provided configuration and applies any pending migrations
func InitClient(ctx context.Context, host string, port int, name string, user string, password string, ssl string) (*DBClient, error) {
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

func (db *DBClient) InsertMeasurement(ctx context.Context, m *models.Measurement) (*models.Measurement, error) {
	return m, m.Insert(ctx, db.handle, boil.Infer())
}
