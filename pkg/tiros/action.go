package tiros

//import (
//	"context"
//	"encoding/json"
//	"fmt"
//	"time"
//
//	awssdk "github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/session"
//	kubo "github.com/guseggert/clustertest-kubo"
//	"github.com/guseggert/clustertest/cluster"
//	"github.com/guseggert/clustertest/cluster/aws"
//	"github.com/guseggert/clustertest/cluster/basic"
//	log "github.com/sirupsen/logrus"
//	"github.com/volatiletech/null/v8"
//	"github.com/volatiletech/sqlboiler/v4/boil"
//	"golang.org/x/sync/errgroup"
//
//	"github.com/dennis-tra/tiros/pkg/config"
//	"github.com/dennis-tra/tiros/pkg/db"
//)
//
//func Action(ctx context.Context, conf config.RunConfig) error {
//	db, err := db.InitClient(ctx, conf.DatabaseHost, conf.DatabasePort, conf.DatabaseName, conf.DatabaseUser, conf.DatabasePassword, conf.DatabaseSSLMode)
//	if err != nil {
//		return fmt.Errorf("initializing database connection: %w", err)
//	}
//	defer db.Close()
//
//	dbRun, err := db.InsertRun(ctx, conf)
//	if err != nil {
//		return fmt.Errorf("initializing run: %w", err)
//	}
//
//	var clusterImpls []cluster.Cluster
//	for idx, region := range conf.Regions {
//		// capture loop variable
//		r := region
//
//		clusterImpl := aws.NewCluster().
//			WithNodeAgentBin(conf.NodeAgent).
//			WithSession(session.Must(session.NewSession(&awssdk.Config{Region: &r}))).
//			WithPublicSubnetID(conf.SubnetIDs[idx]).
//			WithInstanceProfileARN(conf.InstanceProfileARNs[idx]).
//			WithInstanceSecurityGroupID(conf.InstanceSecurityGroupIDs[idx]).
//			WithS3BucketARN(conf.S3BucketARNs[idx]).
//			WithInstanceType(conf.InstanceType)
//
//		clusterImpls = append(clusterImpls, clusterImpl)
//	}
//
//	// For each version, load the Kubo binary, initialize the repo, and run the daemon.
//	errg := errgroup.Group{}
//	for i, clusterImpl := range clusterImpls {
//		logEntry := log.WithFields(log.Fields{
//			"versions":        conf.Versions,
//			"nodesPerVersion": conf.NodesPerVersion,
//			"websites":        conf.Websites,
//			"times":           conf.Times,
//			"settleShort":     conf.SettleShort,
//			"settleLong":      conf.SettleLong,
//		})
//
//		ci := clusterImpl
//		region := conf.Regions[i]
//
//		errg.Go(func() error {
//			kc, nodes, err := setupNodes(ctx, conf, ci, region)
//			if err != nil {
//				return fmt.Errorf("setting up nodes: %w", err)
//			}
//			defer kc.Cleanup()
//
//			logEntry.Infoln("Daemons running, waiting to settle...\n")
//			time.Sleep(conf.SettleShort)
//
//			err = runRegion(ctx, conf, db, dbRun, nodes, region)
//			if err != nil {
//				return fmt.Errorf("running region experiment: %w", err)
//			}
//
//			logEntry.Infof("Waiting %s to settle...\n", conf.SettleLong)
//			time.Sleep(conf.SettleLong)
//
//			err = runRegion(ctx, conf, db, dbRun, nodes, region)
//			if err != nil {
//				return fmt.Errorf("running region experiment: %w", err)
//			}
//
//			return nil
//		})
//
//	}
//
//	defer func() {
//		dbRun.FinishedAt = null.TimeFrom(time.Now())
//		if _, err = dbRun.Update(ctx, db.handle, boil.Infer()); err != nil {
//			log.WithError(err).Warn("Could not update measurement run")
//		}
//	}()
//
//	return errg.Wait()
//}
//
//func setupNodes(ctx context.Context, conf config.RunConfig, clus cluster.Cluster, region string) (*kubo.Cluster, []*kubo.Node, error) {
//	logEntry := log.With("versions", conf.versions, "nodesPerVersion", conf.nodesPerVersion, "websites", conf.websites, "times", conf.times, "settleShort", conf.settleShort, "settleLong", conf.settleLong, "region", region)
//	logEntry.Infow("Setting up nodes...")
//
//	c := kubo.New(basic.New(clus).WithLogger(log))
//
//	logEntry.Infof("Launching %d nodes in %s\n", len(conf.versions)*conf.nodesPerVersion, region)
//	nodes := c.MustNewNodes(len(conf.versions) * conf.nodesPerVersion)
//	var nodeVersions []string
//	for i, v := range conf.Versions {
//		for j := 0; j < conf.NodesPerVersion; j++ {
//			node := nodes[i*conf.NodesPerVersion+j]
//			node.WithKuboVersion(v)
//			nodeVersions = append(nodeVersions, v)
//		}
//	}
//
//	group, groupCtx := errgroup.WithContext(ctx)
//	for _, node := range nodes {
//		n := node.Context(groupCtx)
//		orgN := node
//
//		group.Go(func() error {
//			if err := n.LoadBinary(); err != nil {
//				return fmt.Errorf("loading binary: %w", err)
//			}
//
//			if err := n.Init(); err != nil {
//				return fmt.Errorf("initializing kubo: %w", err)
//			}
//
//			if err := n.ConfigureForRemote(); err != nil {
//				return fmt.Errorf("configuring kubo: %w", err)
//			}
//
//			if _, err := n.Context(ctx).StartDaemonAndWaitForAPI(); err != nil {
//				return fmt.Errorf("waiting for kubo to startup: %w", err)
//			}
//
//			orgN.APIAvailableSince = time.Now()
//
//			return nil
//		})
//	}
//
//	logEntry.Infoln("Setting up nodes...")
//	err := group.Wait()
//	if err != nil {
//		c.Cleanup()
//		return nil, nil, fmt.Errorf("waiting on nodes to setup: %w", err)
//	}
//
//	return c, nodes, nil
//}
//
//func runRegion(ctx context.Context, conf *config, dbClient *db.DBClient, dbRun *models2.Run, nodes []*kubo.Node, region string) error {
//	group, groupCtx := errgroup.WithContext(ctx)
//	for i, node := range nodes {
//		node := node.Context(groupCtx)
//		nodeNum := i
//		group.Go(func() error {
//			if err := preparePhantomas(node); err != nil {
//				return fmt.Errorf("prepare phantomas: %w", err)
//			}
//
//			gatewayURL, err := node.GatewayURL()
//			if err != nil {
//				return fmt.Errorf("getting gateway url: %w", err)
//			}
//
//			for _, website := range conf.websites {
//				for i := 0; i < conf.times; i++ {
//					err := requestURL(groupCtx, conf, dbClient, dbRun, node, region, i, nodeNum, website, models2.MeasurementTypeKUBO, fmt.Sprintf("%s/ipns/%s", gatewayURL, website))
//					if err != nil {
//						return err
//					}
//				}
//			}
//
//			for _, website := range conf.websites {
//				for i := 0; i < conf.times; i++ {
//					err := requestURL(groupCtx, conf, dbClient, dbRun, node, region, i, nodeNum, website, models2.MeasurementTypeHTTP, fmt.Sprintf("https://%s", website))
//					if err != nil {
//						return err
//					}
//				}
//			}
//
//			return nil
//		})
//	}
//
//	return group.Wait()
//}
//
//func requestURL(ctx context.Context, conf *config, dbClient *db.DBClient, dbRun *models2.Run, node *kubo.Node, region string, try int, nodeNum int, website string, mType string, url string) error {
//	logParams := log.With("region", region, "version", node.MustVersion(), "url", url, "try_num", try, "node_num", nodeNum)
//
//	measurement := models2.Measurement{
//		RunID:        dbRun.ID,
//		Region:       region,
//		Website:      website,
//		URL:          url,
//		Version:      node.MustVersion(),
//		Type:         mType,
//		Try:          int16(try),
//		Node:         int16(nodeNum),
//		InstanceType: conf.instanceType,
//		Uptime:       fmt.Sprintf("%f seconds", time.Since(node.APIAvailableSince).Seconds()),
//	}
//
//	logParams.Infow("Requesting website...", url, url)
//	metrics, err := runPhantomas(ctx, node, measurement.URL)
//	if err != nil {
//		logParams.Infow("Error running phantomas", "err", err)
//		measurement.Error = null.StringFrom(err.Error())
//	} else {
//		logParams.Infow("Measured Metrics")
//
//		metricsDat, err := json.Marshal(metrics)
//		if err != nil {
//			return fmt.Errorf("marshalling metrics: %w", err)
//		}
//		measurement.Metrics = null.JSONFrom(metricsDat)
//	}
//
//	logParams.Infow("Inserting measurement...")
//	if err := measurement.Insert(ctx, dbClient.handle, boil.Infer()); err != nil {
//		log.Warnw("error inserting row", "err", err)
//	}
//
//	gcCtx, cancelGC := context.WithTimeout(ctx, 10*time.Second)
//	err = kubo.ProcMust(node.Context(gcCtx).RunKubo(cluster.StartProcRequest{
//		Args: []string{"repo", "gc"},
//	}))
//	if err != nil {
//		cancelGC()
//		return fmt.Errorf("%s node %d running gc: %w", region, nodeNum, err)
//	}
//	cancelGC()
//
//	return nil
//}
