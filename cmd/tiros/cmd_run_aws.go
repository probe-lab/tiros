package main

import (
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guseggert/clustertest/cluster/aws"
	"github.com/guseggert/clustertest/cluster/basic"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/tiros/pkg/config"
	"github.com/dennis-tra/tiros/pkg/tiros"
)

var RunAWSCommand = &cli.Command{
	Name: "aws",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:        "public-subnet-ids",
			Usage:       "The public subnet IDs to run the cluster in",
			EnvVars:     []string{"TIROS_RUN_AWS_PUBLIC_SUBNET_IDS"},
			Value:       cli.NewStringSlice(config.DefaultRunAWSConfig.SubnetIDs...),
			DefaultText: strings.Join(config.DefaultRunAWSConfig.SubnetIDs, ","),
		},
		&cli.StringSliceFlag{
			Name:        "instance-profile-arns",
			Usage:       "The instance profiles to run the Kubo nodes with",
			EnvVars:     []string{"TIROS_RUN_AWS_INSTANCE_PROFILE_ARNS"},
			Value:       cli.NewStringSlice(config.ARNsToStrings(config.DefaultRunAWSConfig.InstanceProfileARNs)...),
			DefaultText: strings.Join(config.ARNsToStrings(config.DefaultRunAWSConfig.InstanceProfileARNs), ","),
		},
		&cli.StringSliceFlag{
			Name:        "s3-bucket-arns",
			Usage:       "The S3 buckets where the nodeagent binaries are stored",
			EnvVars:     []string{"TIROS_RUN_AWS_S3_BUCKET_ARNS"},
			Value:       cli.NewStringSlice(config.ARNsToStrings(config.DefaultRunAWSConfig.S3BucketARNs)...),
			DefaultText: strings.Join(config.ARNsToStrings(config.DefaultRunAWSConfig.S3BucketARNs), ","),
		},
		&cli.StringSliceFlag{
			Name:        "instance-security-group-ids",
			Usage:       "The security groups of the Kubo instances",
			EnvVars:     []string{"TIROS_RUN_AWS_SECURITY_GROUP_IDS"},
			Value:       cli.NewStringSlice(config.DefaultRunAWSConfig.InstanceSecurityGroupIDs...),
			DefaultText: strings.Join(config.DefaultRunAWSConfig.InstanceSecurityGroupIDs, ","),
		},
		&cli.StringFlag{
			Name:        "key-name",
			Usage:       "The SSH key pair name to apply to the EC2 instances",
			EnvVars:     []string{"TIROS_RUN_AWS_KEY_NAME"},
			Value:       config.DefaultRunAWSConfig.KeyName,
			DefaultText: config.DefaultRunAWSConfig.KeyName,
		},
		&cli.StringSliceFlag{
			Name:        "regions",
			Usage:       "the AWS regions to use, if using an AWS cluster",
			EnvVars:     []string{"TIROS_RUN_AWS_REGIONS"},
			Value:       cli.NewStringSlice(config.DefaultRunConfig.Regions...),
			DefaultText: strings.Join(config.DefaultRunConfig.Regions, ","),
		},
	},
	Action: RunAWSAction,
}

func RunAWSAction(c *cli.Context) error {
	log.Infoln("Starting Tiros AWS scheduler...")

	conf, err := config.DefaultRunAWSConfig.Apply(c)
	if err != nil {
		return fmt.Errorf("parsing aws config: %w", err)
	}

	log.Infoln("Configuration:")
	fmt.Println(conf.String())

	// starting cluster in all regions
	exp := tiros.NewExperiment(conf.RunConfig)

	for i, region := range conf.Regions {
		i := i
		region := region
		log.WithField("region", region).WithField("instanceType", conf.InstanceType).Infoln("Starting cluster...")
		cl := aws.NewCluster().
			WithNodeAgentBin(conf.NodeAgent).
			WithSession(session.Must(session.NewSession(&awssdk.Config{Region: &region}))).
			WithPublicSubnetID(conf.SubnetIDs[i]).
			WithInstanceProfileARN(conf.InstanceProfileARNs[i]).
			WithInstanceSecurityGroupID(conf.InstanceSecurityGroupIDs[i]).
			WithS3BucketARN(conf.S3BucketARNs[i]).
			WithInstanceType(conf.InstanceType).
			WithKeyName(conf.KeyName)

		exp.Cluster[region] = basic.New(cl.Context(c.Context))
	}

	return RunAction(c, exp)
}
