BEGIN;

ALTER TABLE measurements DROP COLUMN status_code;
ALTER TABLE measurements DROP COLUMN body;

COMMIT;