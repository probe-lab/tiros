BEGIN;

CREATE TYPE rating AS ENUM (
    'GOOD',
    'NEEDS_IMPROVEMENT',
    'POOR'
    );

ALTER TABLE measurements
    ADD COLUMN tti INTERVAL;
ALTER TABLE measurements
    ADD COLUMN cls INTERVAL;

ALTER TABLE measurements
    ADD COLUMN tti_rating rating;
ALTER TABLE measurements
    ADD COLUMN cls_rating rating;
ALTER TABLE measurements
    ADD COLUMN ttfb_rating rating;
ALTER TABLE measurements
    ADD COLUMN fcp_rating rating;
ALTER TABLE measurements
    ADD COLUMN lcp_rating rating;

COMMIT;

