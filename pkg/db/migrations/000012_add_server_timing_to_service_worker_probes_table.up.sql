ALTER TABLE service_worker_probes
    ADD COLUMN IF NOT EXISTS server_timing_metrics Nested(
        name        LowCardinality(String),
        duration_s  Float64,
        system      LowCardinality(String),
        provider_id String,
        transport   LowCardinality(String),
        extra       String
    ) COMMENT 'Per-metric Server-Timing entries parsed from the final response. Abbreviations expanded to readable names: name in (dnslink_resolve|ipfs_resolve|ipns_resolve|provider|find_providers|connect|block); system in (http_gateway|libp2p|trustless_gateway|bitswap) — discovery subsystem for provider/find_providers, retrieval subsystem for connect/block; transport in (tcp|http|websockets|webrtc|webrtc_direct|quic|webtransport|unknown). Preserves duplicates and order.',
    ADD COLUMN IF NOT EXISTS st_ipfs_resolve_s              Nullable(Float64)      COMMENT 'Duration of the ipfs_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_dnslink_resolve_s           Nullable(Float64)      COMMENT 'Duration of the dnslink_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_ipns_resolve_s              Nullable(Float64)      COMMENT 'Duration of the ipns_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_first_connect_s             Nullable(Float64)      COMMENT 'Fastest connect duration across all providers (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_first_block_s               Nullable(Float64)      COMMENT 'Fastest block retrieval duration across all providers (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_provider_count_http_gateway UInt16                 COMMENT 'Number of provider metrics with system=http_gateway; derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_provider_count_libp2p       UInt16                 COMMENT 'Number of provider metrics with system=libp2p; derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_fastest_block_system        LowCardinality(String) COMMENT 'System of the block metric with the fastest duration (trustless_gateway|bitswap); empty if none; derived from server_timing';
