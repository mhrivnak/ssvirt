-- Remove namespace field from organizations table (organizations are now PostgreSQL-only)
ALTER TABLE organizations DROP COLUMN IF EXISTS namespace;

-- Add namespace field to VDCs table (VDCs now map to Kubernetes namespaces)
ALTER TABLE vdcs ADD COLUMN IF NOT EXISTS namespace VARCHAR(253) UNIQUE;

-- Add index for performance
CREATE INDEX IF NOT EXISTS idx_vdcs_namespace ON vdcs(namespace);

-- Add constraint to ensure VDC names are unique within an organization
ALTER TABLE vdcs DROP CONSTRAINT IF EXISTS unique_vdc_name_per_org;
ALTER TABLE vdcs ADD CONSTRAINT unique_vdc_name_per_org UNIQUE (organization_id, name);