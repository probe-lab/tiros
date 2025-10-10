package main

import (
	"math"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type ipfsAddMetrics struct {
	duration time.Duration
	start    time.Time
	end      time.Time
}

func parseIPFSAddTrace(req *coltracepb.ExportTraceServiceRequest) *ipfsAddMetrics {
	var traceStartUnixNano int64 = math.MaxInt64
	var traceEndUnixNano int64 = 0

	addBlockSpans := make([]*v1.Span, 0)
	for _, rspan := range req.GetResourceSpans() {
		for _, sspan := range rspan.GetScopeSpans() {
			for _, span := range sspan.GetSpans() {
				if span.StartTimeUnixNano < math.MaxInt64 && int64(span.StartTimeUnixNano) < traceStartUnixNano {
					traceStartUnixNano = int64(span.StartTimeUnixNano)
				}

				if span.EndTimeUnixNano < math.MaxInt64 && int64(span.EndTimeUnixNano) > traceEndUnixNano {
					traceEndUnixNano = int64(span.EndTimeUnixNano)
				}

				if span.Name == "Blockservice.blockService.AddBlocks" {
					addBlockSpans = append(addBlockSpans, span)
				}
			}
		}
	}

	if len(addBlockSpans) == 0 || traceStartUnixNano == math.MaxInt64 || traceEndUnixNano == 0 {
		return nil
	}

	return &ipfsAddMetrics{
		start:    time.Unix(0, traceStartUnixNano),
		end:      time.Unix(0, traceEndUnixNano),
		duration: time.Duration(traceEndUnixNano - traceStartUnixNano),
	}
}

type provideMetrics struct {
	duration time.Duration
	start    time.Time
	end      time.Time
}

func parseProvideTrace(req *coltracepb.ExportTraceServiceRequest) *provideMetrics {
	var traceStartUnixNano int64 = math.MaxInt64
	var traceEndUnixNano int64 = 0

	for _, rspan := range req.GetResourceSpans() {
		for _, sspan := range rspan.GetScopeSpans() {
			for _, span := range sspan.GetSpans() {
				if span.StartTimeUnixNano < math.MaxInt64 && int64(span.StartTimeUnixNano) < traceStartUnixNano {
					traceStartUnixNano = int64(span.StartTimeUnixNano)
				}

				if span.EndTimeUnixNano < math.MaxInt64 && int64(span.EndTimeUnixNano) > traceEndUnixNano {
					traceEndUnixNano = int64(span.EndTimeUnixNano)
				}

			}
		}
	}

	if traceStartUnixNano == math.MaxInt64 || traceEndUnixNano == 0 {
		return nil
	}

	return &provideMetrics{
		start:    time.Unix(0, traceStartUnixNano),
		end:      time.Unix(0, traceEndUnixNano),
		duration: time.Duration(traceEndUnixNano - traceStartUnixNano),
	}
}
