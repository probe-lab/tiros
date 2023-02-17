package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/urfave/cli/v2"
)

type config struct {
	verbose                  bool
	nodeagent                string
	regions                  []string
	websites                 []string
	versions                 []string
	nodesPerVersion          int
	times                    int
	settleShort              time.Duration
	settleLong               time.Duration
	subnetIDs                []string
	instanceType             string
	instanceProfileARNs      []arn.ARN
	s3BucketARNs             []arn.ARN
	instanceSecurityGroupIDs []string
	dbHost                   string
	dbPort                   int
	dbName                   string
	dbUser                   string
	dbPassword               string
	dbSSL                    string
}

func configFromContext(c *cli.Context) (*config, error) {
	conf := config{
		verbose:                  c.Bool("verbose"),
		nodeagent:                c.String("nodeagent"),
		regions:                  c.StringSlice("regions"),
		versions:                 c.StringSlice("versions"),
		nodesPerVersion:          c.Int("nodes-per-version"),
		times:                    c.Int("times"),
		settleShort:              c.Duration("settle-short"),
		settleLong:               c.Duration("settle-long"),
		subnetIDs:                c.StringSlice("public-subnet-ids"),
		instanceType:             c.String("instance-type"),
		instanceProfileARNs:      []arn.ARN{},
		s3BucketARNs:             []arn.ARN{},
		instanceSecurityGroupIDs: c.StringSlice("instance-security-group-ids"),
		dbHost:                   c.String("db-host"),
		dbPort:                   c.Int("db-port"),
		dbName:                   c.String("db-name"),
		dbUser:                   c.String("db-user"),
		dbPassword:               c.String("db-password"),
		dbSSL:                    c.String("db-sslmode"),
	}

	if conf.settleShort > conf.settleLong {
		return nil, fmt.Errorf("settle short %s cannot be longer than settle long %s", conf.settleShort, conf.settleLong)
	}

	for _, arnStr := range c.StringSlice("instance-profile-arns") {
		iparn, err := arn.Parse(arnStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing instnace profile arn: %w", err)
		}
		conf.instanceProfileARNs = append(conf.instanceProfileARNs, iparn)
	}

	for _, arnStr := range c.StringSlice("s3-bucket-arns") {
		s3arn, err := arn.Parse(arnStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing s3 bucket arn: %w", err)
		}
		conf.s3BucketARNs = append(conf.s3BucketARNs, s3arn)
	}

	websites := c.StringSlice("websites")
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(websites), func(i, j int) {
		websites[i], websites[j] = websites[j], websites[i]
	})

	conf.websites = websites

	return &conf, nil
}
