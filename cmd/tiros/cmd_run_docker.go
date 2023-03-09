package main

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/guseggert/clustertest/cluster"

	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster/basic"

	"github.com/guseggert/clustertest/cluster/docker"
	log "github.com/sirupsen/logrus"

	"github.com/dennis-tra/tiros/pkg/config"

	"github.com/urfave/cli/v2"
)

var RunDockerCommand = &cli.Command{
	Name: "docker",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:        "nodes",
			Usage:       "The number of nodes to spawn",
			EnvVars:     []string{"PARSEC_SCHEDULE_DOCKER_NODES"},
			DefaultText: strconv.Itoa(config.DefaultRunDockerConfig.Nodes),
			Value:       config.DefaultRunDockerConfig.Nodes,
		},
	},
	Action: RunDockerAction,
}

func RunDockerAction(c *cli.Context) error {
	log.Infoln("Starting Tiros docker run...")

	conf, err := config.DefaultRunDockerConfig.Apply(c)
	if err != nil {
		return err
	}

	_ = conf
	clus, err := docker.NewCluster()
	if err != nil {
		return fmt.Errorf("new docker cluster: %w", err)
	}

	kc := kubo.New(basic.New(clus))

	nodes, err := kc.NewNodes(1)
	if err != nil {
		return err
	}

	n := nodes[0]

	if err := n.LoadBinary(); err != nil {
		return fmt.Errorf("loading kubo binary: %w", err)
	}

	if err := n.Init(); err != nil {
		return fmt.Errorf("initializing kubo: %w", err)
	}

	if err := n.ConfigureForRemote(); err != nil {
		return fmt.Errorf("configuring kubo: %w", err)
	}

	if _, err := n.Context(c.Context).StartDaemonAndWaitForAPI(); err != nil {
		return fmt.Errorf("waiting for kubo to startup: %w", err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	_, err := n.Run(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"run",
			"--network=host",
			"--privileged",
			"-p", "3000:3000",
			"browserless/chrome",
		},
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: stdout: %s, stderr: %s", err, stdout, stderr)
	}

	//tc := tiros.NewCluster(basic.New(cl).Context(c.Context), "docker", "container")
	//
	//log.Infoln("Initializing docker nodes")
	//nodes, err := pc.NewNodes(conf.Nodes)
	//if err != nil {
	//	return fmt.Errorf("new docker nodes: %w", err)
	//}
	//return RunAction(c, nodes)
}
