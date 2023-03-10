package tiros

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster/basic"
	"golang.org/x/sync/errgroup"
)

type Cluster struct {
	*basic.Cluster
	Region          string
	Versions        []string
	NodesPerVersion int
}

func NewCluster(bc *basic.Cluster, region string, versions []string, nodesPerVersion int) *Cluster {
	log.WithFields(log.Fields{
		"region":          region,
		"versions":        versions,
		"nodesPerVersion": nodesPerVersion,
	}).Infoln("Init new cluster")
	return &Cluster{
		Cluster:         bc,
		Region:          region,
		Versions:        versions,
		NodesPerVersion: nodesPerVersion,
	}
}

func (c *Cluster) NewNodes() ([]*Node, error) {
	kc := kubo.New(c.Cluster).Context(c.Ctx)

	log.WithFields(log.Fields{
		"region":          c.Region,
		"versions":        c.Versions,
		"nodesPerVersion": c.NodesPerVersion,
	}).Infoln("Starting new nodes..")

	knodes, err := kc.NewNodes(len(c.Versions) * c.NodesPerVersion)
	if err != nil {
		return nil, fmt.Errorf("new kubo nodes: %w", err)
	}

	tnodes := make([]*Node, len(c.Versions)*c.NodesPerVersion)
	for i, version := range c.Versions {
		for j := 0; j < c.NodesPerVersion; j++ {
			idx := i*c.NodesPerVersion + j
			knode := knodes[idx].WithKuboVersion(version)
			tnodes[idx] = NewNode(c, knode, j)
		}
	}

	errg := errgroup.Group{}
	for _, tnode := range tnodes {
		tnode := tnode
		errg.Go(func() error {
			return tnode.initialize()
		})
	}
	if err = errg.Wait(); err != nil {
		return nil, fmt.Errorf("init tiros node: %w", err)
	}

	return tnodes, nil
}
