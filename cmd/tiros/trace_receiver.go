package main

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	plgrpc "github.com/probe-lab/go-commons/grpc"
	"go.opentelemetry.io/otel/trace"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// TraceReceiver implements the OTLP gRPC service
type TraceReceiver struct {
	coltracepb.UnimplementedTraceServiceServer
	server *plgrpc.Server

	mu sync.Mutex

	traceMatchers  []TraceMatcher
	traceMatchChan chan *ExportTraceServiceRequest
	forwardClient  coltracepb.TraceServiceClient
	traceOut       string
	traceCounter   int
}

type TraceMatcher func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool

func NewTraceReceiver(host string, port int, traceOut string) (*TraceReceiver, error) {
	server, err := plgrpc.NewServer(&plgrpc.ServerConfig{
		Host: host,
		Port: port,
		LogOpts: []logging.Option{
			logging.WithLevels(func(code codes.Code) logging.Level {
				return logging.LevelInfo
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

	if traceOut != "" {
		if err := os.MkdirAll(traceOut, 0o755); err != nil {
			return nil, fmt.Errorf("create trace directory: %w", err)
		}
	}

	tr := &TraceReceiver{
		server:         server,
		traceMatchChan: make(chan *ExportTraceServiceRequest),
		traceMatchers:  make([]TraceMatcher, 0),
		traceOut:       traceOut,
		traceCounter:   0,
	}

	coltracepb.RegisterTraceServiceServer(server, tr)

	conn, err := grpc.Dial(
		"localhost:55680",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to downstream: %w", err)
	}

	tr.forwardClient = coltracepb.NewTraceServiceClient(conn)

	return tr, nil
}

func (tr *TraceReceiver) Shutdown() {
	close(tr.traceMatchChan)
	tr.server.Shutdown()
}

func (tr *TraceReceiver) Reset() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.traceMatchers = make([]TraceMatcher, 0)
}

type ExportTraceServiceRequest struct {
	*coltracepb.ExportTraceServiceRequest
}

func (t *ExportTraceServiceRequest) Spans() iter.Seq[*v1.Span] {
	return iter.Seq[*v1.Span](func(yield func(span *v1.Span) bool) {
		for _, rspan := range t.GetResourceSpans() {
			for _, sspan := range rspan.GetScopeSpans() {
				for _, span := range sspan.GetSpans() {
					yield(span)
				}
			}
		}
	})
}

func (tr *TraceReceiver) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.traceOut != "" {
		protoName := "./" + path.Join(tr.traceOut, fmt.Sprintf("trace-%d.proto.json", tr.traceCounter))

		marshaler := protojson.MarshalOptions{Indent: "  "}
		if protoData, err := marshaler.Marshal(req); err != nil {
			slog.Warn("failed to marshal trace", "err", err)
		} else if err := os.WriteFile(protoName, protoData, 0o644); err != nil {
			slog.Warn("failed to write trace", "err", err)
		}
		tr.traceCounter++
	}

	resp := &coltracepb.ExportTraceServiceResponse{
		PartialSuccess: &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: 0,
			ErrorMessage:  "",
		},
	}
	if _, err := tr.forwardClient.Export(ctx, req); err != nil {
		slog.Warn(err.Error())
	}

	if len(tr.traceMatchers) == 0 {
		return resp, nil
	}

	for _, rspan := range req.GetResourceSpans() {
		for _, sspan := range rspan.GetScopeSpans() {
			for _, span := range sspan.GetSpans() {
				for _, matcher := range tr.traceMatchers {
					matched := matcher(rspan, sspan, span)
					if matched {
						tr.traceMatchChan <- &ExportTraceServiceRequest{req}
						return resp, nil
					}
				}
			}
		}
	}

	return resp, nil
}

func traceIDMatcher(traceID trace.TraceID) TraceMatcher {
	return func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool {
		return bytes.Equal(span.TraceId, traceID[:])
	}
}

func strAttrMatcher(k, v string) TraceMatcher {
	return func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool {
		for _, a := range span.Attributes {
			return a.Key == k && a.Value.GetStringValue() == v
		}
		return false
	}
}

func nameMatcher(name string) TraceMatcher {
	return func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool {
		return span.Name == name
	}
}

func nameAttrMatcher(name, k, v string) TraceMatcher {
	return func(rspan *v1.ResourceSpans, sspan *v1.ScopeSpans, span *v1.Span) bool {
		return strAttrMatcher(k, v)(rspan, sspan, span) && nameMatcher(name)(rspan, sspan, span)
	}
}
