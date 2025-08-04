-- Create UUID extension if not exists
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    namespace VARCHAR(255) UNIQUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_organizations_name ON organizations(name);
CREATE INDEX IF NOT EXISTS idx_organizations_namespace ON organizations(namespace);

-- Virtual Data Centers table
CREATE TABLE IF NOT EXISTS vdcs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    allocation_model VARCHAR(50), -- PayAsYouGo, AllocationPool, ReservationPool
    cpu_limit INTEGER,
    memory_limit_mb INTEGER,
    storage_limit_mb INTEGER,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vdcs_deleted_at ON vdcs(deleted_at);
CREATE INDEX IF NOT EXISTS idx_vdcs_organization_id ON vdcs(organization_id);

-- Catalogs table
CREATE TABLE IF NOT EXISTS catalogs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    description TEXT,
    is_shared BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_catalogs_deleted_at ON catalogs(deleted_at);
CREATE INDEX IF NOT EXISTS idx_catalogs_organization_id ON catalogs(organization_id);

-- vApp Templates table
CREATE TABLE IF NOT EXISTS vapp_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    catalog_id UUID NOT NULL REFERENCES catalogs(id) ON DELETE CASCADE,
    description TEXT,
    vm_instance_type VARCHAR(255), -- OpenShift VirtualMachineInstanceType
    os_type VARCHAR(100),
    cpu_count INTEGER,
    memory_mb INTEGER,
    disk_size_gb INTEGER,
    template_data JSONB, -- Template configuration
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vapp_templates_deleted_at ON vapp_templates(deleted_at);
CREATE INDEX IF NOT EXISTS idx_vapp_templates_catalog_id ON vapp_templates(catalog_id);

-- vApps (instances) table
CREATE TABLE IF NOT EXISTS vapps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    vdc_id UUID NOT NULL REFERENCES vdcs(id) ON DELETE CASCADE,
    template_id UUID REFERENCES vapp_templates(id) ON DELETE SET NULL,
    status VARCHAR(50), -- RESOLVED, DEPLOYED, SUSPENDED, etc.
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vapps_deleted_at ON vapps(deleted_at);
CREATE INDEX IF NOT EXISTS idx_vapps_vdc_id ON vapps(vdc_id);
CREATE INDEX IF NOT EXISTS idx_vapps_template_id ON vapps(template_id);

-- Virtual Machines (metadata) table
CREATE TABLE IF NOT EXISTS vms (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    vapp_id UUID NOT NULL REFERENCES vapps(id) ON DELETE CASCADE,
    vm_name VARCHAR(255), -- OpenShift VM resource name
    namespace VARCHAR(255), -- OpenShift namespace
    status VARCHAR(50),
    cpu_count INTEGER,
    memory_mb INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vms_deleted_at ON vms(deleted_at);
CREATE INDEX IF NOT EXISTS idx_vms_vapp_id ON vms(vapp_id);
CREATE INDEX IF NOT EXISTS idx_vms_vm_name ON vms(vm_name);
CREATE INDEX IF NOT EXISTS idx_vms_namespace ON vms(namespace);

-- User/Role mappings table
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role VARCHAR(100) NOT NULL, -- OrgAdmin, VAppUser, VAppAuthor, etc.
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_roles_deleted_at ON user_roles(deleted_at);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_organization_id ON user_roles(organization_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_unique ON user_roles(user_id, organization_id, role) WHERE deleted_at IS NULL;