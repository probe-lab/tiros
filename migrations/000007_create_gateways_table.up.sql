CREATE TABLE gateways
(
    domain         String,
    deactivated_at Nullable(DateTime('UTC'))
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (domain);
