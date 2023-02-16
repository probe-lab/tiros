package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
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
		Urls:            conf.urls,
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
		logEntry := log.With("versions", conf.versions, "nodesPerVersion", conf.nodesPerVersion, "urls", conf.urls, "times", conf.times, "settleShort", conf.settleShort, "settleLong", conf.settleLong)
		ci := clusterImpl
		region := conf.regions[i]

		errg.Go(func() error {
			nodes, readyTimes, err := setupNodes(ctx, conf, ci, region)
			if err != nil {
				return fmt.Errorf("setting up nodes: %w", err)
			}

			logEntry.Infoln("Daemons running, waiting to settle...\n")
			time.Sleep(conf.settleShort)

			err = runRegion(ctx, conf, db, run, nodes, readyTimes, region)
			if err != nil {
				return fmt.Errorf("running region experiment: %w", err)
			}

			logEntry.Infof("Waiting %s to settle...\n", conf.settleLong)
			time.Sleep(conf.settleLong)

			err = runRegion(ctx, conf, db, run, nodes, readyTimes, region)
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

func setupNodes(ctx context.Context, conf *config, clus cluster.Cluster, region string) ([]*kubo.Node, *sync.Map, error) {
	logEntry := log.With("versions", conf.versions, "nodesPerVersion", conf.nodesPerVersion, "urls", conf.urls, "times", conf.times, "settleShort", conf.settleShort, "settleLong", conf.settleLong)
	logEntry.Infow("Setting up nodes...")

	c := kubo.New(basic.New(clus).WithLogger(log))
	defer c.Cleanup()

	logEntry.Infof("Launching %d nodes in %s\n", len(conf.versions)*conf.nodesPerVersion, region)
	nodes := c.MustNewNodes(len(conf.versions) * conf.nodesPerVersion)
	var nodeVersions []string
	for i, v := range conf.versions {
		for j := 0; j < conf.nodesPerVersion; j++ {
			node := nodes[i*conf.nodesPerVersion+j]
			node.WithKuboVersion(v)
			nodeVersions = append(nodeVersions, v)
		}
	}

	readyTimes := &sync.Map{}

	group, groupCtx := errgroup.WithContext(ctx)
	for i, node := range nodes {
		n := node.Context(groupCtx)
		nodeNum := i

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

			readyTimes.Store(nodeNum, time.Now())

			return nil
		})
	}

	logEntry.Infoln("Setting up nodes...")
	err := group.Wait()
	if err != nil {
		return nil, &sync.Map{}, fmt.Errorf("waiting on nodes to setup: %w", err)
	}

	return nodes, readyTimes, nil
}

func runRegion(ctx context.Context, conf *config, dbClient *DBClient, dbRun *models.Run, nodes []*kubo.Node, readyTimes *sync.Map, region string) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for i, node := range nodes {
		node := node.Context(groupCtx)
		nodeNum := i
		group.Go(func() error {
			for _, url := range conf.urls {
				for i := 0; i < conf.times; i++ {
					logParams := log.With("region", region, "version", node.MustVersion(), "url", url, "try_num", i, "node_num", nodeNum)

					val, ok := readyTimes.Load(nodeNum)
					if !ok {
						return fmt.Errorf("node %d not found in map", nodeNum)
					}
					measurement := models.Measurement{
						RunID:   dbRun.ID,
						Region:  region,
						URL:     url,
						Version: node.MustVersion(),
						NodeNum: int16(nodeNum),
						Uptime:  fmt.Sprintf("%f seconds", time.Since(val.(time.Time)).Seconds()),
					}

					logParams.Infow("Requesting website...")
					loadTime, err := runPhantomas(groupCtx, node, url)
					if err != nil {
						logParams.Warnw("Error running phantomas", "err", err)
						measurement.Error = null.StringFrom(err.Error())
					} else {
						logParams.Infow("Measured latency", "latency", loadTime.Seconds())
						measurement.Latency = null.Float64From(loadTime.Seconds())
					}

					logParams.Infow("Inserting measurement...")
					if err := measurement.Insert(ctx, dbClient.handle, boil.Infer()); err != nil {
						log.Warnw("error inserting row", "err", err)
					}

					gcCtx, cancelGC := context.WithTimeout(groupCtx, 10*time.Second)
					err = kubo.ProcMust(node.Context(gcCtx).RunKubo(cluster.StartProcRequest{
						Args: []string{"repo", "gc"},
					}))
					if err != nil {
						cancelGC()
						return fmt.Errorf("%s node %d running gc: %w", region, nodeNum, err)
					}
					cancelGC()
				}
			}
			return nil
		})
	}

	return group.Wait()
}

type phantomasOutput struct {
	Metrics struct {
		PerformanceTimingPageLoad int
	}
}

func runPhantomas(ctx context.Context, node *kubo.Node, url string) (*time.Duration, error) {
	ctx, cancelCurl := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelCurl()

	gatewayURL, err := node.GatewayURL()
	if err != nil {
		return nil, err
	}

	_, err = node.Run(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"pull",
			"macbre/phantomas:latest",
		},
	})
	if err != nil {
		return nil, err
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	_, err = node.Run(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"run",
			"--network=host",
			"--privileged",
			"macbre/phantomas:latest",
			"/opt/phantomas/bin/phantomas.js",
			"--timeout=60",
			fmt.Sprintf("--url=%s%s", gatewayURL, url),
		},
		Stdout: stdout,
		Stderr: stderr,
	})

	if err != nil {
		fmt.Printf("stdout: %s\n", stdout)
		fmt.Printf("stderr: %s\n", stderr)
		return nil, err
	}
	out := &phantomasOutput{}
	err = json.Unmarshal(stdout.Bytes(), out)
	if err != nil {
		return nil, err
	}
	loadTime := time.Duration(out.Metrics.PerformanceTimingPageLoad) * time.Millisecond
	return &loadTime, nil
}

func newLogger(verbose bool) (*zap.Logger, error) {
	if verbose {
		return zap.NewDevelopment()
	} else {
		return zap.NewProduction()
	}
}
