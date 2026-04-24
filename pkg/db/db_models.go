package db

import (
	"encoding/json"
	"time"
)

type UploadModel struct {
	RunID            string
	Region           string     `ch:"region"`
	TirosVersion     string     `ch:"tiros_version"`
	KuboVersion      string     `ch:"kubo_version"`
	KuboPeerID       string     `ch:"kubo_peer_id"`
	FileSizeB        *uint32    `ch:"file_size_b"`
	CID              *string    `ch:"cid"`
	IPFSAddStart     time.Time  `ch:"ipfs_add_start"`
	IPFSAddDurationS float64    `ch:"ipfs_add_duration_s"`
	ProvideStart     *time.Time `ch:"provide_start"`
	ProvideDurationS *float64   `ch:"provide_duration_s"`
	ProvideDelayS    *float64   `ch:"provide_delay_s"`
	UploadDurationS  *float64   `ch:"upload_duration_s"`
	Error            *string    `ch:"error"`
}

type DownloadModel struct {
	RunID                string
	Region               string     `ch:"region"`
	TirosVersion         string     `ch:"tiros_version"`
	KuboVersion          string     `ch:"kubo_version"`
	KuboPeerID           string     `ch:"kubo_peer_id"`
	FileSizeB            int32      `ch:"file_size_b"`
	MIMEType             string     `ch:"mime_type"`
	CID                  string     `ch:"cid"`
	IPFSCatStart         time.Time  `ch:"ipfs_cat_start"`
	IPFSCatDurationS     float64    `ch:"ipfs_cat_duration_s"`
	IPFSCatTTFBS         *float64   `ch:"ipfs_cat_ttfb_s"`
	IdleBroadcastStart   *time.Time `ch:"idle_broadcast_start"`
	FoundProvCount       int32      `ch:"found_prov_count"`
	ConnProvCount        int32      `ch:"conn_prov_count"`
	FirstConnProvFoundAt *time.Time `ch:"first_conn_prov_found_at"`
	FirstProvConnAt      *time.Time `ch:"first_prov_conn_at"`
	FirstProvPeerID      *string    `ch:"first_prov_peer_id"`
	IPNIStart            *time.Time `ch:"ipni_start"`
	IPNIDurationS        *float64   `ch:"ipni_duration_s"`
	IPNIStatus           *int32     `ch:"ipni_status"`
	FirstBlockReceivedAt *time.Time `ch:"first_block_rec_at"`
	DiscoveryMethod      *string    `ch:"discovery_method"`
	CIDSource            string     `ch:"cid_source"`
	Error                *string    `ch:"error"`
}

type WebsiteProbeProtocol string

const (
	WebsiteProbeProtocolIPFS WebsiteProbeProtocol = "IPFS"
	WebsiteProbeProtocolHTTP WebsiteProbeProtocol = "HTTP"
)

type WebsiteProbeModel struct {
	RunID        string          `ch:"run_id"`
	Region       string          `ch:"region"`
	TirosVersion string          `ch:"tiros_version"`
	KuboVersion  string          `ch:"kubo_version"`
	KuboPeerID   string          `ch:"kubo_peer_id"`
	Website      string          `ch:"website"`
	URL          string          `ch:"url"`
	Protocol     string          `ch:"protocol"`
	IPFSImpl     string          `ch:"ipfs_impl"`
	Try          int             `ch:"try"`
	TTFB         *float64        `ch:"ttfb_s"`
	FCP          *float64        `ch:"fcp_s"`
	LCP          *float64        `ch:"lcp_s"`
	TTI          *float64        `ch:"tti_s"`
	CLS          *float64        `ch:"cls_s"`
	TTFBRating   *string         `ch:"ttfb_rating"`
	CLSRating    *string         `ch:"cls_rating"`
	FCPRating    *string         `ch:"fcp_rating"`
	LCPRating    *string         `ch:"lcp_rating"`
	StatusCode   int             `ch:"status_code"`
	Body         *string         `ch:"body"`
	Metrics      json.RawMessage `ch:"metrics"`
	Error        *string         `ch:"error"`
	CreatedAt    time.Time       `ch:"created_at"`
}

