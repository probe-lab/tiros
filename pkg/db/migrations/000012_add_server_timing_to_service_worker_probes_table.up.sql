ALTER TABLE service_worker_probes
    ADD COLUMN IF NOT EXISTS server_timing_metrics Nested(
        name        LowCardinality(String),
        duration_s  Float64,
        system      LowCardinality(String),
        provider_id String,
        transport   LowCardinality(String),
        extra       String
    ),
    ADD COLUMN IF NOT EXISTS st_ipfs_resolve_s              Nullable(Float64),
    ADD COLUMN IF NOT EXISTS st_dnslink_resolve_s           Nullable(Float64),
    ADD COLUMN IF NOT EXISTS st_ipns_resolve_s              Nullable(Float64),
    ADD COLUMN IF NOT EXISTS st_first_connect_s             Nullable(Float64),
    ADD COLUMN IF NOT EXISTS st_first_block_s               Nullable(Float64),
    ADD COLUMN IF NOT EXISTS st_provider_count_http_gateway UInt16,
    ADD COLUMN IF NOT EXISTS st_provider_count_libp2p       UInt16,
    ADD COLUMN IF NOT EXISTS st_fastest_block_system        LowCardinality(String);
