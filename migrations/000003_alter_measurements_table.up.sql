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

UPDATE measurements
SET ttfb_rating = CASE -- https://web.dev/ttfb/#what-is-a-good-ttfb-score
        WHEN ttfb < '0.8 seconds' THEN
            'GOOD'
        WHEN ttfb < '1.8 seconds' THEN
            'NEEDS_IMPROVEMENT'
        ELSE
            'POOR'
    END::rating,
    fcp_rating = CASE -- https://web.dev/fcp/#what-is-a-good-fcp-score
        WHEN fcp < '1.8 seconds' THEN
            'GOOD'
        WHEN fcp < '3 seconds' THEN
            'NEEDS_IMPROVEMENT'
        ELSE
            'POOR'
    END::rating,
    lcp_rating = CASE -- https://web.dev/lcp/#what-is-a-good-lcp-score
        WHEN fcp < '2.5 seconds' THEN
            'GOOD'
        WHEN fcp < '4 seconds' THEN
            'NEEDS_IMPROVEMENT'
        ELSE
            'POOR'
    END::rating;

COMMIT;

