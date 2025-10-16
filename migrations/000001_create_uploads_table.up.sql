CREATE TABLE uploads
(
    -- the AWS region Tiros was deployed in
    region              String,
    -- the Tiros version that produced this upload measurement
    tiros_version       String,
    -- the Kubo version under test
    kubo_version        String,
    -- the Peer ID of the Kubo instance that produced this upload measurement
    kubo_peer_id        String,
    -- the file size of the uploaded file in bytes
    file_size_b         UInt32,
    -- the resulting IPFS CID of the uploaded file
    cid                 Nullable(String),
    -- the timestamp when `ipfs add` was called
    ipfs_add_start      DateTime64(3, 'UTC'),
    -- the duration of the `ipfs add` command in seconds
    ipfs_add_duration_s Float64,
    -- the timestamp when the provide operation was started
    provide_start       Nullable(DateTime64(3, 'UTC')),
    -- the duration of the provide operation in seconds
    provide_duration_s  Nullable(Float64),
    -- the delay between the `ipfs add` command and the start of the provide
    -- operation in seconds. May be negative if the provide operation
    -- started before the `ipfs add` command finished.
    provide_delay_s     Nullable(Float64),
    -- the total time from ipfs add to provide in seconds
    upload_duration_s   Nullable(Float64),
    -- the error message if the upload failed
    error               Nullable(String)
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (ipfs_add_start, region, kubo_version)
      PARTITION BY toStartOfMonth(ipfs_add_start);