CREATE TABLE providers
(
    run_it          String,
    -- the AWS region Tiros was deployed in
    region          String,
    -- the Tiros version that produced this download measurement
    tiros_version   String,
    -- the Kubo version under test
    kubo_version    String,
    -- the Peer ID of the Kubo instance that produced this download measurement
    kubo_peer_id    String,

    website         String,
    path            String,
    provider_id     String,
    agent_version   String,
    multi_addresses Array(String),
    is_relayed      Nullable(Bool),

    error           String,
    created_at      DateTime64(3, 'UTC')

) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (created_at, region, kubo_version)
      PARTITION BY toStartOfMonth(created_at);