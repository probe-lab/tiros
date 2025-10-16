CREATE TABLE website_probes
(
    run_id        String,
    -- the AWS region Tiros was deployed in
    region        String,
    -- the Tiros version that produced this download measurement
    tiros_version String,
    -- the Kubo version under test
    kubo_version  String,
    -- the Peer ID of the Kubo instance that produced this download measurement
    kubo_peer_id  String,
    -- the website under test
    website       String,
    -- the actual URL that the browser tried to load
    url           String,
    -- the protocol used to load the website (HTTP or IPFS)
    protocol      LowCardinality(String),
    -- if the protocol is IPFS, which IPFS implementation was used? Kubo, Helia?
    ipfs_impl     LowCardinality(String),
    -- which retry was this?
    try           Nullable(Int8),
    -- the time to first byte metric in milliseconds
    ttfb_ms       Nullable(Int32),
    -- the first contentful paint metric in milliseconds
    fcp_ms        Nullable(Int32),
    -- the largest contentful paint metric in milliseconds
    lcp_ms        Nullable(Int32),
    -- the time to interactive metric in milliseconds
    tti_ms        Nullable(Int32),
    -- the cumulative layout shift metric in milliseconds
    cls_ms        Nullable(Int32),
    -- the time to first byte web vitals rating (GOOD, NEEDS_IMPROVEMENT, POOR)
    ttfb_rating   LowCardinality(String),
    -- the cumulative layout shift web vitals rating (GOOD, NEEDS_IMPROVEMENT, POOR)
    cls_rating    LowCardinality(String),
    -- the first contentful paint web vitals rating (GOOD, NEEDS_IMPROVEMENT, POOR)
    fcp_rating    LowCardinality(String),
    -- the largest contentful paint web vitals rating (GOOD, NEEDS_IMPROVEMENT, POOR)
    lcp_rating    LowCardinality(String),
    -- the status code from Kubo when loading the website
    status_code   Int32,
    -- the complete HTML body of the website in case of an error
    body          Nullable(String),
    -- other metrics that were collected by Tiros
    metrics       JSON(),
    -- any error message that occurred
    error         Nullable(String),
    -- the time the measurement was created
    created_at    DateTime64(3, 'UTC')

) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (created_at, region, website)
      PARTITION BY toStartOfMonth(created_at);