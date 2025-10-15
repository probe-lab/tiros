package main

import (
	"math"
	"time"

	"github.com/ipfs/go-cid"
	"go.opentelemetry.io/otel/trace"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type UploadResult struct {
	CID            cid.Cid
	RawCID         cid.Cid
	IPFSAddTraceID trace.TraceID
	ProvideTraceID trace.TraceID
	IPFSAddStart   time.Time
	IPFSAddEnd     time.Time
	ProvideStart   time.Time
	ProvideEnd     time.Time

	spansByTraceID map[trace.TraceID][]*v1.Span
}

func (r *UploadResult) parse(req *ExportTraceServiceRequest) {
	for span := range req.Spans() {
		if _, found := r.spansByTraceID[trace.TraceID(span.TraceId)]; found {
			r.spansByTraceID[trace.TraceID(span.TraceId)] = append(r.spansByTraceID[trace.TraceID(span.TraceId)], span)
		} else {
			r.spansByTraceID[trace.TraceID(span.TraceId)] = []*v1.Span{span}
		}

		// if we already have a trace ID for the provide operation, we can skip
		if r.ProvideTraceID.IsValid() {
			continue
		}

		// if the span is not a provide operation, we can skip
		if span.Name != "IpfsDHT.Provide" {
			continue
		}

		// if the span does not have the attribute key set to the raw CID, we can skip
		for _, attr := range span.Attributes {
			if attr.Key == "key" && attr.Value.GetStringValue() == r.RawCID.String() {
				r.ProvideTraceID = trace.TraceID(span.TraceId)
			}
		}
	}

	if spans, found := r.spansByTraceID[r.IPFSAddTraceID]; found {
		start, end := extractTimeRange(spans)
		r.IPFSAddStart = start
		r.IPFSAddEnd = end
	}

	if spans, found := r.spansByTraceID[r.ProvideTraceID]; found {
		start, end := extractTimeRange(spans)
		r.ProvideStart = start
		r.ProvideEnd = end
	}
}

func (r *UploadResult) isPopulated() bool {
	return r.IPFSAddTraceID.IsValid() && r.ProvideTraceID.IsValid()
}

type DownloadResult struct {
	CID             cid.Cid
	IPFSCatTraceID  trace.TraceID
	FindProvTraceID trace.TraceID

	IPFSCatStart time.Time
	IPFSCatEnd   time.Time
	IPFSCatTTFB  time.Duration
	FileSize     int

	IdleBroadcastStartedAt        time.Time
	FoundProvidersCount           int
	ConnectedProvidersCount       int
	FirstConnectedProviderFoundAt time.Time
	FirstProviderConnectedAt      time.Time

	IPNIStart  time.Time
	IPNIEnd    time.Time
	IPNIStatus int

	FirstBlockReceivedAt time.Time
	DiscoveryMethod      string

	spansByTraceID map[trace.TraceID][]*v1.Span
	cmdHandlerDone bool
}

func (r *DownloadResult) parse(req *ExportTraceServiceRequest) {
	var findProvSpan *v1.Span
	for span := range req.Spans() {
		if _, found := r.spansByTraceID[trace.TraceID(span.TraceId)]; found {
			r.spansByTraceID[trace.TraceID(span.TraceId)] = append(r.spansByTraceID[trace.TraceID(span.TraceId)], span)
		} else {
			r.spansByTraceID[trace.TraceID(span.TraceId)] = []*v1.Span{span}
		}

		// if we already have a trace ID for the provide operation, we can skip
		if r.FindProvTraceID.IsValid() {
			continue
		}

		// if the span is not a provide operation, we can skip
		if span.Name != "ProviderQueryManager.FindProvidersAsync" {
			continue
		}

		// if the span does not have the attribute key set to the raw CID, we can skip
		for _, attr := range span.Attributes {
			if attr.Key == "cid" && attr.Value.GetStringValue() == r.CID.String() {
				r.FindProvTraceID = trace.TraceID(span.TraceId)
				findProvSpan = span
			}
		}
	}

	if findProvSpan != nil {
		providersFoundAt := map[string]time.Time{}
		providersConnAt := map[string]time.Time{}
		for _, evt := range findProvSpan.Events {
			if evt.Name == "FoundProvider" {
				for _, attr := range evt.Attributes {
					if attr.Key == "peer" {
						providersFoundAt[attr.Value.GetStringValue()] = time.Unix(0, int64(evt.TimeUnixNano))
					}
				}
			} else if evt.Name == "ConnectedToProvider" {
				for _, attr := range evt.Attributes {
					if attr.Key == "peer" {
						providersConnAt[attr.Value.GetStringValue()] = time.Unix(0, int64(evt.TimeUnixNano))
					}
				}
			}
		}

		connectedProviderFoundAt := time.Unix(0, math.MaxInt64)
		connectedProviderAt := time.Unix(0, math.MaxInt64)
		for peerID, connAt := range providersConnAt {
			if connAt.Before(connectedProviderAt) {
				connectedProviderAt = connAt
				if foundAt, ok := providersFoundAt[peerID]; ok {
					connectedProviderFoundAt = foundAt
				}
			}
		}

		r.FoundProvidersCount = len(providersFoundAt)
		r.ConnectedProvidersCount = len(providersConnAt)
		r.FirstConnectedProviderFoundAt = connectedProviderFoundAt
		r.FirstProviderConnectedAt = connectedProviderAt
	}

	if spans, found := r.spansByTraceID[r.FindProvTraceID]; found && r.IPNIStart.IsZero() {
		for _, span := range spans {
			if span.Name != "DelegatedHTTPClient.FindProviders" {
				continue
			}

			r.IPNIStart = time.Unix(0, int64(span.StartTimeUnixNano))
			r.IPNIEnd = time.Unix(0, int64(span.EndTimeUnixNano))

			for _, attr := range span.GetAttributes() {
				if attr.Key == "http.response.status_code" {
					r.IPNIStatus = int(attr.GetValue().GetIntValue())
					break
				}
			}
		}
	}

	if spans, found := r.spansByTraceID[r.IPFSCatTraceID]; found {
		for _, span := range spans {
			switch {
			case span.Name == "corehttp.cmdsHandler":
				r.cmdHandlerDone = true
			case span.Name == "Bitswap.Client.Getter.SyncGetBlock":
				for _, evt := range span.Events {
					if evt.Name == "IdleBroadcast" {
						r.IdleBroadcastStartedAt = time.Unix(0, int64(evt.TimeUnixNano))
					}
				}
			case span.Name == "Bitswap.Client.Getter.handleIncoming":
				for _, evt := range span.Events {
					if evt.Name != "received block" {
						continue
					}

					blockReceivedAt := time.Unix(0, int64(evt.TimeUnixNano))
					if blockReceivedAt.Before(r.FirstBlockReceivedAt) || r.FirstBlockReceivedAt.IsZero() {
						r.FirstBlockReceivedAt = blockReceivedAt
					}
				}
			}
		}
	}

	if !r.FirstBlockReceivedAt.IsZero() && (r.FirstBlockReceivedAt.Before(r.FirstProviderConnectedAt) || r.FirstProviderConnectedAt.IsZero()) {
		r.DiscoveryMethod = "bitswap"
	} else if r.IPNIStatus == 200 {
		r.DiscoveryMethod = "ipni"
	} else if !r.IdleBroadcastStartedAt.IsZero() {
		r.DiscoveryMethod = "dht"
	} else {
		r.DiscoveryMethod = "unknown"
	}
}

func (r *DownloadResult) isPopulated() bool {
	if !r.cmdHandlerDone {
		return false
	}

	if r.IdleBroadcastStartedAt.IsZero() {
		return true
	}
	return true
}

func extractTimeRange(spans []*v1.Span) (time.Time, time.Time) {
	var (
		start int64 = math.MaxInt64
		end   int64 = 0
	)

	for _, span := range spans {
		if span.StartTimeUnixNano < math.MaxInt64 && int64(span.StartTimeUnixNano) < start {
			start = int64(span.StartTimeUnixNano)
		}

		if span.EndTimeUnixNano < math.MaxInt64 && int64(span.EndTimeUnixNano) > end {
			end = int64(span.EndTimeUnixNano)
		}
	}

	return time.Unix(0, start), time.Unix(0, end)
}
