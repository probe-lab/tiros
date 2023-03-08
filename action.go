package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster"
	"github.com/guseggert/clustertest/cluster/aws"
	"github.com/guseggert/clustertest/cluster/basic"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/dennis-tra/tiros/models"
)

var log *zap.SugaredLogger

func Action(ctx context.Context, conf *config) error {
	l, err := newLogger(conf.verbose)
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	log = l.Sugar()

	db, err := InitDB(conf.dbHost, conf.dbPort, conf.dbName, conf.dbUser, conf.dbPassword, conf.dbSSL)
	if err != nil {
		return fmt.Errorf("initializing database connection: %w", err)
	}
	defer db.Close()

	run := &models.Run{
		Regions:         conf.regions,
		Urls:            conf.websites,
		SettleShort:     conf.settleShort.Seconds(),
		SettleLong:      conf.settleLong.Seconds(),
		NodesPerVersion: int16(conf.nodesPerVersion),
		Versions:        conf.versions,
		Times:           int16(conf.times),
	}
	if err = run.Insert(ctx, db.handle, boil.Infer()); err != nil {
		return fmt.Errorf("initializing run: %w", err)
	}

	var clusterImpls []cluster.Cluster
	for idx, region := range conf.regions {
		// capture loop variable
		r := region

		clusterImpl := aws.NewCluster().
			WithNodeAgentBin(conf.nodeagent).
			WithSession(session.Must(session.NewSession(&awssdk.Config{Region: &r}))).
			WithLogger(log).
			WithPublicSubnetID(conf.subnetIDs[idx]).
			WithInstanceProfileARN(conf.instanceProfileARNs[idx]).
			WithInstanceSecurityGroupID(conf.instanceSecurityGroupIDs[idx]).
			WithS3BucketARN(conf.s3BucketARNs[idx]).
			WithInstanceType(conf.instanceType)

		clusterImpls = append(clusterImpls, clusterImpl)
	}

	// For each version, load the Kubo binary, initialize the repo, and run the daemon.
	errg := errgroup.Group{}
	for i, clusterImpl := range clusterImpls {
		logEntry := log.With("versions", conf.versions, "nodesPerVersion", conf.nodesPerVersion, "websites", conf.websites, "times", conf.times, "settleShort", conf.settleShort, "settleLong", conf.settleLong)
		ci := clusterImpl
		region := conf.regions[i]

		errg.Go(func() error {
			kc, nodes, err := setupNodes(ctx, conf, ci, region)
			if err != nil {
				return fmt.Errorf("setting up nodes: %w", err)
			}
			defer kc.Cleanup()

			logEntry.Infoln("Daemons running, waiting to settle...\n")
			time.Sleep(conf.settleShort)

			err = runRegion(ctx, conf, db, run, nodes, region)
			if err != nil {
				return fmt.Errorf("running region experiment: %w", err)
			}

			logEntry.Infof("Waiting %s to settle...\n", conf.settleLong)
			time.Sleep(conf.settleLong)

			err = runRegion(ctx, conf, db, run, nodes, region)
			if err != nil {
				return fmt.Errorf("running region experiment: %w", err)
			}

			return nil
		})

	}

	defer func() {
		run.FinishedAt = null.TimeFrom(time.Now())
		if _, err = run.Update(ctx, db.handle, boil.Infer()); err != nil {
			log.Warnw("Could not update measurement run", "err", err)
		}
	}()

	return errg.Wait()
}

