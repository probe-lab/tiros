package main

import (
	"context"
	"fmt"

	kuboclient "github.com/ipfs/kubo/client/rpc"
	"github.com/multiformats/go-multiaddr"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var probeConfig = struct {
	DryRun bool
	DB     struct {
		Host     string
		Port     int
		Name     string
		Password string
		User     string
		SSLMode  string
	}
	IPFS struct {
		Host           string
		APIPort        int
		GatewayPort    int
		Implementation string
	}
	Chrome struct {
		CDPHost string
		CDPPort int
	}
}{
	DryRun: false,
	DB: struct {
		Host     string
		Port     int
		Name     string
		Password string
		User     string
		SSLMode  string
	}{
		Host:     "localhost",
		Port:     5432,
		Name:     "tiros",
		User:     "tiros",
		SSLMode:  "disable",
		Password: "",
	},
	IPFS: struct {
		Host           string
		APIPort        int
		GatewayPort    int
		Implementation string
	}{
		Host:           "127.0.0.1",
		APIPort:        5001,
		GatewayPort:    8080,
		Implementation: "KUBO",
	},
	Chrome: struct {
		CDPHost string
		CDPPort int
	}{
		CDPHost: "localhost",
		CDPPort: 3000,
	},
}

var probeCmd = &cli.Command{
	Name: "probe",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "dry-run",
			Usage:       "Whether to skip DB interactions",
			EnvVars:     []string{"TIROS_PROBE_DRY_RUN"},
			Value:       probeConfig.DryRun,
			Destination: &probeConfig.DryRun,
		},
		&cli.StringFlag{
			Name:        "db-host",
			Usage:       "On which host address can this clustertest reach the database",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_HOST"},
			Value:       probeConfig.DB.Host,
			Destination: &probeConfig.DB.Host,
		},
		&cli.IntFlag{
			Name:        "db-port",
			Usage:       "On which port can this clustertest reach the database",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_PORT"},
			Value:       probeConfig.DB.Port,
			Destination: &probeConfig.DB.Port,
		},
		&cli.StringFlag{
			Name:        "db-name",
			Usage:       "The name of the database to use",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_NAME"},
			Value:       probeConfig.DB.Name,
			Destination: &probeConfig.DB.Name,
		},
		&cli.StringFlag{
			Name:        "db-password",
			Usage:       "The password for the database to use",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_PASSWORD"},
			Value:       probeConfig.DB.Password,
			Destination: &probeConfig.DB.Password,
		},
		&cli.StringFlag{
			Name:        "db-user",
			Usage:       "The user with which to access the database to use",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_USER"},
			Value:       probeConfig.DB.User,
			Destination: &probeConfig.DB.User,
		},
		&cli.StringFlag{
			Name:        "db-sslmode",
			Usage:       "The sslmode to use when connecting the the database",
			EnvVars:     []string{"TIROS_PROBE_DATABASE_SSL_MODE"},
			Value:       probeConfig.DB.SSLMode,
			Destination: &probeConfig.DB.SSLMode,
		},
		&cli.StringFlag{
			Name:        "ipfs-host",
			Usage:       "host at which to reach the IPFS Gateway",
			EnvVars:     []string{"TIROS_PROBE_IPFS_HOST"},
			Value:       probeConfig.IPFS.Host,
			Destination: &probeConfig.IPFS.Host,
		},
		&cli.IntFlag{
			Name:        "ipfs-api-port",
			Usage:       "port to reach a Kubo-compatible RPC API",
			EnvVars:     []string{"TIROS_PROBE_IPFS_API_PORT"},
			Value:       probeConfig.IPFS.APIPort,
			Destination: &probeConfig.IPFS.APIPort,
		},
		&cli.IntFlag{
			Name:        "ipfs-gateway-port",
			Usage:       "port to reach the IPFS Gateway",
			EnvVars:     []string{"TIROS_PROBE_IPFS_GATEWAY_PORT"},
			Value:       probeConfig.IPFS.GatewayPort,
			Destination: &probeConfig.IPFS.GatewayPort,
		},
		&cli.StringFlag{
			Name:        "chrome-cdp-host",
			Usage:       "host at which the Chrome DevTools Protocol is reachable",
			EnvVars:     []string{"TIROS_PROBE_CHROME_CDP_HOST"},
			Value:       probeConfig.Chrome.CDPHost,
			Destination: &probeConfig.Chrome.CDPHost,
		},
		&cli.IntFlag{
			Name:        "chrome-cdp-port",
			Usage:       "port to reach the Chrome DevTools Protocol port",
			EnvVars:     []string{"TIROS_PROBE_CHROME_CDP_PORT"},
			Value:       probeConfig.Chrome.CDPPort,
			Destination: &probeConfig.Chrome.CDPPort,
		},
		&cli.StringFlag{
			Name:        "ipfs-implementation",
			Usage:       "Which implementation are we testing (KUBO, HELIA)",
			EnvVars:     []string{"TIROS_PROBE_IPFS_IMPLEMENTATION"},
			Value:       probeConfig.IPFS.Implementation,
			Destination: &probeConfig.IPFS.Implementation,
		},
	},
	Subcommands: []*cli.Command{
		probeWebsitesCmd,
		probeUploadCmd,
	},
}

func newKuboClient() (*kuboclient.HttpApi, error) {
	apiMaddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", probeConfig.IPFS.Host, probeConfig.IPFS.APIPort))
	if err != nil {
		return nil, err
	}

	return kuboclient.NewApiWithClient(apiMaddr, otelhttp.DefaultClient)
}

func newDBClient(ctx context.Context) (IDBClient, error) {
	// Initialize database client
	var err error
	var dbClient IDBClient = DBDummyClient{}
	if !probeConfig.DryRun {
		dbClient, err = InitClient(ctx,
			probeConfig.DB.Host,
			probeConfig.DB.Port,
			probeConfig.DB.Name,
			probeConfig.DB.User,
			probeConfig.DB.Password,
			probeConfig.DB.SSLMode,
		)
		if err != nil {
			return nil, fmt.Errorf("init db connection: %w", err)
		}
	}
	return dbClient, nil
}
