package main

import (
	"context"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/probe-lab/tiros/models"
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

func (D DBDummyClient) InsertRun(c *cli.Context, ipfsImpl string, version string) (*models.Run, error) {
	return &models.Run{
		ID:       2,
		Region:   "dummy",
		IpfsImpl: ipfsImpl,
	}, nil
}

func (D DBDummyClient) InsertMeasurement(ctx context.Context, m *models.Measurement) (*models.Measurement, error) {
	return nil, nil
}
