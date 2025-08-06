-- Remove namespace field from organizations table (organizations are now PostgreSQL-only)
ALTER TABLE organizations DROP COLUMN IF EXISTS namespace;

-- Add namespace_name field to VDCs table (VDCs now map to Kubernetes namespaces)
ALTER TABLE vdcs ADD COLUMN IF NOT EXISTS namespace_name VARCHAR(253) UNIQUE;

-- Add index for performance
CREATE INDEX IF NOT EXISTS idx_vdcs_namespace_name ON vdcs(namespace_name);

-- Add constraint to ensure VDC names are unique within an organization
ALTER TABLE vdcs DROP CONSTRAINT IF EXISTS unique_vdc_name_per_org;
ALTER TABLE vdcs ADD CONSTRAINT unique_vdc_name_per_org UNIQUE (organization_id, name);