type ProviderModel struct {
	RunID          string    `ch:"run_id"`
	Region         string    `ch:"region"`
	TirosVersion   string    `ch:"tiros_version"`
	KuboVersion    string    `ch:"kubo_version"`
	KuboPeerID     string    `ch:"kubo_peer_id"`
	Website        string    `ch:"website"`
	Path           string    `ch:"path"`
	ProviderID     string    `ch:"provider_id"`
	AgentVersion   *string   `ch:"agent_version"`
	MultiAddresses []string  `ch:"multi_addresses"`
	IsRelayed      *bool     `ch:"is_relayed"`
	Error          error     `ch:"error"`
	CreatedAt      time.Time `ch:"created_at"`
}

type GatewayProbeFormat string

const (
	GatewayProbeFormatNone GatewayProbeFormat = "none"
	GatewayProbeFormatRaw  GatewayProbeFormat = "raw"
	GatewayProbeFormatCAR  GatewayProbeFormat = "car"
)

type GatewayProbeModel struct {
	RunID             string    `ch:"run_id"`
	Region            string    `ch:"region"`
	TirosVersion      string    `ch:"tiros_version"`
	Gateway           string    `ch:"gateway"`
	CID               string    `ch:"cid"`
	CIDSource         string    `ch:"cid_source"`
	Format            string    `ch:"format"`
	RequestStart      time.Time `ch:"request_start"`
	DNSDurationS      *float64  `ch:"dns_duration_s"`
	ConnDurationS     *float64  `ch:"conn_duration_s"`
	TTFBS             *float64  `ch:"ttfb_s"`
	DownloadDurationS float64   `ch:"download_duration_s"`
	BytesReceived     int64     `ch:"bytes_received"`
	ContentLength     *int64    `ch:"content_length"`
	DownloadSpeedMbps *float64  `ch:"download_speed_mbps"`
	StatusCode        int       `ch:"status_code"`
	IPFSPath          *string   `ch:"ipfs_path"`
	IPFSRoots         *string   `ch:"ipfs_roots"`
	CacheStatus       *string   `ch:"cache_status"`
	ContentType       *string   `ch:"content_type"`
	CARValidated      *bool     `ch:"car_validated"`
	RedirectCount     int       `ch:"redirect_count"`
	FinalURL          *string   `ch:"final_url"`
	Error             *string   `ch:"error"`
	CreatedAt         time.Time `ch:"created_at"`
}

