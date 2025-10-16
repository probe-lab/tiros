CREATE TABLE websites
(
    domain         String,
    deactivated_at Nullable(DateTime)
) ENGINE = ReplicatedMergeTree
      PRIMARY KEY (deactivated_at, domain);