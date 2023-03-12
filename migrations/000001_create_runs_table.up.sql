BEGIN;

CREATE TABLE runs
(
    id           INT GENERATED ALWAYS AS IDENTITY,
    region       TEXT        NOT NULL,
    websites     TEXT[]      NOT NULL,
    settle_short FLOAT       NOT NULL,
    settle_long  FLOAT       NOT NULL,
    version      TEXT        NOT NULL,
    times        SMALLINT    NOT NULL,
    cpu          INT         NOT NULL,
    memory       INT         NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    finished_at  TIMESTAMPTZ,

    PRIMARY KEY (id)
);

CREATE INDEX idx_runs_created_at ON runs (created_at);

COMMIT;