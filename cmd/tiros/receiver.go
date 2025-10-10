package main

import (
	"context"
	"sync"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	plgrpc "github.com/probe-lab/go-commons/grpc"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
)

// TraceReceiver implements the OTLP gRPC service
type TraceReceiver struct {
	coltracepb.UnimplementedTraceServiceServer
	server *plgrpc.Server

	matchersMu     sync.RWMutex
	traceMatchers  []TraceMatcher
	traceMatchChan chan *TraceMatch
}

type TraceMatcher func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool

type TraceMatch struct {
	matcherIdx int
	req        *coltracepb.ExportTraceServiceRequest
}

func NewTraceReceiver(host string, port int) (*TraceReceiver, error) {
	server, err := plgrpc.NewServer(&plgrpc.ServerConfig{
		Host: host,
		Port: port,
		LogOpts: []logging.Option{
			logging.WithLevels(func(code codes.Code) logging.Level {
				if code == codes.OK {
					return logging.LevelDebug
				}
				return logging.LevelWarn
			}),
		},
	})
	if err != nil {
		return nil, err
	}

	tr := &TraceReceiver{
		server:         server,
		traceMatchChan: make(chan *TraceMatch),
		traceMatchers:  make([]TraceMatcher, 0),
	}

	coltracepb.RegisterTraceServiceServer(server, tr)

	return tr, nil
}

func (tr *TraceReceiver) Reset() {
	tr.matchersMu.Lock()
	defer tr.matchersMu.Unlock()
	tr.traceMatchers = make([]TraceMatcher, 0)
}

func (tr *TraceReceiver) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	tr.matchersMu.RLock()
	defer tr.matchersMu.RUnlock()

	resp := &coltracepb.ExportTraceServiceResponse{
		PartialSuccess: &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: 0,
			ErrorMessage:  "",
		},
	}

	if len(tr.traceMatchers) == 0 {
		return resp, nil
	}

	for _, rspan := range req.GetResourceSpans() {
		for _, sspan := range rspan.GetScopeSpans() {
			for _, span := range sspan.GetSpans() {
				for i, matcher := range tr.traceMatchers {
					matched := matcher(rspan, sspan, span)
					if matched {
						tr.traceMatchChan <- &TraceMatch{matcherIdx: i, req: req}
						return resp, nil
					}
				}
			}
		}
	}

	return resp, nil
}
