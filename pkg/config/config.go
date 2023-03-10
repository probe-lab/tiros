package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/urfave/cli/v2"
)

type GlobalConfig struct {
	Version       string
	TelemetryHost string
	TelemetryPort int
	Debug         bool
	LogLevel      int
}

var DefaultGlobalConfig = GlobalConfig{
	TelemetryHost: "0.0.0.0",
	TelemetryPort: 6666,
	Debug:         false,
	LogLevel:      4,
}

func (gc GlobalConfig) Apply(c *cli.Context) GlobalConfig {
	newConfig := gc

	newConfig.Version = c.App.Version

	if c.IsSet("debug") {
		newConfig.Debug = c.Bool("debug")
	}
	if c.IsSet("log-level") {
		newConfig.LogLevel = c.Int("log-level")
	}
	if c.IsSet("telemetry-host") {
		newConfig.TelemetryHost = c.String("telemetry-host")
	}
	if c.IsSet("telemetry-port") {
		newConfig.TelemetryPort = c.Int("telemetry-port")
	}

	return newConfig
}

func (gc GlobalConfig) String() string {
	str, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return err.Error()
	}

	return string(str)
}

type RunConfig struct {
	GlobalConfig
	NodeAgent        string
	Regions          []string
	Websites         []string
	Versions         []string
	NodesPerVersion  int
	Times            int
	SettleShort      time.Duration
	SettleLong       time.Duration
	DatabaseHost     string
	DatabasePort     int
	DatabaseName     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseSSLMode  string
	InstanceType     string
}

func (rc RunConfig) String() string {
	redacted := rc
	redacted.DatabasePassword = "*****"

	str, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err.Error()
	}

	return string(str)
}

var DefaultRunConfig = RunConfig{
	GlobalConfig:     DefaultGlobalConfig,
	NodeAgent:        "/home/tiros/nodeagent", // correct if you use the default docker image
	Regions:          []string{},
	Websites:         []string{"protocol.ai"},
	Versions:         []string{"v0.18.0"},
	NodesPerVersion:  1,
	Times:            3,
	SettleShort:      10 * time.Second,
	SettleLong:       20 * time.Minute,
	DatabaseHost:     "localhost",
	DatabasePort:     5432,
	DatabaseName:     "tiros",
	DatabaseUser:     "tiros",
	DatabasePassword: "password",
	DatabaseSSLMode:  "disable",
	InstanceType:     "local",
}

func (rc RunConfig) Apply(c *cli.Context) RunConfig {
	newConfig := rc

	newConfig.GlobalConfig = newConfig.GlobalConfig.Apply(c)

	if c.IsSet("versions") {
		newConfig.Versions = c.StringSlice("versions")
	}
	if c.IsSet("nodes-per-version") {
		newConfig.NodesPerVersion = c.Int("nodes-per-version")
	}
	if c.IsSet("regions") {
		newConfig.Regions = c.StringSlice("regions")
	}
	if c.IsSet("settle-short") {
		newConfig.SettleShort = c.Duration("settle-short")
	}
	if c.IsSet("settle-long") {
		newConfig.SettleLong = c.Duration("settle-long")
	}
	if c.IsSet("websites") {
		newConfig.Websites = c.StringSlice("websites")
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(newConfig.Websites), func(i, j int) {
			newConfig.Websites[i], newConfig.Websites[j] = newConfig.Websites[j], newConfig.Websites[i]
		})
	}
	if c.IsSet("times") {
		newConfig.Times = c.Int("times")
	}
	if c.IsSet("nodeagent") {
		newConfig.NodeAgent = c.String("nodeagent")
	}
	if c.IsSet("db-host") {
		newConfig.DatabaseHost = c.String("db-host")
	}
	if c.IsSet("db-port") {
		newConfig.DatabasePort = c.Int("db-port")
	}
	if c.IsSet("db-name") {
		newConfig.DatabaseName = c.String("db-name")
	}
	if c.IsSet("db-password") {
		newConfig.DatabasePassword = c.String("db-password")
	}
	if c.IsSet("db-user") {
		newConfig.DatabaseUser = c.String("db-user")
	}
	if c.IsSet("db-sslmode") {
		newConfig.DatabaseSSLMode = c.String("db-sslmode")
	}
	if c.IsSet("instance-type") {
		newConfig.InstanceType = c.String("instance-type")
	}

	return newConfig
}

func ARNsToStrings(arns []arn.ARN) []string {
	s := make([]string, len(arns))
	for i, a := range arns {
		s[i] = a.String()
	}
	return s
}

type RunAWSConfig struct {
	RunConfig

	SubnetIDs                []string
	InstanceProfileARNs      []arn.ARN
	S3BucketARNs             []arn.ARN
	InstanceSecurityGroupIDs []string
	KeyName                  string
}

func (rawsc RunAWSConfig) String() string {
	str, err := json.MarshalIndent(rawsc, "", "  ")
	if err != nil {
		return err.Error()
	}

	return string(str)
}

var DefaultRunAWSConfig = RunAWSConfig{
	RunConfig: DefaultRunConfig,

	InstanceProfileARNs:      nil,
	S3BucketARNs:             nil,
	InstanceSecurityGroupIDs: nil,
	KeyName:                  "",
}

func (rdc RunAWSConfig) Apply(c *cli.Context) (RunAWSConfig, error) {
	newConfig := rdc

	newConfig.RunConfig = newConfig.RunConfig.Apply(c)

	if c.IsSet("public-subnet-ids") {
		newConfig.SubnetIDs = c.StringSlice("public-subnet-ids")
	}

	if c.IsSet("instance-profile-arns") {
		for _, arnStr := range c.StringSlice("instance-profile-arns") {
			iparn, err := arn.Parse(arnStr)
			if err != nil {
				return RunAWSConfig{}, fmt.Errorf("error parsing instnace profile arn: %w", err)
			}
			newConfig.InstanceProfileARNs = append(newConfig.InstanceProfileARNs, iparn)
		}
	}

	if c.IsSet("s3-bucket-arns") {
		for _, arnStr := range c.StringSlice("s3-bucket-arns") {
			s3arn, err := arn.Parse(arnStr)
			if err != nil {
				return RunAWSConfig{}, fmt.Errorf("error parsing s3 bucket arn: %w", err)
			}
			newConfig.S3BucketARNs = append(newConfig.S3BucketARNs, s3arn)
		}
	}

	if c.IsSet("instance-security-group-ids") {
		newConfig.InstanceSecurityGroupIDs = c.StringSlice("instance-security-group-ids")
	}

	if c.IsSet("key-name") {
		newConfig.KeyName = c.String("key-name")
	}

	return newConfig, nil
}

type RunLocalConfig struct {
	RunConfig
	Nodes int
}

var DefaultRunLocalConfig = RunLocalConfig{
	RunConfig: DefaultRunConfig,
	Nodes:     2,
}

func (rdc RunLocalConfig) String() string {
	str, err := json.MarshalIndent(rdc, "", "  ")
	if err != nil {
		return err.Error()
	}

	return string(str)
}

func (rdc RunLocalConfig) Apply(c *cli.Context) (RunLocalConfig, error) {
	newConfig := rdc

	newConfig.RunConfig = newConfig.RunConfig.Apply(c)

	if c.IsSet("nodes") {
		newConfig.Nodes = c.Int("nodes")
	}

	return newConfig, nil
}
