package main

//
//import (
//	"fmt"
//	"strings"
//	"time"
//
//	awssdk "github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/session"
//	"github.com/dennis-tra/tiros/pkg/config"
//	kubo "github.com/guseggert/clustertest-kubo"
//	"github.com/guseggert/clustertest/cluster/aws"
//	"github.com/guseggert/clustertest/cluster/basic"
//	log "github.com/sirupsen/logrus"
//	"github.com/urfave/cli/v2"
//	"golang.org/x/sync/errgroup"
//)
//
//var RunAWSCommand = &cli.Command{
//	Name: "aws",
//	Flags: []cli.Flag{
//		&cli.StringSliceFlag{
//			Name:        "public-subnet-ids",
//			Usage:       "The public subnet IDs to run the cluster in",
//			EnvVars:     []string{"TIROS_PUBLIC_SUBNET_IDS"},
//			Value:       cli.NewStringSlice(config.DefaultRunConfig.SubnetIDs...),
//			DefaultText: strings.Join(config.DefaultRunConfig.SubnetIDs, ","),
//		},
//		&cli.StringSliceFlag{
//			Name:        "instance-profile-arns",
//			Usage:       "The instance profiles to run the Kubo nodes with",
//			EnvVars:     []string{"TIROS_INSTANCE_PROFILE_ARNS"},
//			Value:       cli.NewStringSlice(config.ARNsToStrings(config.DefaultRunConfig.InstanceProfileARNs)...),
//			DefaultText: strings.Join(config.ARNsToStrings(config.DefaultRunConfig.InstanceProfileARNs), ","),
//		},
//		&cli.StringSliceFlag{
//			Name:        "s3-bucket-arns",
//			Usage:       "The S3 buckets where the nodeagent binaries are stored",
//			EnvVars:     []string{"TIROS_S3_BUCKET_ARNS"},
//			Value:       cli.NewStringSlice(config.ARNsToStrings(config.DefaultRunConfig.S3BucketARNs)...),
//			DefaultText: strings.Join(config.ARNsToStrings(config.DefaultRunConfig.S3BucketARNs), ","),
//		},
//		&cli.StringSliceFlag{
//			Name:        "instance-security-group-ids",
//			Usage:       "The security groups of the Kubo instances",
//			EnvVars:     []string{"TIROS_SECURITY_GROUP_IDS"},
//			Value:       cli.NewStringSlice(config.DefaultRunConfig.InstanceSecurityGroupIDs...),
//			DefaultText: strings.Join(config.DefaultRunConfig.InstanceSecurityGroupIDs, ","),
//		},
//		&cli.StringFlag{
//			Name:        "instance-type",
//			Usage:       "the EC2 instance type to run the experiment on",
//			EnvVars:     []string{"TIROS_INSTANCE_TYPE"},
//			Value:       config.DefaultRunConfig.InstanceType,
//			DefaultText: config.DefaultRunConfig.InstanceType,
//		},
//	},
//	Action: RunAWSAction,
//}
//
//func RunAWSAction(c *cli.Context) error {
//	log.Infoln("Starting Parsec docker scheduler...")
//
//	conf, err := config.DefaultRunAWSConfig.Apply(c)
//	if err != nil {
//		return err
//	}
//
//	nodesChan := make(chan []*kubo.Node, len(conf.Regions))
//
//	errg := errgroup.Group{}
//	for i, region := range conf.Regions {
//		i := i
//		region := region
//		errg.Go(func() error {
//			logEntry := log.WithFields(log.Fields{
//				"versions":        conf.Versions,
//				"nodesPerVersion": conf.NodesPerVersion,
//				"websites":        conf.Websites,
//				"times":           conf.Times,
//				"settleShort":     conf.SettleShort,
//				"settleLong":      conf.SettleLong,
//				"region":          region,
//			})
//
//			cl := aws.NewCluster().
//				WithNodeAgentBin(conf.NodeAgent).
//				WithSession(session.Must(session.NewSession(&awssdk.Config{Region: &region}))).
//				WithPublicSubnetID(conf.SubnetIDs[i]).
//				WithInstanceProfileARN(conf.InstanceProfileARNs[i]).
//				WithInstanceSecurityGroupID(conf.InstanceSecurityGroupIDs[i]).
//				WithS3BucketARN(conf.S3BucketARNs[i]).
//				WithInstanceType(conf.InstanceType)
//
//			kc := kubo.New(basic.New(cl).Context(c.Context))
//
//			logEntry.Infoln("Starting new nodes")
//			nodes, err := kc.NewNodes(len(conf.Versions) * conf.NodesPerVersion)
//			if err != nil {
//				return fmt.Errorf("new nodes: %w", err)
//			}
//
//			var nodeVersions []string
//			for i, v := range conf.Versions {
//				for j := 0; j < conf.NodesPerVersion; j++ {
//					node := nodes[i*conf.NodesPerVersion+j]
//					node.WithKuboVersion(v)
//					nodeVersions = append(nodeVersions, v)
//				}
//			}
//
//			group, groupCtx := errgroup.WithContext(c.Context)
//			for _, node := range nodes {
//				n := node.Context(groupCtx)
//				orgN := node
//
//				group.Go(func() error {
//					if err := n.LoadBinary(); err != nil {
//						return fmt.Errorf("loading binary: %w", err)
//					}
//
//					if err := n.Init(); err != nil {
//						return fmt.Errorf("initializing kubo: %w", err)
//					}
//
//					if err := n.ConfigureForRemote(); err != nil {
//						return fmt.Errorf("configuring kubo: %w", err)
//					}
//
//					if _, err := n.Context(c.Context).StartDaemonAndWaitForAPI(); err != nil {
//						return fmt.Errorf("waiting for kubo to startup: %w", err)
//					}
//
//					orgN.APIAvailableSince = time.Now()
//
//					return nil
//				})
//			}
//
//			nodesChan <- nodes
//
//			return nil
//		})
//	}
//
//	if err = errg.Wait(); err != nil {
//		return fmt.Errorf("wait for new nodes: %w", err)
//	}
//
//	nodes := make([][]*kubo.Node, len(conf.Regions))
//	for i := 0; i < len(conf.Regions); i++ {
//		nodes[i] = <-nodesChan
//	}
//	close(nodesChan)
//
//	return RunAction(c, nodes)
//}
