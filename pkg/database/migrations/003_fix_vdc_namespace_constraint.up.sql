-- Fix VDC namespace unique constraint to work with soft deletes
-- Replace the simple unique constraint with a partial unique index

-- Drop the existing unique constraint on namespace
ALTER TABLE vdcs DROP CONSTRAINT IF EXISTS vdcs_namespace_key;

-- Drop the existing index if it exists  
DROP INDEX IF EXISTS idx_vdcs_namespace;

-- Create a partial unique index that only applies to non-deleted records
-- This allows deleted records to have duplicate namespaces while maintaining
-- uniqueness for active records
CREATE UNIQUE INDEX idx_vdcs_namespace_unique 
ON vdcs(namespace) 
WHERE deleted_at IS NULL;