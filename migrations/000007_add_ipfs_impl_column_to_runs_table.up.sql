BEGIN;

ALTER TABLE runs
    ADD COLUMN ipfs_impl TEXT;

UPDATE runs SET ipfs_impl = 'KUBO';

ALTER TABLE runs
    ALTER COLUMN ipfs_impl SET NOT NULL;

COMMIT;