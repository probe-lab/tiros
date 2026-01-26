CREATE TABLE providers
(
    run_id          String,
    -- the AWS region Tiros was deployed in
    region          String,
    -- the Tiros version that produced this download measurement
    tiros_version   String,
    -- the Kubo version under test
    kubo_version    String,
    -- the Peer ID of the Kubo instance that produced this download measurement
    kubo_peer_id    String,
    -- the website that this peer provides
    website         String,
    -- the path that was checked
    path            String,
    -- the peer ID of the provider
    provider_id     String,
    -- the agent version of the provider
    agent_version   Nullable(String),
    -- the multiaddresses of the provider
    multi_addresses Array(String),
    -- whether the provider is relayed
    is_relayed      Nullable(Bool),
    -- the error that occurred, if any
    error           Nullable(String),
    -- the time that the measurement was created
    created_at      DateTime64(3, 'UTC')
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (created_at, region, kubo_version)
      PARTITION BY toStartOfMonth(created_at);