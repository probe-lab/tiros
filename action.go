package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dennis-tra/tiros/models"
	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster/basic"
	"github.com/volatiletech/sqlboiler/v4/boil"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/guseggert/clustertest/cluster"
	"github.com/guseggert/clustertest/cluster/aws"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var log *zap.SugaredLogger

func Action(cliCtx *cli.Context) error {
	runID := uuid.NewString()

	nodeagent := cliCtx.String("nodeagent")
	regions := cliCtx.StringSlice("regions")
	subnetIDs := cliCtx.StringSlice("public-subnet-ids")
	instanceProfileARNs := cliCtx.StringSlice("instance-profile-arns")
	instanceSecurityGroupIDs := cliCtx.StringSlice("instance-security-group-ids")
	s3BucketARNs := cliCtx.StringSlice("s3-bucket-arns")

	l, err := newLogger(cliCtx.Bool("verbose"))
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	log = l.Sugar()

	db, err := InitDB(cliCtx.String("db-host"), cliCtx.Int("db-port"), cliCtx.String("db-name"), cliCtx.String("db-user"), cliCtx.String("db-password"), cliCtx.String("db-sslmode"))
	if err != nil {
		return fmt.Errorf("initializing database connection: %w", err)
	}
	defer db.Close()

	var clusterImpls []cluster.Cluster
	for idx, region := range regions {
		// capture loop variable
		r := region

		iparn, err := arn.Parse(instanceProfileARNs[idx])
		if err != nil {
			return fmt.Errorf("error parsing instnace profile arn: %w", err)
		}

		s3arn, err := arn.Parse(s3BucketARNs[idx])
		if err != nil {
			return fmt.Errorf("error parsing s3 bucket arn: %w", err)
		}

		clusterImpl := aws.NewCluster().
			WithNodeAgentBin(nodeagent).
			WithSession(session.Must(session.NewSession(&awssdk.Config{Region: &r}))).
			WithLogger(log).
			WithPublicSubnetID(subnetIDs[idx]).
			WithInstanceProfileARN(iparn).
			WithInstanceSecurityGroupID(instanceSecurityGroupIDs[idx]).
			WithS3BucketARN(s3arn)

		clusterImpls = append(clusterImpls, clusterImpl)
	}

	// For each version, load the Kubo binary, initialize the repo, and run the daemon.
	errg := errgroup.Group{}
	for i, clusterImpl := range clusterImpls {
		ci := clusterImpl
		region := regions[i]
		errg.Go(func() error {
			return runRegion(cliCtx, db, runID, ci, region)
		})
	}

	return errg.Wait()
}

func runRegion(cliCtx *cli.Context, dbClient *DBClient, runID string, clus cluster.Cluster, region string) error {
	ctx := cliCtx.Context

	versions := cliCtx.StringSlice("versions")
	nodesPerVersion := cliCtx.Int("nodes-per-version")
	urls := cliCtx.StringSlice("urls")
	times := cliCtx.Int("times")
	settle := cliCtx.Duration("settle")

	with := log.With("region", region)
	with.Infow("Run region test", "versions", versions, "nodesPerVersion", nodesPerVersion, "urls", urls, "times", times, "settle", settle)

	c := kubo.New(basic.New(clus).WithLogger(log))
	defer c.Cleanup()

	with.Infof("Launching %d nodes in %s\n", len(versions)*nodesPerVersion, region)

	nodes := c.MustNewNodes(len(versions) * nodesPerVersion)
	var nodeVersions []string
	for i, v := range versions {
		for j := 0; j < nodesPerVersion; j++ {
			node := nodes[i*nodesPerVersion+j]
			node.WithKuboVersion(v)
			nodeVersions = append(nodeVersions, v)
		}
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		node := node
		group.Go(func() error {
			node = node.Context(groupCtx)

			if err := node.LoadBinary(); err != nil {
				return fmt.Errorf("loading binary: %w", err)
			}

			if err := node.Init(); err != nil {
				return fmt.Errorf("initializing kubo: %w", err)
			}

			if err := node.ConfigureForRemote(); err != nil {
				return fmt.Errorf("configuring kubo: %w", err)
			}

			if _, err := node.Context(ctx).StartDaemonAndWaitForAPI(); err != nil {
				return fmt.Errorf("waiting for kubo to startup: %w", err)
			}

			return nil
		})
	}

	with.Infoln("Setting up nodes...")
	err := group.Wait()
	if err != nil {
		return fmt.Errorf("waiting on nodes to setup: %w", err)
	}

	with.Infoln("Daemons running, waiting to settle...\n")
	time.Sleep(settle)

	group, groupCtx = errgroup.WithContext(ctx)
	for i, node := range nodes {
		node := node.Context(groupCtx)
		nodeNum := i
		group.Go(func() error {
			for _, url := range urls {
				for i := 0; i < times; i++ {
					logParams := log.With("region", region, "version", node.MustVersion(), "url", url, "try_num", i, "node_num", nodeNum)

					logParams.Infow("Requesting website...")
					loadTime, err := runPhantomas(groupCtx, node, url)
					if err != nil {
						logParams.Warnw("Error running phantomas", "err", err)
						continue
					}

					logParams.Infow("Inserting measurement...", "latency_s", loadTime.Seconds())
					measurement := models.Measurement{
						RunID:   runID,
						Region:  region,
						URL:     url,
						Version: node.MustVersion(),
						NodeNum: int16(nodeNum),
						Latency: loadTime.Seconds(),
					}
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

	if err = group.Wait(); err != nil {
		return fmt.Errorf("running %s test: %w", region, err)
	}

	return nil
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
