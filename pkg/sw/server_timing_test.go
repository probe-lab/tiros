package sw

import (
	"testing"

	servertiming "github.com/dennis-tra/go-server-timing"
)

func TestParseServerTimings(t *testing.T) {
	t.Parallel()

	// Header from helia-verified-fetch README. Parsing it with the servertiming
	// library gives us the exact structure the service worker emits in the wild.
	const readmeHeader = `i;dur=0,p;dur=0;desc="h,bagqbeaawn",p;dur=0;desc="h,bagqbeaawn",` +
		`p;dur=1;desc="h,bagqbeaa7n",p;dur=1;desc="h,bagqbeaa7n",` +
		`f;dur=1;desc="h,4",f;dur=1;desc="h,4",f;dur=144;desc="l,0",f;dur=144;desc="l,0",` +
		`c;dur=206;desc="t,bagqbeaa7n,h",b;dur=1000;desc="t,bagqbeaa7n,bafybeigoc"`

	parsed, err := servertiming.ParseHeader(readmeHeader)
	if err != nil {
		t.Fatalf("parse readme header: %v", err)
	}
	readmeMetrics := parsed.Metrics()

	tests := []struct {
		name    string
		metrics []*servertiming.Metric

		wantLen                      int
		wantIPFSResolveS             *float64
		wantDNSLinkResolveS          *float64
		wantIPNSResolveS             *float64
		wantFirstConnectS            *float64
		wantFirstBlockS              *float64
		wantProviderCountHTTPGateway uint16
		wantProviderCountLibp2p      uint16
		wantFastestBlockSystem       string

		// Spot checks on parsed sub-fields at particular indices.
		checks []fieldCheck
	}{
		{
			name:                         "readme_example",
			metrics:                      readmeMetrics,
			wantLen:                      11,
			wantIPFSResolveS:             fptr(0),
			wantDNSLinkResolveS:          nil,
			wantIPNSResolveS:             nil,
			wantFirstConnectS:            fptr(0.206),
			wantFirstBlockS:              fptr(1.0),
			wantProviderCountHTTPGateway: 4, // 4 'p' entries, all system=http_gateway
			wantProviderCountLibp2p:      0,
			wantFastestBlockSystem:       "trustless_gateway",
			checks: []fieldCheck{
				// Entry 0: i;dur=0
				{idx: 0, name: "ipfs_resolve", durS: 0},
				// Entry 1: p;dur=0;desc="h,bagqbeaawn"
				{idx: 1, name: "provider", durS: 0, system: "http_gateway", providerID: "bagqbeaawn"},
				// Entry 7: f;dur=144;desc="l,0"
				{idx: 7, name: "find_providers", durS: 0.144, system: "libp2p", extra: "0"},
				// Entry 9: c;dur=206;desc="t,bagqbeaa7n,h"
				{idx: 9, name: "connect", durS: 0.206, system: "trustless_gateway", providerID: "bagqbeaa7n", transport: "http"},
				// Entry 10: b;dur=1000;desc="t,bagqbeaa7n,bafybeigoc"
				{idx: 10, name: "block", durS: 1.0, system: "trustless_gateway", providerID: "bagqbeaa7n", extra: "bafybeigoc"},
			},
		},
		{
			name:                   "empty",
			metrics:                nil,
			wantLen:                0,
			wantFastestBlockSystem: "",
		},
		{
			name:                   "only_resolve_metrics_no_desc",
			metrics:                mustParse(t, `d;dur=12,i;dur=0,n;dur=34`),
			wantLen:                3,
			wantDNSLinkResolveS:    fptr(0.012),
			wantIPFSResolveS:       fptr(0),
			wantIPNSResolveS:       fptr(0.034),
			wantFastestBlockSystem: "",
			checks: []fieldCheck{
				{idx: 0, name: "dnslink_resolve", durS: 0.012},
				{idx: 1, name: "ipfs_resolve", durS: 0},
				{idx: 2, name: "ipns_resolve", durS: 0.034},
			},
		},
		{
			name: "multiple_blocks_picks_fastest_system",
			metrics: mustParse(t,
				`b;dur=500;desc="t,prov1,cidX",b;dur=100;desc="b,prov2,cidY",b;dur=900;desc="t,prov3,cidZ"`),
			wantLen:                3,
			wantFirstBlockS:        fptr(0.1),
			wantFastestBlockSystem: "bitswap",
		},
		{
			name:                         "libp2p_and_http_provider_counts",
			metrics:                      mustParse(t, `p;dur=1;desc="h,a",p;dur=2;desc="l,b",p;dur=3;desc="l,c"`),
			wantLen:                      3,
			wantProviderCountHTTPGateway: 1,
			wantProviderCountLibp2p:      2,
			wantFastestBlockSystem:       "",
		},
		{
			name:    "unknown_abbrev_passes_through",
			metrics: mustParse(t, `x;dur=5;desc="z,pid1"`),
			wantLen: 1,
			checks: []fieldCheck{
				// Unknown metric code 'x' passes through verbatim. The 'z' system code
				// is unmapped for any m.Name, so system stays empty (x isn't p/f/c/b).
				{idx: 0, name: "x", durS: 0.005},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseServerTimings(tt.metrics)

			// All parallel slices must have equal length — this is a hard invariant
			// ClickHouse enforces on Nested columns on insert.
			assertLen(t, "NameArr", got.NameArr, tt.wantLen)
			assertLen(t, "DurSArr", got.DurSArr, tt.wantLen)
			assertLen(t, "SystemArr", got.SystemArr, tt.wantLen)
			assertLen(t, "ProviderIDArr", got.ProviderIDArr, tt.wantLen)
			assertLen(t, "TransportArr", got.TransportArr, tt.wantLen)
			assertLen(t, "ExtraArr", got.ExtraArr, tt.wantLen)

			assertPtrFloat(t, "IPFSResolveS", got.IPFSResolveS, tt.wantIPFSResolveS)
			assertPtrFloat(t, "DNSLinkResolveS", got.DNSLinkResolveS, tt.wantDNSLinkResolveS)
			assertPtrFloat(t, "IPNSResolveS", got.IPNSResolveS, tt.wantIPNSResolveS)
			assertPtrFloat(t, "FirstConnectS", got.FirstConnectS, tt.wantFirstConnectS)
			assertPtrFloat(t, "FirstBlockS", got.FirstBlockS, tt.wantFirstBlockS)

			if got.ProviderCountHTTPGateway != tt.wantProviderCountHTTPGateway {
				t.Errorf("ProviderCountHTTPGateway: got %d, want %d", got.ProviderCountHTTPGateway, tt.wantProviderCountHTTPGateway)
			}
			if got.ProviderCountLibp2p != tt.wantProviderCountLibp2p {
				t.Errorf("ProviderCountLibp2p: got %d, want %d", got.ProviderCountLibp2p, tt.wantProviderCountLibp2p)
			}
			if got.FastestBlockSystem != tt.wantFastestBlockSystem {
				t.Errorf("FastestBlockSystem: got %q, want %q", got.FastestBlockSystem, tt.wantFastestBlockSystem)
			}

			for _, c := range tt.checks {
				c.assert(t, got)
			}
		})
	}
}

