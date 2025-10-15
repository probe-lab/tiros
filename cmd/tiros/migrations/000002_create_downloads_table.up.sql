CREATE TABLE downloads
(
    region                   String,
    tiros_version            String,
    kubo_version             String,
    kubo_peer_id             String,
    file_size_b              UInt32,
    cid                      String,
    ipfs_cat_start           DateTime64(3, 'UTC'),
    ipfs_cat_ttfb_ms         Int32,
    ipfs_cat_duration_ms     Int32,
    idle_broadcast_start     DateTime64(3, 'UTC'),
    found_prov_count         Int32,
    conn_prov_count          Int32,
    first_conn_prov_found_at DateTime64(3, 'UTC'),
    first_prov_conn_at       DateTime64(3, 'UTC'),
    first_prov_peer_id       String,
    ipni_start               DateTime64(3, 'UTC'),
    ipni_duration_ms         Int32,
    ipni_status              Int32,
    first_block_rec_at       DateTime64(3, 'UTC'),
    discovery_method         LowCardinality(String),
    error                    String
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (ipfs_cat_start, region, kubo_version)
      PARTITION BY toStartOfMonth(ipfs_cat_start);