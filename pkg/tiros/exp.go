package tiros

import (
	"context"
	"fmt"
	"time"

	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster"

	"github.com/guseggert/clustertest/cluster/basic"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/dennis-tra/tiros/pkg/config"
	"github.com/dennis-tra/tiros/pkg/db"
	"github.com/dennis-tra/tiros/pkg/models"
)

type Experiment struct {
	Cluster map[string]*basic.Cluster
	db      *db.DBClient
	nodes   []*Node
	dbRun   *models.Run
	conf    config.RunConfig
}

func NewExperiment(conf config.RunConfig) *Experiment {
	return &Experiment{
		conf:    conf,
		Cluster: map[string]*basic.Cluster{},
	}
}

func (e *Experiment) Init(ctx context.Context) error {
	var err error
	e.db, err = db.InitClient(ctx, e.conf.DatabaseHost, e.conf.DatabasePort, e.conf.DatabaseName, e.conf.DatabaseUser, e.conf.DatabasePassword, e.conf.DatabaseSSLMode)
	if err != nil {
		return fmt.Errorf("init db connection: %w", err)
	}

	errg := errgroup.Group{}
	nodesChan := make(chan []*Node, len(e.Cluster))
	for region, clus := range e.Cluster {

		tc := NewCluster(clus, region, e.conf.Versions, e.conf.NodesPerVersion)

		errg.Go(func() error {
			nodes, err := tc.NewNodes()
			if err != nil {
				return err
			}

			nodesChan <- nodes

			return nil
		})
	}
	if err = errg.Wait(); err != nil {
		return fmt.Errorf("init tiros nodes: %w", err)
	}

	tnodes := []*Node{}
	for i := 0; i < len(e.Cluster); i++ {
		nodes := <-nodesChan
		tnodes = append(tnodes, nodes...)
	}
	close(nodesChan)

	e.nodes = tnodes

	return nil
}

func (e *Experiment) Run(ctx context.Context) error {
	log.WithField("nodeCount", len(e.nodes)).Infoln("Starting website probing")
	defer log.WithField("nodeCount", len(e.nodes)).Infoln("Stopped website probing")

	var err error
	e.dbRun, err = e.db.InsertRun(ctx, e.conf)
	if err != nil {
		return fmt.Errorf("initializing run: %w", err)
	}

	errg, errCtx := errgroup.WithContext(ctx)
	for _, tnode := range e.nodes {
		tnode := tnode
		errg.Go(func() error {
			tnode.logEntry().Infoln("Start probing...")
			defer tnode.logEntry().Infoln("Done probing!")

			tnode.logEntry().Infoln("Sleeping for", e.conf.SettleShort)
			time.Sleep(e.conf.SettleShort)

			if err := e.probe(errCtx, tnode, models.MeasurementTypeKUBO); err != nil {
				return fmt.Errorf("probe kubo websites: %w", err)
			}

			if err := e.probe(errCtx, tnode, models.MeasurementTypeHTTP); err != nil {
				return fmt.Errorf("probe HTTP websites: %w", err)
			}

			tnode.logEntry().Infoln("Sleeping for", e.conf.SettleLong)
			time.Sleep(e.conf.SettleLong)

			if err := e.probe(errCtx, tnode, models.MeasurementTypeKUBO); err != nil {
				return fmt.Errorf("probe kubo websites: %w", err)
			}

			if err := e.probe(errCtx, tnode, models.MeasurementTypeHTTP); err != nil {
				return fmt.Errorf("probe HTTP websites: %w", err)
			}

			return nil
		})
	}

	return errg.Wait()
}

func (e *Experiment) probe(ctx context.Context, tnode *Node, mType string) error {
	logEntry := tnode.logEntry().WithField("type", mType)

	logEntry.Infoln("Probing websites")
	defer logEntry.Infoln("Probing websites done.")

	for _, website := range e.conf.Websites {
		for i := 0; i < e.conf.Times; i++ {
			result, err := tnode.probe(ctx, website, mType)
			if err != nil {
				logEntry.WithError(err).Warnln("Couldn't probe website", website)
				continue
			}

			logEntry.WithFields(log.Fields{
				"ttfb": p2f(result.TimeToFirstByte),
				"lcp":  p2f(result.LargestContentfulPaint),
				"fcp":  p2f(result.FirstContentFulPaint),
			}).WithError(err).Infoln("Probed website", website)

			metrics, err := result.NullJSON()
			if err != nil {
				logEntry.WithError(err).Warnln("Couldn't extract metrics from probe result")
				continue
			}

			m := &models.Measurement{
				RunID:        e.dbRun.ID,
				Region:       tnode.Cluster.Region,
				Website:      website,
				Version:      tnode.MustVersion(),
				Type:         mType,
				URL:          result.URL,
				Try:          int16(i),
				Node:         int16(tnode.NodeNum),
				InstanceType: e.conf.InstanceType,
				Metrics:      metrics,
				Error:        result.NullError(),
				Uptime:       fmt.Sprintf("%f seconds", time.Since(tnode.APIAvailableSince).Seconds()),
			}

			if _, err := e.db.InsertMeasurement(ctx, m); err != nil {
				return fmt.Errorf("insert measurement: %w", err)
			}

			if mType == models.MeasurementTypeHTTP {
				continue
			}

			// Garbage collect kubo node
			gcCtx, cancelGC := context.WithTimeout(ctx, 10*time.Second)
			err = kubo.ProcMust(tnode.Context(gcCtx).RunKubo(cluster.StartProcRequest{Args: []string{"repo", "gc"}}))
			cancelGC()
			if err != nil {
				return fmt.Errorf("%s node %d running gc: %w", tnode.Cluster.Region, tnode.NodeNum, err)
			}
		}
	}

	return nil
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return -1
	}
	return *ptr
}
