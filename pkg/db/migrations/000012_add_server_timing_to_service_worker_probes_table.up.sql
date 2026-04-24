ALTER TABLE service_worker_probes
    DROP COLUMN IF EXISTS server_timings,
    ADD COLUMN IF NOT EXISTS server_timing Nested(
        name        LowCardinality(String),
        dur_s       Float64,
        router      LowCardinality(String),
        broker      LowCardinality(String),
        provider_id String,
        transport   LowCardinality(String),
        extra       String
    ) COMMENT 'Per-metric Server-Timing entries parsed from the final response. Abbreviations expanded to readable names: name in (dnslink_resolve|ipfs_resolve|ipns_resolve|provider|find_providers|connect|block); router in (http_gateway|libp2p); broker in (trustless_gateway|bitswap); transport in (tcp|http|websockets|webrtc|webrtc_direct|quic|webtransport|unknown). Preserves duplicates and order.',
    ADD COLUMN IF NOT EXISTS st_ipfs_resolve_s              Nullable(Float64)      COMMENT 'Duration of the ipfs_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_dnslink_resolve_s           Nullable(Float64)      COMMENT 'Duration of the dnslink_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_ipns_resolve_s              Nullable(Float64)      COMMENT 'Duration of the ipns_resolve metric (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_first_connect_s             Nullable(Float64)      COMMENT 'Fastest connect duration across all providers (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_first_block_s               Nullable(Float64)      COMMENT 'Fastest block retrieval duration across all providers (seconds); derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_provider_count_http_gateway UInt16                 COMMENT 'Number of provider metrics with router=http_gateway; derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_provider_count_libp2p       UInt16                 COMMENT 'Number of provider metrics with router=libp2p; derived from server_timing',
    ADD COLUMN IF NOT EXISTS st_fastest_block_broker        LowCardinality(String) COMMENT 'Broker (trustless_gateway|bitswap) of the block metric with the fastest duration; empty if none; derived from server_timing';
