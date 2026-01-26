CREATE TABLE downloads
(
    -- the AWS region Tiros was deployed in
    region                   String,
    -- the Tiros version that produced this download measurement
    tiros_version            String,
    -- the Kubo version under test
    kubo_version             String,
    -- the Peer ID of the Kubo instance that produced this download measurement
    kubo_peer_id             String,
    -- the file size of the downloaded file in bytes
    file_size_b              Nullable(UInt32),
    -- the CID of the downloaded file
    cid                      String,
    -- the timestamp at which the download was started
    ipfs_cat_start           DateTime64(3, 'UTC'),
    -- the total time it took to download the file in seconds
    ipfs_cat_duration_s      Float64,
    -- the time it took to receive the first byte (time to first byte - ttfb) in seconds
    ipfs_cat_ttfb_s          Nullable(Float64),
    -- if the file cannot be resolved within the ProviderDelay timeout through
    -- bitswap (usually this is set to 1s), Kubo will start looking in the DHT
    -- or IPNI. This is internally marked as an "Idle Broadcast". This is the
    -- timestamp at which the first idle broadcast started (there could be multiple)
    idle_broadcast_start     Nullable(DateTime64(3, 'UTC')),
    -- the total number of providers that were found in the DHT or IPNI
    found_prov_count         Int32,
    -- the total number of providers that Kubo connected to during this download
    conn_prov_count          Int32,
    -- the timestamp at which Kubo found the provider to which it connected to first
    first_conn_prov_found_at Nullable(DateTime64(3, 'UTC')),
    -- the earliest timestamp at which Kubo connected to one of the found providers
    first_prov_conn_at       Nullable(DateTime64(3, 'UTC')),
    -- the Peer ID of the provider to which Kubo connected to first
    first_prov_peer_id       Nullable(String),
    -- the timestamp at which Kubo started to query IPNI for the CID
    ipni_start               Nullable(DateTime64(3, 'UTC')),
    -- the duration of the IPNI query in seconds
    ipni_duration_s          Nullable(Float64),
    -- the status code of the IPNI query
    ipni_status              Nullable(Int32),
    -- the timestamp at which Kubo received the first block from the provider
    first_block_rec_at       Nullable(DateTime64(3, 'UTC')),
    -- a derived field that indicates which routing subsystem resolved the CID (bitswap, dht, ipni)
    discovery_method         Nullable(String),
    -- a key that indicates where the CID that was downloaded came from
    cid_source               LowCardinality(String),
    -- the error message if the download failed
    error                    Nullable(String)
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (ipfs_cat_start, region, kubo_version)
      PARTITION BY toStartOfMonth(ipfs_cat_start);