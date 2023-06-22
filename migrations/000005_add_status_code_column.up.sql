BEGIN;

ALTER TABLE measurements ADD COLUMN status_code INT;
ALTER TABLE measurements ADD COLUMN body TEXT;

COMMIT;