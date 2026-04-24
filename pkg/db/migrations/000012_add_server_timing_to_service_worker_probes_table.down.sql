ALTER TABLE service_worker_probes
    DROP COLUMN IF EXISTS server_timing,
    DROP COLUMN IF EXISTS st_ipfs_resolve_s,
    DROP COLUMN IF EXISTS st_dnslink_resolve_s,
    DROP COLUMN IF EXISTS st_ipns_resolve_s,
    DROP COLUMN IF EXISTS st_first_connect_s,
    DROP COLUMN IF EXISTS st_first_block_s,
    DROP COLUMN IF EXISTS st_provider_count_http_gateway,
    DROP COLUMN IF EXISTS st_provider_count_libp2p,
    DROP COLUMN IF EXISTS st_fastest_block_broker,
    ADD COLUMN IF NOT EXISTS server_timings JSON() COMMENT 'Map of service worker internal timing metrics';
