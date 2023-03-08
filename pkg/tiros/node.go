package tiros

import (
	"context"
	"net/http"

	"github.com/guseggert/clustertest/cluster/basic"
	log "github.com/sirupsen/logrus"
)

type Node struct {
	*basic.Node

	id      string
	ctx     context.Context
	client  http.Client
	host    string
	port    int
	fmt     log.Formatter
	done    chan struct{}
	cluster *Cluster
}

//func NewNode(c *Cluster, n *basic.Node, id string) (*Node, error) {
//	logEntry := log.WithField("nodeID", id)
//
//	kc := kubo.New(basic.New(c))
//
//	parsecBinPath := ""
//	if util.FileExists("parsec") {
//		parsecBinPath = "parsec"
//	} else {
//		execFile, err := os.Executable()
//		if err != nil {
//			return nil, fmt.Errorf("find path to executable: %w", err)
//		}
//		parsecBinPath = execFile
//	}
//
//	parsecBinary, err := os.ReadFile(parsecBinPath)
//	if err != nil {
//		return nil, fmt.Errorf("read parsec binary: %w", err)
//	}
//
//	logEntry.Infoln("Sending parsec binary...")
//	if err = n.SendFile("/parsec", bytes.NewReader(parsecBinary)); err != nil {
//		return nil, fmt.Errorf("send parsec binary: %w", err)
//	}
//
//	logEntry.Infoln("Setting parsec permissions...")
//	proc, err := n.StartProc(cluster.StartProcRequest{
//		Command: "chmod",
//		Args:    []string{"+x", "/parsec"},
//	})
//	if err != nil {
//		return nil, fmt.Errorf("setting parsec permissions: %w", err)
//	}
//	if _, err := proc.Wait(); err != nil {
//		return nil, fmt.Errorf("set parsec permissions: %w", err)
//	}
//
//	newTransport := http.DefaultTransport.(*http.Transport).Clone()
//	newTransport.DialContext = n.Node.Dial
//	httpClient := http.Client{Transport: newTransport}
//
//	logger := log.New()
//	parsecNode := &Node{
//		Node:    n,
//		cluster: c,
//		id:      id,
//		client:  httpClient,
//		host:    host,
//		port:    port,
//		fmt:     logger.Formatter,
//		done:    make(chan struct{}),
//	}
//
//	logger.Formatter = parsecNode
//	nodeLogger := logger.WithField("nodeID", id)
//
//	logEntry.Infoln("Starting parsec server...")
//	proc, err = n.StartProc(cluster.StartProcRequest{
//		Command: "/parsec",
//		Args:    []string{"server"},
//		Stderr:  nodeLogger.Writer(),
//		Stdout:  nodeLogger.Writer(),
//	})
//	if err != nil {
//		return nil, fmt.Errorf("start parsec server: %w", err)
//	}
//
//	go func() {
//		logEntry.Infoln("Waiting parsec server...")
//		procRes, err := proc.Context(n.Ctx).Wait()
//		if procRes != nil {
//			logEntry = logEntry.WithField("exitCode", procRes.ExitCode).WithField("uptime", procRes.TimeMS)
//		}
//		if err != nil {
//			logEntry = logEntry.WithError(err)
//		}
//		logEntry.Infoln("Parsec server exited")
//		close(parsecNode.done)
//	}()
//
//	return parsecNode, nil
//}
//
//func (n *Node) WaitForAPI(ctx context.Context) (*InfoResponse, error) {
//	tick := 5 * time.Second
//	t := time.NewTicker(tick)
//	for {
//		select {
//		case <-t.C:
//			log.WithField("node", n.id).Infoln("Check API availability...")
//			info, err := n.Info(ctx)
//			if err == nil {
//				return info, nil
//			}
//			t.Reset(tick)
//		case <-ctx.Done():
//			return nil, ctx.Err()
//		}
//	}
//}
//
//func (n *Node) ID() string {
//	return n.id
//}
//
//func (n *Node) Cluster() *Cluster {
//	return n.cluster
//}
//
//func (n *Node) Info(ctx context.Context) (*InfoResponse, error) {
//	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d/info", n.host, n.port), nil)
//	if err != nil {
//		return nil, fmt.Errorf("create info request: %w", err)
//	}
//	req = req.WithContext(ctx)
//
//	res, err := n.client.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("get info: %w", err)
//	}
//
//	dat, err := io.ReadAll(res.Body)
//	if err != nil {
//		return nil, fmt.Errorf("read provide response: %w", err)
//	}
//
//	info := InfoResponse{}
//	if err = json.Unmarshal(dat, &info); err != nil {
//		return nil, fmt.Errorf("unmarshal info response: %w", err)
//	}
//
//	return &info, nil
//}
//
//func (n *Node) Retrieve(ctx context.Context, c cid.Cid, count int) (*RetrievalResponse, error) {
//	rr := &RetrieveRequest{}
//
//	data, err := json.Marshal(rr)
//	if err != nil {
//		return nil, fmt.Errorf("marshal retrieval request: %w", err)
//	}
//
//	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%d/retrieve/%s", n.host, n.port, c.String()), bytes.NewReader(data))
//	if err != nil {
//		return nil, fmt.Errorf("create retrieve request: %w", err)
//	}
//	req = req.WithContext(ctx)
//
//	req.Header.Add("Content-Type", "application/json")
//
//	res, err := n.client.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("post retrieval request: %w", err)
//	}
//
//	dat, err := io.ReadAll(res.Body)
//	if err != nil {
//		return nil, fmt.Errorf("read retrieval response: %w", err)
//	}
//
//	retrieval := RetrievalResponse{}
//	if err = json.Unmarshal(dat, &retrieval); err != nil {
//		return nil, fmt.Errorf("unmarshal retrieval response: %w", err)
//	}
//
//	return &retrieval, nil
//}
//
//func (n *Node) Provide(ctx context.Context, c *util.Content) (*ProvideResponse, error) {
//	pr := &ProvideRequest{
//		Content: c.Raw,
//	}
//
//	data, err := json.Marshal(pr)
//	if err != nil {
//		return nil, fmt.Errorf("marshal provide request: %w", err)
//	}
//
//	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%d/provide", n.host, n.port), bytes.NewReader(data))
//	if err != nil {
//		return nil, fmt.Errorf("create retrieve request: %w", err)
//	}
//	req = req.WithContext(ctx)
//
//	req.Header.Add("Content-Type", "application/json")
//
//	res, err := n.client.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("start provide: %w", err)
//	}
//
//	dat, err := io.ReadAll(res.Body)
//	if err != nil {
//		return nil, fmt.Errorf("read provide response: %w", err)
//	}
//
//	provide := ProvideResponse{}
//	if err = json.Unmarshal(dat, &provide); err != nil {
//		return nil, fmt.Errorf("unmarshal provide response: %w", err)
//	}
//
//	return &provide, nil
//}
//
//func (n *Node) Assign(dbNode *models.Node) {
//	n.dbNode = dbNode
//}
//
//func (n *Node) DatabaseID() int {
//	if n.dbNode != nil {
//		return n.dbNode.ID
//	}
//	return 0
//}
//
//func (n *Node) Format(entry *log.Entry) ([]byte, error) {
//	logMsg := map[string]interface{}{}
//	if err := json.Unmarshal([]byte(entry.Message), &logMsg); err != nil {
//		return n.fmt.Format(entry)
//	}
//
//	for k, v := range logMsg {
//		switch k {
//		case "msg":
//			entry.Message = v.(string)
//		case "time":
//			t, err := time.Parse(time.RFC3339Nano, v.(string))
//			if err != nil {
//				return nil, fmt.Errorf("parsing time: %w", err)
//			}
//			entry.Time = t
//		case "level":
//			l, err := log.ParseLevel(logMsg["level"].(string))
//			if err != nil {
//				return nil, fmt.Errorf("parsing level: %w", err)
//			}
//			entry.Level = l
//		default:
//			if k == "error" && v != nil && v.(string) == "<nil>" {
//				continue
//			}
//			entry.Data[k] = v
//		}
//	}
//
//	if !log.IsLevelEnabled(entry.Level) {
//		return nil, nil
//	}
//
//	return n.fmt.Format(entry)
//}
