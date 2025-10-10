CREATE TABLE uploads
(
    region               String,
    tiros_version        String,
    kubo_version         String,
    file_size_mib        UInt32,
    ipfs_add_start       DateTime64(3, 'UTC'),
    ipfs_add_duration_ms Int32,
    provide_start        DateTime64(3, 'UTC'),
    provide_duration_ms  Int32,
    provide_delay_ms     Int32,
    upload_duration_ms   Int32
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (ipfs_add_start, region, kubo_version)
      PARTITION BY toStartOfMonth(ipfs_add_start);