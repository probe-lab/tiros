ALTER TABLE service_worker_probes
    ADD COLUMN IF NOT EXISTS gateway_cache_status LowCardinality(Nullable(String));