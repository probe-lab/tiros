package main

import (
	"encoding/json"
	"time"
)

type UploadModel struct {
	RunID            string     `ch:"run_id"`
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
	RunID                string     `ch:"run_id"`
	Region               string     `ch:"region"`
	TirosVersion         string     `ch:"tiros_version"`
	KuboVersion          string     `ch:"kubo_version"`
	KuboPeerID           string     `ch:"kubo_peer_id"`
	FileSizeB            int32      `ch:"file_size_b"`
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
	TTFBS        *float64        `ch:"ttfb_s"`
	FCPS         *float64        `ch:"fcp_s"`
	LCPS         *float64        `ch:"lcp_s"`
	TTIS         *float64        `ch:"tti_s"`
	CLSS         *float64        `ch:"cls_s"`
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
	RunID string `ch:"run_id"`
	// ID             int               `boil:"id" json:"id" toml:"id" yaml:"id"`
	// RunID          int               `boil:"run_id" json:"run_id" toml:"run_id" yaml:"run_id"`
	// Website        string            `boil:"website" json:"website" toml:"website" yaml:"website"`
	// Path           string            `boil:"path" json:"path" toml:"path" yaml:"path"`
	// PeerID         string            `boil:"peer_id" json:"peer_id" toml:"peer_id" yaml:"peer_id"`
	// AgentVersion   null.String       `boil:"agent_version" json:"agent_version,omitempty" toml:"agent_version" yaml:"agent_version,omitempty"`
	// MultiAddresses types.StringArray `boil:"multi_addresses" json:"multi_addresses,omitempty" toml:"multi_addresses" yaml:"multi_addresses,omitempty"`
	// IsRelayed      null.Bool         `boil:"is_relayed" json:"is_relayed,omitempty" toml:"is_relayed" yaml:"is_relayed,omitempty"`
	// Country        null.String       `boil:"country" json:"country,omitempty" toml:"country" yaml:"country,omitempty"`
	// Continent      null.String       `boil:"continent" json:"continent,omitempty" toml:"continent" yaml:"continent,omitempty"`
	// Asn            null.Int          `boil:"asn" json:"asn,omitempty" toml:"asn" yaml:"asn,omitempty"`
	// DatacenterID   null.Int          `boil:"datacenter_id" json:"datacenter_id,omitempty" toml:"datacenter_id" yaml:"datacenter_id,omitempty"`
	// Error          null.String       `boil:"error" json:"error,omitempty" toml:"error" yaml:"error,omitempty"`
	// CreatedAt      time.Time         `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
}
