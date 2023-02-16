package main

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed migrations
var migrations embed.FS

type DBClient struct {
	// Database handle
	handle *sql.DB
}

// InitDB establishes a database connection with the provided configuration
// and applies any pending migrations
func InitDB(host string, port int, name string, user string, password string, ssl string) (*DBClient, error) {
	log.Infow("Initializing database client", "host", host, "port", port, "name", name, "user", user, "ssl", ssl)

	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		host, port, name, user, password, ssl,
	)

	// Open database handle
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Ping database to verify connection.
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	client := &DBClient{handle: db}

	return client, client.applyMigrations(db, name)
}

func (c *DBClient) Close() error {
	return c.handle.Close()
}

func (c *DBClient) applyMigrations(db *sql.DB, name string) error {
	tmpDir, err := os.MkdirTemp("", "tiros-"+app.Version)
	if err != nil {
		return fmt.Errorf("create migrations tmp dir: %w", err)
	}
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			log.Warnw("Could not clean up tmp directory", "tmpDir", tmpDir, "err", err)
		}
	}()
	log.Debugw("Created temporary directory", "dir", tmpDir)

	err = fs.WalkDir(migrations, ".", func(path string, d fs.DirEntry, err error) error {
		join := filepath.Join(tmpDir, path)
		if d.IsDir() {
			return os.MkdirAll(join, 0755)
		}

		data, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		return os.WriteFile(join, data, 0644)
	})
	if err != nil {
		return fmt.Errorf("create migrations files: %w", err)
	}

	// Apply migrations
	driver, err := postgres.WithInstance(db, &postgres.Config{})
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
