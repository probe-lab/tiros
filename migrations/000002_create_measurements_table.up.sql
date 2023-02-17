BEGIN;

CREATE TYPE measurement_type AS ENUM (
    'HTTP',
    'KUBO'
    );

CREATE TABLE measurements
(
    id            INT GENERATED ALWAYS AS IDENTITY,
    run_id        INT              NOT NULL,
    region        TEXT             NOT NULL,
    website       TEXT             NOT NULL,
    url           TEXT             NOT NULL,
    version       TEXT             NOT NULL,
    type          measurement_type NOT NULL,
    try           SMALLINT         NOT NULL,
    node          SMALLINT         NOT NULL,
    instance_type TEXT             NOT NULL,
    uptime        INTERVAL         NOT NULL,
    metrics       JSONB,
    error         TEXT,
    created_at    TIMESTAMPTZ      NOT NULL,

    CONSTRAINT fk_measurements_run
        FOREIGN KEY (run_id)
            REFERENCES runs (id),

    PRIMARY KEY (id)
);

CREATE INDEX idx_measurements_created_at ON measurements (created_at);

COMMIT;

