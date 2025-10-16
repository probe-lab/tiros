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
