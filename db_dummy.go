package main

import (
	"context"

	"github.com/dennis-tra/tiros/models"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/urfave/cli/v2"
)

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
