-- Reverse the changes: add namespace back to organizations and remove from VDCs
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS namespace VARCHAR(255) UNIQUE;

-- Remove VDC namespace fields
DROP INDEX IF EXISTS idx_vdcs_namespace;
ALTER TABLE vdcs DROP CONSTRAINT IF EXISTS unique_vdc_name_per_org;
ALTER TABLE vdcs DROP COLUMN IF EXISTS namespace;