func setupNodes(ctx context.Context, conf *config, clus cluster.Cluster, region string) (*kubo.Cluster, []*kubo.Node, error) {
	logEntry := log.With("versions", conf.versions, "nodesPerVersion", conf.nodesPerVersion, "websites", conf.websites, "times", conf.times, "settleShort", conf.settleShort, "settleLong", conf.settleLong, "region", region)
	logEntry.Infow("Setting up nodes...")

	c := kubo.New(basic.New(clus).WithLogger(log))

	logEntry.Infof("Launching %d nodes in %s\n", len(conf.versions)*conf.nodesPerVersion, region)
	nodes := c.MustNewNodes(len(conf.versions) * conf.nodesPerVersion)
	var nodeVersions []string
	for i, v := range conf.versions {
		for j := 0; j < conf.nodesPerVersion; j++ {
			node := nodes[i*conf.nodesPerVersion+j]
			node.WithKuboVersion(v)
			node.WithNodeLogger(log.With("region", region, "node_num", j))
			nodeVersions = append(nodeVersions, v)
		}
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		n := node.Context(groupCtx)
		orgN := node

		group.Go(func() error {
			if err := n.LoadBinary(); err != nil {
				return fmt.Errorf("loading binary: %w", err)
			}

			if err := n.Init(); err != nil {
				return fmt.Errorf("initializing kubo: %w", err)
			}

			if err := n.ConfigureForRemote(); err != nil {
				return fmt.Errorf("configuring kubo: %w", err)
			}

			if _, err := n.Context(ctx).StartDaemonAndWaitForAPI(); err != nil {
				return fmt.Errorf("waiting for kubo to startup: %w", err)
			}

			orgN.APIAvailableSince = time.Now()

			return nil
		})
	}

	logEntry.Infoln("Setting up nodes...")
	err := group.Wait()
	if err != nil {
		c.Cleanup()
		return nil, nil, fmt.Errorf("waiting on nodes to setup: %w", err)
	}

	return c, nodes, nil
}

func runRegion(ctx context.Context, conf *config, dbClient *DBClient, dbRun *models.Run, nodes []*kubo.Node, region string) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for i, node := range nodes {
		node := node.Context(groupCtx)
		nodeNum := i
		group.Go(func() error {
			if err := preparePhantomas(node); err != nil {
				return fmt.Errorf("prepare phantomas: %w", err)
			}

			gatewayURL, err := node.GatewayURL()
			if err != nil {
				return fmt.Errorf("getting gateway url: %w", err)
			}

			for _, website := range conf.websites {
				for i := 0; i < conf.times; i++ {
					err := requestURL(groupCtx, conf, dbClient, dbRun, node, region, i, nodeNum, website, models.MeasurementTypeKUBO, fmt.Sprintf("%s/ipns/%s", gatewayURL, website))
					if err != nil {
						return err
					}
				}
			}

			for _, website := range conf.websites {
				for i := 0; i < conf.times; i++ {
					err := requestURL(groupCtx, conf, dbClient, dbRun, node, region, i, nodeNum, website, models.MeasurementTypeHTTP, fmt.Sprintf("https://%s", website))
					if err != nil {
						return err
					}
				}
			}

			return nil
		})
	}

	return group.Wait()
}

func requestURL(ctx context.Context, conf *config, dbClient *DBClient, dbRun *models.Run, node *kubo.Node, region string, try int, nodeNum int, website string, mType string, url string) error {
	logParams := log.With("region", region, "version", node.MustVersion(), "url", url, "try_num", try, "node_num", nodeNum)

	measurement := models.Measurement{
		RunID:        dbRun.ID,
		Region:       region,
		Website:      website,
		URL:          url,
		Version:      node.MustVersion(),
		Type:         mType,
		Try:          int16(try),
		Node:         int16(nodeNum),
		InstanceType: conf.instanceType,
		Uptime:       fmt.Sprintf("%f seconds", time.Since(node.APIAvailableSince).Seconds()),
	}

	logParams.Infow("Requesting website...", url, url)
	metrics, err := runPhantomas(ctx, node, measurement.URL)
	if err != nil {
		logParams.Infow("Error running phantomas", "err", err)
		measurement.Error = null.StringFrom(err.Error())
	} else {
		logParams.Infow("Measured Metrics")

		metricsDat, err := json.Marshal(metrics)
		if err != nil {
			return fmt.Errorf("marshalling metrics: %w", err)
		}
		measurement.Metrics = null.JSONFrom(metricsDat)
	}

	logParams.Infow("Inserting measurement...")
	if err := measurement.Insert(ctx, dbClient.handle, boil.Infer()); err != nil {
		log.Warnw("error inserting row", "err", err)
	}

	gcCtx, cancelGC := context.WithTimeout(ctx, 10*time.Second)
	err = kubo.ProcMust(node.Context(gcCtx).RunKubo(cluster.StartProcRequest{
		Args: []string{"repo", "gc"},
	}))
	if err != nil {
		cancelGC()
		return fmt.Errorf("%s node %d running gc: %w", region, nodeNum, err)
	}
	cancelGC()

	return nil
}

func newLogger(verbose bool) (*zap.Logger, error) {
	if verbose {
		return zap.NewDevelopment()
	} else {
		return zap.NewProduction()
	}
}
