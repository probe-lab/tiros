BEGIN;

CREATE TABLE measurements
(
    id            INT GENERATED ALWAYS AS IDENTITY,
    run_id        INT         NOT NULL,
    region        TEXT        NOT NULL,
    url           TEXT        NOT NULL,
    version       TEXT        NOT NULL,
    node_num      SMALLINT    NOT NULL,
    instance_type TEXT        NOT NULL,
    uptime        INTERVAL    NOT NULL,
    page_load     FLOAT,
    metrics       JSONB,
    error         TEXT,
    created_at    TIMESTAMPTZ NOT NULL,

    CONSTRAINT fk_measurements_run
        FOREIGN KEY (run_id)
            REFERENCES runs (id),

    PRIMARY KEY (id)
);

CREATE INDEX idx_measurements_created_at ON measurements (created_at);

COMMIT;

