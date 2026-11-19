-- Blobs in the newer format require this pointer. Refuse lossy downgrade.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM objects WHERE storage_key IS NOT NULL)
       OR EXISTS (SELECT 1 FROM storage_cleanup) THEN
        RAISE EXCEPTION 'Migrate versioned files and drain cleanup before downgrading';
    END IF;
END $$;
DROP TABLE storage_cleanup;
ALTER TABLE objects DROP COLUMN storage_key;
ALTER TABLE object_placements DROP COLUMN nodes;
