-- Reverse the VDC namespace constraint fix
-- This will restore the original unique constraint

-- Drop the partial unique index
DROP INDEX IF EXISTS idx_vdcs_namespace_unique;

-- Restore the original namespace column constraint
-- Note: This may fail if there are conflicting deleted records
ALTER TABLE vdcs ADD CONSTRAINT vdcs_namespace_key UNIQUE (namespace);

-- Restore the original index
CREATE INDEX IF NOT EXISTS idx_vdcs_namespace ON vdcs(namespace);