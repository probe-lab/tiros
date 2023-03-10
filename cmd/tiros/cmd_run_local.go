package main

import (
	"fmt"

	"github.com/guseggert/clustertest/cluster/basic"
	"github.com/guseggert/clustertest/cluster/local"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/dennis-tra/tiros/pkg/config"
	"github.com/dennis-tra/tiros/pkg/tiros"
)

var RunLocalCommand = &cli.Command{
	Name:   "local",
	Action: RunLocalAction,
}

func RunLocalAction(c *cli.Context) error {
	log.Infoln("Starting Tiros local scheduler...")

	conf, err := config.DefaultRunLocalConfig.Apply(c)
	if err != nil {
		return fmt.Errorf("parsing aws config: %w", err)
	}

	log.Infoln("Configuration:")
	fmt.Println(conf.String())

	// starting cluster in all regions
	exp := tiros.NewExperiment(conf.RunConfig)

	exp.Cluster["local"] = basic.New(local.NewCluster()).Context(c.Context)

	return RunAction(c, exp)
}
