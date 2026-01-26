CREATE TABLE gateway_probes
(
    run_id               String,
    -- the AWS region Tiros was deployed in
    region               String,
    -- the Tiros version that produced this gateway measurement
    tiros_version        String,
    -- the IPFS gateway under test (e.g., "ipfs.io", "dweb.link")
    gateway              String,
    -- the CID that was requested from the gateway
    cid                  String,
    -- the source of the CID (e.g., "bitsniffer_bitswap", "static")
    cid_source           String,
    -- the format requested from the gateway ("raw" or "car")
    format               LowCardinality(String),
    -- the timestamp when the request started
    request_start        DateTime64(3, 'UTC'),
    -- the DNS resolution duration in seconds
    dns_duration_s       Nullable(Float64),
    -- the TCP connection establishment duration in seconds (including TLS if applicable)
    conn_duration_s      Nullable(Float64),
    -- the time to first byte in seconds
    ttfb_s               Nullable(Float64),
    -- the total download duration in seconds (from request start to completion/cancellation)
    download_duration_s  Float64,
    -- the number of bytes received before cancellation or completion
    bytes_received       Int64,
    -- the Content-Length header value if present
    content_length       Nullable(Int64),
    -- the download speed in megabits per second
    download_speed_mbps  Nullable(Float64),
    -- the HTTP status code returned by the gateway
    status_code          Int32,
    -- the X-Ipfs-Path header value (IPFS path resolution verification)
    ipfs_path            Nullable(String),
    -- the X-Ipfs-Roots header value (CID roots for trustless verification)
    ipfs_roots           Nullable(String),
    -- the cache status (e.g., "HIT", "MISS", ...)
    cache_status         Nullable(String),
    -- the Content-Type header value
    content_type         Nullable(String),
    -- whether the CAR file was validated (only applicable for "car" format)
    car_validated        Nullable(Bool),
    -- the number of HTTP redirects that occurred
    redirect_count       Int32,
    -- the final URL after following all redirects
    final_url            Nullable(String),
    -- any error message that occurred during the request
    error                Nullable(String),
    -- the time the measurement was created
    created_at           DateTime64(3, 'UTC')

) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (created_at, region, gateway, format)
      PARTITION BY toStartOfMonth(created_at);
