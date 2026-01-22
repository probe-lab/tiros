CREATE TABLE IF NOT EXISTS service_worker_probes
(
    run_id                   String COMMENT 'Unique identifier for this measurement run',
    region                   String COMMENT 'Region identifier where the measurement was performed',
    tiros_version            LowCardinality(String) COMMENT 'Version of Tiros performing the measurement',
    gateway                  LowCardinality(String) COMMENT 'Service worker gateway domain (e.g., "inbrowser.link")',
    cid                      String COMMENT 'IPFS Content ID being retrieved',
    cid_source               String COMMENT 'Source of the CID (e.g., "static", "bitsniffer_bitswap")',
    url                      String COMMENT 'Full URL requested (e.g., "https://inbrowser.link/ipfs/QmXxx")',
    total_ttfb_s             Nullable(Float64) COMMENT 'Time to first byte including all redirects (seconds, browser ResourceTiming)',
    final_ttfb_s             Nullable(Float64) COMMENT 'Time to first byte of final service worker response only (seconds, browser ResourceTiming)',
    time_to_final_redirect_s Nullable(Float64) COMMENT 'Time from initial request to final service worker request (seconds, browser ResourceTiming)',
    service_worker_version   Nullable(String) COMMENT 'Service worker version from "server" header (e.g., "@helia/service-worker-gateway/2.1.2#production@fb6750e")',
    status_code              Int32 COMMENT 'HTTP status code of final response (200 for success)',
    content_type             LowCardinality(Nullable(String)) COMMENT 'MIME type of the content (from "content-type" header)',
    content_length           Nullable(Int64) COMMENT 'Size of the content in bytes (from "content-length" header)',
    ipfs_path                Nullable(String) COMMENT 'IPFS path of the content (from "x-ipfs-path" header)',
    ipfs_roots               Nullable(String) COMMENT 'IPFS root CIDs involved in resolution (from "x-ipfs-roots" header)',
    server_timings           JSON() COMMENT 'Map of service worker internal timing metrics',
    found_providers          Int32 COMMENT 'Number of unique providers found via delegated routing',
    served_from_gateway      Bool COMMENT 'Whether content was successfully retrieved from a trustless gateway',
    delegated_router_ttfb_s  Nullable(Float64) COMMENT 'Fastest TTFB from any delegated router request (seconds)',
    trustless_gateway_ttfb_s Nullable(Float64) COMMENT 'Fastest TTFB from any successful trustless gateway request (seconds)',
    error                    Nullable(String) COMMENT 'Error message if probe failed, null if successful',
    created_at               DateTime64(3) COMMENT 'Timestamp when this record was created'
)
    ENGINE = ReplicatedMergeTree()
        ORDER BY (created_at, region, gateway)
        PARTITION BY toStartOfMonth(created_at);

