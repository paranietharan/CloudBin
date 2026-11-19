ALTER TABLE objects ADD COLUMN storage_key TEXT;
CREATE TABLE storage_cleanup (
    storage_key TEXT PRIMARY KEY,
    nodes TEXT[] NOT NULL,
    eligible_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '1 hour'
);
CREATE INDEX storage_cleanup_due ON storage_cleanup(eligible_at);
ALTER TABLE object_placements ADD COLUMN nodes TEXT[];
