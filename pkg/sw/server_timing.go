package sw

import (
	"strings"

	servertiming "github.com/dennis-tra/go-server-timing"
)

// Server-Timing abbreviation tables from the helia-verified-fetch README.
// Stored in the DB as their expanded names so queries are self-documenting.
// Unknown abbreviations pass through verbatim (forward-compat with new codes).

// Metric names: d|i|n|p|f|c|b.
const (
	metricDNSLinkResolve = "dnslink_resolve"
	metricIPFSResolve    = "ipfs_resolve"
	metricIPNSResolve    = "ipns_resolve"
	metricProvider       = "provider"
	metricFindProviders  = "find_providers"
	metricConnect        = "connect"
	metricBlock          = "block"
)

// Subsystem values. Routers (h|l) apply to provider/find_providers metrics;
// brokers (t|b) apply to connect/block metrics. The abbreviations don't
// collide across those positions, so a single unified lookup table is safe.
const (
	systemHTTPGateway      = "http_gateway"
	systemLibp2p           = "libp2p"
	systemTrustlessGateway = "trustless_gateway"
	systemBitswap          = "bitswap"
)

var metricNames = map[string]string{
	"d": metricDNSLinkResolve,
	"i": metricIPFSResolve,
	"n": metricIPNSResolve,
	"p": metricProvider,
	"f": metricFindProviders,
	"c": metricConnect,
	"b": metricBlock,
}

var systemNames = map[string]string{
	"h": systemHTTPGateway,      // router, only valid for p/f
	"l": systemLibp2p,           // router, only valid for p/f
	"t": systemTrustlessGateway, // broker, only valid for c/b
	"b": systemBitswap,          // broker, only valid for c/b
}

var transportNames = map[string]string{
	"t": "tcp",
	"h": "http",
	"w": "websockets",
	"r": "webrtc",
	"d": "webrtc_direct",
	"q": "quic",
	"b": "webtransport",
	"u": "unknown",
}

// ServerTimingRow is the parsed, DB-ready projection of a []*servertiming.Metric.
// The *Arr slices are parallel (same length, one entry per metric, duplicates preserved,
// original order retained) and map directly onto the Nested `server_timing` column
// in the service_worker_probes table. The scalar fields are the hot-path aggregates
// used by dashboards.
//
// Ref grammar (from helia-verified-fetch README):
//
//	d/i/n : no desc              // DNSLink / IPFS / IPNS resolve
//	p     : desc="router,pid"    // provider found (router: h|l)
//	f     : desc="router,count"  // find-providers total per routing system
//	c     : desc="broker,pid,t"  // connect (broker: t|b, t: transport)
//	b     : desc="broker,pid,cid"// block retrieved
//
// router and broker are merged into a single `system` column since they never
// co-occur on the same metric; the metric name carries the lifecycle stage.
type ServerTimingRow struct {
	NameArr       []string
	DurSArr       []float64
	SystemArr     []string
	ProviderIDArr []string
	TransportArr  []string
	ExtraArr      []string

	IPFSResolveS             *float64
	DNSLinkResolveS          *float64
	IPNSResolveS             *float64
	FirstConnectS            *float64
	FirstBlockS              *float64
	ProviderCountHTTPGateway uint16
	ProviderCountLibp2p      uint16
	FastestBlockSystem       string
}

// ParseServerTimings converts a slice of server-timing metrics into a ServerTimingRow.
// Single-letter abbreviations are expanded to readable names; unknown codes pass through
// verbatim. Absent sub-fields are filled with "" sentinels so all parallel slices share
// the same length.
func ParseServerTimings(metrics []*servertiming.Metric) ServerTimingRow {
	row := ServerTimingRow{
		NameArr:       make([]string, 0, len(metrics)),
		DurSArr:       make([]float64, 0, len(metrics)),
		SystemArr:     make([]string, 0, len(metrics)),
		ProviderIDArr: make([]string, 0, len(metrics)),
		TransportArr:  make([]string, 0, len(metrics)),
		ExtraArr:      make([]string, 0, len(metrics)),
	}

	for _, m := range metrics {
		if m == nil {
			continue
		}

		name := expand(metricNames, m.Name)
		dur := m.Duration.Seconds()
		var system, providerID, transport, extra string

		// desc semantics depend on the raw metric name (the single letter).
		parts := splitDesc(m.Description)
		switch m.Name {
		case "p":
			// desc="router,providerId"
			system = expand(systemNames, get(parts, 0))
			providerID = get(parts, 1)
		case "f":
			// desc="router,count"
			system = expand(systemNames, get(parts, 0))
			extra = get(parts, 1)
		case "c":
			// desc="broker,providerId,transport"
			system = expand(systemNames, get(parts, 0))
			providerID = get(parts, 1)
			transport = expand(transportNames, get(parts, 2))
		case "b":
			// desc="broker,providerId,cid"
			system = expand(systemNames, get(parts, 0))
			providerID = get(parts, 1)
			extra = get(parts, 2)
		default:
			// d, i, n — no desc.
		}

		row.NameArr = append(row.NameArr, name)
		row.DurSArr = append(row.DurSArr, dur)
		row.SystemArr = append(row.SystemArr, system)
		row.ProviderIDArr = append(row.ProviderIDArr, providerID)
		row.TransportArr = append(row.TransportArr, transport)
		row.ExtraArr = append(row.ExtraArr, extra)

		// Scalar projections — switch on the raw abbreviation since that's the wire form.
		switch m.Name {
		case "i":
			if row.IPFSResolveS == nil {
				row.IPFSResolveS = new(dur)
			}
		case "d":
			if row.DNSLinkResolveS == nil {
				row.DNSLinkResolveS = new(dur)
			}
		case "n":
			if row.IPNSResolveS == nil {
				row.IPNSResolveS = new(dur)
			}
		case "c":
			if row.FirstConnectS == nil || dur < *row.FirstConnectS {
				row.FirstConnectS = new(dur)
			}
		case "b":
			if row.FirstBlockS == nil || dur < *row.FirstBlockS {
				row.FirstBlockS = new(dur)
				row.FastestBlockSystem = system
			}
		case "p":
			switch system {
			case systemHTTPGateway:
				row.ProviderCountHTTPGateway++
			case systemLibp2p:
				row.ProviderCountLibp2p++
			}
		}
	}

	return row
}

// expand returns table[code] if present, otherwise code itself (pass-through for
// unknown abbreviations so we don't silently drop new Server-Timing codes).
func expand(table map[string]string, code string) string {
	if code == "" {
		return ""
	}
	if v, ok := table[code]; ok {
		return v
	}
	return code
}

// splitDesc splits a Server-Timing desc value by commas. Returns nil for empty input.
func splitDesc(desc string) []string {
	if desc == "" {
		return nil
	}
	return strings.Split(desc, ",")
}

func get(parts []string, idx int) string {
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return parts[idx]
}
