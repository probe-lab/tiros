ALTER TABLE downloads
    ADD COLUMN IF NOT EXISTS mime_type LowCardinality(Nullable(String)) AFTER file_size_b;