package tiros

import (
	"context"
	"fmt"

	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster/basic"
	"golang.org/x/sync/errgroup"
)

type Cluster struct {
	*basic.Cluster
	Region string
}

func NewCluster(bc *basic.Cluster, region string) *Cluster {
	return &Cluster{
		Cluster: bc,
		Region:  region,
	}
}

//func (c *Cluster) NewNodes(n int) ([]*Node, error) {
//	clusterNodes, err := c.Cluster.NewNodes(n)
//	if err != nil {
//		return nil, err
//	}
//
//	tirosNodes := make([]*Node, len(clusterNodes))
//	for i, cn := range clusterNodes {
//		n, err := NewNode(c, cn.Context(c.Ctx), fmt.Sprintf("node-%d", i))
//		if err != nil {
//			return nil, fmt.Errorf("new tiros node: %w", err)
//		}
//		tirosNodes[i] = n
//	}
//
//	return tirosNodes, nil
//}

func (c *Cluster) NewNode(ctx context.Context, version string) (*Node, error) {
	cn, err := c.Cluster.NewNode()
	if err != nil {
		return nil, fmt.Errorf("new cluster node: %w", err)
	}

	errg, errCtx := errgroup.WithContext(ctx)
	errg.Go(func() error {
		kc := kubo.New(c.Cluster)

		nodes, err := kc.NewNodes(1)
		if err != nil {
			return err
		}
		n := nodes[0]

		n.WithKuboVersion(version)

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
		return nil
	})

	go func() {
	}()

	return n, err
}

//func NewNode(clus *Cluster) (*Node, error) {
//}
