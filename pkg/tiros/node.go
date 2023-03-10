package tiros

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/friendsofgo/errors"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster"
	"github.com/guseggert/clustertest/cluster/basic"
	"github.com/ipfs/kubo/config"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"golang.org/x/sync/errgroup"

	"github.com/dennis-tra/tiros/pkg/models"
)

const websiteRequestTimeout = 20 * time.Second

type Node struct {
	*kubo.Node
	Cluster *Cluster
	NodeNum int

	chromeOnlineSince time.Time
	chromeProc        *basic.Process
}

func NewNode(c *Cluster, n *kubo.Node, nodeNum int) *Node {
	return &Node{
		Node:    n,
		Cluster: c,
		NodeNum: nodeNum,
	}
}

func (n *Node) logEntry() *log.Entry {
	return log.WithFields(log.Fields{
		"region":  n.Cluster.Region,
		"num":     n.NodeNum,
		"version": n.MustVersion(),
	})
}

func (n *Node) initialize() error {
	n.logEntry().Infoln("Init Tiros node...")
	defer n.logEntry().Infoln("Init Tiros node done!")

	errg, errCtx := errgroup.WithContext(n.Ctx)
	errg.Go(func() error {
		return n.initKubo(errCtx)
	})
	errg.Go(func() error {
		return n.initChrome(errCtx)
	})
	return errg.Wait()
}

func (n *Node) StopChrome() error {
	n.logEntry().Infoln("Sending SIGINT to Chrome")

	if n.chromeProc != nil {
		return n.chromeProc.Signal(syscall.SIGINT)
	}
	return nil
}

func (n *Node) initKubo(ctx context.Context) error {
	n.logEntry().Infoln("  ...init Kubo...")
	defer n.logEntry().Infoln("  ...init Kubo done!")

	kn := n.Node.Context(ctx)

	if err := kn.LoadBinary(); err != nil {
		return fmt.Errorf("loading kubo binary: %w", err)
	}

	if err := kn.Init(); err != nil {
		return fmt.Errorf("initializing kubo: %w", err)
	}

	if err := kn.ConfigureForRemote(); err != nil {
		return fmt.Errorf("configuring kubo: %w", err)
	}

	// Disable resource manager
	err := kn.UpdateConfig(func(cfg *config.Config) {
		// comment in as soon as we only test kubo >= 0.19.0
		// cfg.Routing.Type = "autoclient"
		cfg.Swarm.ResourceMgr.Enabled = config.False
	})
	if err != nil {
		return fmt.Errorf("disable resource manager: %w", err)
	}

	// don't use above kn context
	if _, err := n.Node.StartDaemonAndWaitForAPI(); err != nil {
		return fmt.Errorf("waiting for kubo to startup: %w", err)
	}

	return nil
}

// initChrome does the following:
//   - pull the browserless/chrome image
//   - run the browserless/chrome image
//   - init a tunnel through nodeagent to websocket endpoint of chrome instance
//   - init rod to connect to chrome from local node
func (n *Node) initChrome(ctx context.Context) error {
	n.logEntry().Infoln("  ...pulling Chrome...")
	defer n.logEntry().Infoln("  ...init Chrome done!")

	nodeLogger := log.New().WithFields(n.logEntry().Data)

	_, err := n.Run(cluster.StartProcRequest{
		Command: "docker",
		Args:    []string{"pull", "-q", "browserless/chrome"},
		Stdout:  nodeLogger.WithField("cmd", "docker pull").Writer(),
		Stderr:  nodeLogger.WithField("cmd", "docker pull").Writer(),
	})
	if err != nil {
		return fmt.Errorf("error starting headless chrome: %s", err)
	}

	n.logEntry().Infoln("  ...running Chrome...")
	n.chromeProc, err = n.StartProc(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"run",
			"--network=host",
			"--privileged",
			"-p", "3000:3000",
			"browserless/chrome",
		},
		// Stdout: nodeLogger.WithField("cmd", "docker run").Writer(),
		// Stderr: nodeLogger.WithField("cmd", "docker run").Writer(),
	})
	if err != nil {
		return fmt.Errorf("error starting headless chrome: %s", err)
	}

	go func() {
		n.logEntry().Infoln("  ...waiting for chrome...")
		defer n.logEntry().Infoln("Chrome stopped.")

		if ex, err := n.chromeProc.Context(n.Ctx).Wait(); err != nil && n.Ctx.Err() != context.Canceled {
			n.logEntry().WithError(err).WithField("exitCode", ex.ExitCode).Fatal("Headless chrome process stopped abnormally.")
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	timeout := time.NewTimer(3 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return context.DeadlineExceeded
		case <-ticker.C:
		}

		n.logEntry().Debugln("Checking chrome availability...")
		if _, err := n.initBrowser(); err != nil {
			n.logEntry().WithError(err).Debugln("err init rod browser")
			ticker.Reset(5 * time.Second)
			continue
		}

		return nil
	}
}