// ServiceWorkerProbeModel represents a performance measurement of an IPFS Service Worker Gateway.
// Service worker gateways intercept HTTP requests in the browser and serve IPFS content directly
// from the service worker, after an initial redirect chain from the gateway domain.
type ServiceWorkerProbeModel struct {
	// Run metadata
	RunID        string `ch:"run_id"`        // Unique identifier for this measurement run
	Region       string `ch:"region"`        // AWS region where the measurement was performed
	TirosVersion string `ch:"tiros_version"` // Version of Tiros performing the measurement
	Gateway      string `ch:"gateway"`       // Service worker gateway domain (e.g., "inbrowser.link")
	CID          string `ch:"cid"`           // IPFS Content ID being retrieved
	CIDSource    string `ch:"cid_source"`    // Source of the CID (e.g., "static", "bitsniffer_bitswap")
	URL          string `ch:"url"`           // Full URL requested (e.g., "https://inbrowser.link/ipfs/QmXxx")

	// Core timing metrics (all measured using browser's ResourceTiming API)
	// All timings use the same clock source (browser performance API) for consistency
	TotalTTFBS           *float64 `ch:"total_ttfb_s"`             // Time to first byte including all redirects (seconds)
	FinalTTFBS           *float64 `ch:"final_ttfb_s"`             // Time to first byte of final service worker response only (seconds)
	TimeToFinalRedirectS *float64 `ch:"time_to_final_redirect_s"` // Time from initial request to final service worker request (seconds)

	// Service worker metadata
	ServiceWorkerVersion *string `ch:"service_worker_version"` // Service worker version from "server" header (e.g., "@helia/service-worker-gateway/2.1.2#production@fb6750e")

	// Response details
	StatusCode    int     `ch:"status_code"`    // HTTP status code of final response (200 for success)
	ContentType   *string `ch:"content_type"`   // MIME type of the content (from "content-type" header)
	ContentLength *int64  `ch:"content_length"` // Size of the content in bytes (from "content-length" header)

	// IPFS-specific headers (from final service worker response)
	IPFSPath  *string `ch:"ipfs_path"`  // IPFS path of the content (from "x-ipfs-path" header)
	IPFSRoots *string `ch:"ipfs_roots"` // IPFS root CIDs involved in resolution (from "x-ipfs-roots" header)

	// Server timing data — parallel arrays bound to the Nested `server_timing` column.
	// All slices must have the same length; absent sub-fields use "" / 0 sentinels.
	// Single-letter abbreviations from the wire format are expanded to readable names.
	ServerTimingName       []string  `ch:"server_timing.name"`        // Metric: dnslink_resolve|ipfs_resolve|ipns_resolve|provider|find_providers|connect|block
	ServerTimingDurS       []float64 `ch:"server_timing.dur_s"`       // Metric duration in seconds
	ServerTimingSystem     []string  `ch:"server_timing.system"`      // Subsystem: http_gateway|libp2p (for provider/find_providers) or trustless_gateway|bitswap (for connect/block); empty otherwise
	ServerTimingProviderID []string  `ch:"server_timing.provider_id"` // Provider ID for provider/connect/block; empty otherwise
	ServerTimingTransport  []string  `ch:"server_timing.transport"`   // tcp|http|websockets|webrtc|webrtc_direct|quic|webtransport|unknown for connect; empty otherwise
	ServerTimingExtra      []string  `ch:"server_timing.extra"`       // Trailing desc payload: count for find_providers, cid for block; empty otherwise

	// Hot-path scalar projections of the server timings for cheap dashboard queries.
	// All st_* columns are derived from the Server-Timing header above.
	STIPFSResolveS             *float64 `ch:"st_ipfs_resolve_s"`              // Duration of the ipfs_resolve metric (seconds)
	STDNSLinkResolveS          *float64 `ch:"st_dnslink_resolve_s"`           // Duration of the dnslink_resolve metric (seconds)
	STIPNSResolveS             *float64 `ch:"st_ipns_resolve_s"`              // Duration of the ipns_resolve metric (seconds)
	STFirstConnectS            *float64 `ch:"st_first_connect_s"`             // Fastest connect duration across providers (seconds)
	STFirstBlockS              *float64 `ch:"st_first_block_s"`               // Fastest block retrieval duration across providers (seconds)
	STProviderCountHTTPGateway uint16   `ch:"st_provider_count_http_gateway"` // Number of provider metrics with system=http_gateway
	STProviderCountLibp2p      uint16   `ch:"st_provider_count_libp2p"`       // Number of provider metrics with system=libp2p
	STFastestBlockSystem       string   `ch:"st_fastest_block_system"`        // System of the fastest block metric (trustless_gateway|bitswap); empty if none

	// Provider and gateway metrics
	FoundProviders        int      `ch:"found_providers"`          // Number of unique providers found via delegated routing
	ServedFromGateway     bool     `ch:"served_from_gateway"`      // Whether content was successfully retrieved from a trustless gateway
	GatewayCacheStatus    *string  `ch:"gateway_cache_status"`     // Whether the content was served from the gateway's cache'
	DelegatedRouterTTFBS  *float64 `ch:"delegated_router_ttfb_s"`  // Fastest TTFB from any delegated router request (seconds)
	TrustlessGatewayTTFBS *float64 `ch:"trustless_gateway_ttfb_s"` // Fastest TTFB from any successful trustless gateway request (seconds)

	// Error tracking
	Error *string `ch:"error"` // Error message if probe failed, null if successful

	// Record metadata
	CreatedAt time.Time `ch:"created_at"` // Timestamp when this record was created
}