func TestParseServerTimings_NilMetricSkipped(t *testing.T) {
	t.Parallel()

	// A nil entry in the slice should be skipped without panic or length skew.
	m := &servertiming.Metric{Name: "i"}
	got := ParseServerTimings([]*servertiming.Metric{nil, m, nil})
	if len(got.NameArr) != 1 || got.NameArr[0] != "ipfs_resolve" {
		t.Errorf("expected exactly one 'ipfs_resolve' entry, got %v", got.NameArr)
	}
}

// --- helpers ---

type fieldCheck struct {
	idx        int
	name       string
	durS       float64
	system     string
	providerID string
	transport  string
	extra      string
}

func (c fieldCheck) assert(t *testing.T, got ServerTimingRow) {
	t.Helper()
	if c.idx >= len(got.NameArr) {
		t.Fatalf("idx %d out of range (len=%d)", c.idx, len(got.NameArr))
	}
	if got.NameArr[c.idx] != c.name {
		t.Errorf("idx %d name: got %q, want %q", c.idx, got.NameArr[c.idx], c.name)
	}
	if !floatsClose(got.DurSArr[c.idx], c.durS) {
		t.Errorf("idx %d durS: got %v, want %v", c.idx, got.DurSArr[c.idx], c.durS)
	}
	if got.SystemArr[c.idx] != c.system {
		t.Errorf("idx %d system: got %q, want %q", c.idx, got.SystemArr[c.idx], c.system)
	}
	if got.ProviderIDArr[c.idx] != c.providerID {
		t.Errorf("idx %d providerID: got %q, want %q", c.idx, got.ProviderIDArr[c.idx], c.providerID)
	}
	if got.TransportArr[c.idx] != c.transport {
		t.Errorf("idx %d transport: got %q, want %q", c.idx, got.TransportArr[c.idx], c.transport)
	}
	if got.ExtraArr[c.idx] != c.extra {
		t.Errorf("idx %d extra: got %q, want %q", c.idx, got.ExtraArr[c.idx], c.extra)
	}
}

func mustParse(t *testing.T, header string) []*servertiming.Metric {
	t.Helper()
	h, err := servertiming.ParseHeader(header)
	if err != nil {
		t.Fatalf("parse header %q: %v", header, err)
	}
	return h.Metrics()
}

func fptr(v float64) *float64 { return &v }

func assertLen[T any](t *testing.T, label string, s []T, want int) {
	t.Helper()
	if len(s) != want {
		t.Errorf("%s length: got %d, want %d", label, len(s), want)
	}
}

func assertPtrFloat(t *testing.T, label string, got, want *float64) {
	t.Helper()
	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Errorf("%s: got %v, want %v", label, got, want)
	case !floatsClose(*got, *want):
		t.Errorf("%s: got %v, want %v", label, *got, *want)
	}
}

func floatsClose(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}
