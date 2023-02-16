CREATE TABLE measurements
(
    id         INT GENERATED ALWAYS AS IDENTITY,
    run_id     TEXT        NOT NULL,
    region     TEXT        NOT NULL,
    url        TEXT        NOT NULL,
    version    TEXT        NOT NULL,
    node_num   SMALLINT    NOT NULL,
    latency    FLOAT       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (id)
);

CREATE INDEX idx_measurements_created_at ON measurements (created_at);