func (n *Node) initBrowser() (*rod.Browser, error) {
	// Tunnel websocket through nodeagent
	rodDialer := ws.DefaultDialer
	rodDialer.NetDial = n.Node.Node.Node.Dial
	conn, _, _, err := rodDialer.Dial(n.Ctx, "ws://127.0.0.1:3000")
	if err != nil {
		return nil, fmt.Errorf("dialing chrome instance: %w", err)
	}

	// Try to connect to chrome instance
	client := cdp.New().Start(&WebSocket{conn})
	browser := rod.New().Context(n.Ctx).Client(client)
	if err = browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect to chrome: %w", err)
	}

	return browser, nil
}

// WebSocket is a custom websocket that uses gobwas/ws as the transport layer.
type WebSocket struct {
	conn net.Conn
}

// Send ...
func (w *WebSocket) Send(b []byte) error {
	return wsutil.WriteClientText(w.conn, b)
}

// Read ...
func (w *WebSocket) Read() ([]byte, error) {
	return wsutil.ReadServerText(w.conn)
}

type ProbeResult struct {
	URL                    string
	TimeToFirstByte        *float64
	FirstContentFulPaint   *float64
	LargestContentfulPaint *float64
	NavigationPerformance  *PerformanceNavigationEntry
	Error                  error `json:"-"`
}

func (pr *ProbeResult) NullJSON() (null.JSON, error) {
	data, err := json.Marshal(pr)
	if err != nil {
		return null.NewJSON(nil, false), err
	}
	return null.JSONFrom(data), nil
}

func (pr *ProbeResult) NullError() null.String {
	if pr.Error != nil {
		return null.StringFrom(pr.Error.Error())
	}
	return null.NewString("", false)
}

func (n *Node) probe(ctx context.Context, website string, mType string) (*ProbeResult, error) {
	url, err := n.websiteURL(website, mType)
	if err != nil {
		return nil, fmt.Errorf("website url: %w", err)
	}

	n.logEntry().WithField("type", mType).Infoln("Probing", url)

	pr := ProbeResult{
		URL: url,
	}

	browser, err := n.initBrowser()
	if err != nil {
		return nil, fmt.Errorf("init browser: %w", err)
	}

	var perfEntriesStr string
	err = rod.Try(func() {
		page := browser.
			MustIncognito(). // prevents cache usage
			MustPage().
			Context(ctx).                   // stop when outer ctx stops
			Timeout(websiteRequestTimeout). // only wait 30s
			MustNavigate(url).
			MustWaitIdle(). // wait until network has come to a halt (1 Minute)
			CancelTimeout() // continue using page object for things

		perfEntriesStr = page.MustEval(jsPerformanceEntries).Str()

		page.MustClose()
		browser.MustClose()
	})
	if errors.Is(err, context.DeadlineExceeded) {
		pr.Error = fmt.Errorf("timed out after %s", websiteRequestTimeout)
		return &pr, nil
	} else if err != nil {
		pr.Error = err
		return &pr, nil
	}

	perfEntries, err := unmarshalPerformanceEntries([]byte(perfEntriesStr))
	if err != nil {
		fmt.Println(perfEntriesStr)
		return nil, fmt.Errorf("parse performance entries: %w", err)
	}

	// https://developer.mozilla.org/en-US/docs/Learn/Performance/Perceived_performance#performance_metrics
	for _, e := range perfEntries {
		switch e.EntryType {
		case "resource":
		case "navigation":
			pne, err := e.NavigationEntry()
			if err != nil {
				return nil, fmt.Errorf("parse navigation entry: %w", err)
			}

			// https://developer.mozilla.org/en-US/docs/Web/Performance/Navigation_and_resource_timings#time_to_first_byte
			ttfb := pne.ResponseStart - pne.StartTime
			pr.TimeToFirstByte = &ttfb
			pr.NavigationPerformance = pne

		case "paint":
			// https://web.dev/fcp/
			if pr.FirstContentFulPaint == nil || e.StartTime < *pr.FirstContentFulPaint {
				pr.FirstContentFulPaint = &e.StartTime
			}

		case "largest-contentful-paint":
			// https://web.dev/lcp/
			if pr.LargestContentfulPaint == nil || e.StartTime > *pr.LargestContentfulPaint {
				pr.LargestContentfulPaint = &e.StartTime
			}
		}
	}

	return &pr, nil
}

func (n *Node) websiteURL(website string, mType string) (string, error) {
	switch mType {
	case models.MeasurementTypeKUBO:
		gatewayURL, err := n.GatewayURL()
		if err != nil {
			return "", fmt.Errorf("getting gateway url: %w", err)
		}

		return fmt.Sprintf("%s/ipns/%s", gatewayURL, website), nil

	case models.MeasurementTypeHTTP:
		return fmt.Sprintf("https://%s", website), nil

	default:
		errMsg := fmt.Sprintf("unknown measurement type: %s", mType)
		n.logEntry().Errorln(errMsg)

		return "", fmt.Errorf(errMsg)
	}
}
