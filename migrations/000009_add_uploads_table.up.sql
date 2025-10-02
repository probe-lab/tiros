BEGIN;

CREATE TABLE uploads
(
    id           INT GENERATED ALWAYS AS IDENTITY,
    cid          TEXT        NOT NULL,
    trace_id     TEXT        NOT NULL,
    file_size    INT         NOT NULL,
    region       TEXT        NOT NULL,
    kubo_version TEXT        NOT NULL,

    created_at   TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (id)
);

CREATE INDEX idx_uploads_created_at ON uploads (created_at);

COMMIT;

