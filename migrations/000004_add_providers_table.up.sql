BEGIN;

CREATE TABLE providers
(
    id              INT GENERATED ALWAYS AS IDENTITY,
    run_id          INT         NOT NULL,
    website         TEXT        NOT NULL,
    path            TEXT        NOT NULL,
    peer_id         TEXT        NOT NULL,
    agent_version   TEXT,
    multi_addresses TEXT[],
    is_relayed      BOOL,
    country         TEXT,
    continent       TEXT,
    asn             INT,
    datacenter_id   INT,
    error           TEXT,

    created_at      TIMESTAMPTZ NOT NULL,

    CONSTRAINT fk_measurements_run
        FOREIGN KEY (run_id)
            REFERENCES runs (id),

    PRIMARY KEY (id)
);

CREATE INDEX idx_providers_website ON providers (website);

COMMIT;

