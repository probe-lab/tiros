BEGIN;

ALTER TABLE measurements
    DROP COLUMN tti;
ALTER TABLE measurements
    DROP COLUMN tti_rating;

ALTER TABLE measurements
    DROP COLUMN cls;
ALTER TABLE measurements
    DROP COLUMN cls_rating;

ALTER TABLE measurements
    DROP COLUMN ttfb_rating;
ALTER TABLE measurements
    DROP COLUMN fcp_rating;
ALTER TABLE measurements
    DROP COLUMN lcp_rating;

DROP TYPE rating;

COMMIT;