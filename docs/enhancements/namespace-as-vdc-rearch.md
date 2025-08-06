# Re-Architecture Plan: Organizations and VDCs

## Overview

This plan outlines the re-architecture of how Organizations and Virtual Data Centers (VDCs) are represented in the SSVIRT system. The key changes are:

1. **Organizations**: Will only exist in PostgreSQL database (no Kubernetes namespace)
2. **VDCs**: Will be represented as Kubernetes Namespaces with specific naming and labeling

## Current State

- Organizations → OpenShift Namespaces + PostgreSQL metadata
- VDCs → Resource Quotas + Network Policies + PostgreSQL configuration within org namespaces

## Target State

- Organizations → PostgreSQL metadata only
- VDCs → Kubernetes Namespaces with naming pattern `vdc-{org-name}-{vdc-name}`

## Implementation Plan

### Phase 1: Database Schema Updates

#### 1.1 Update Organizations Table
- Remove any references to Kubernetes namespace names from organizations table
- Ensure organizations are purely logical entities in PostgreSQL
- Update organization creation logic to not create Kubernetes namespaces

#### 1.2 Update VDCs Table
- Add `namespace_name` field to track the Kubernetes namespace for each VDC
- Implement naming convention: `vdc-{org_name}-{vdc_name}`
- Add validation to ensure namespace names are valid Kubernetes identifiers

```sql
ALTER TABLE vdcs ADD COLUMN namespace_name VARCHAR(253) UNIQUE;
-- Add index for performance
CREATE INDEX idx_vdcs_namespace_name ON vdcs(namespace_name);
```

### Phase 2: VDC Namespace Management

#### 2.1 Namespace Creation Logic
Update VDC creation to:
- Generate namespace name using pattern `vdc-{org-name}-{vdc-name}`
- Create Kubernetes namespace with required labels:
  - `ssvirt.io/organization`: Organization name/ID
  - `ssvirt.io/vdc`: VDC name/ID
  - `k8s.ovn.org/primary-user-defined-network`: (empty value or specific network name)
- Apply resource quotas and network policies to the VDC namespace
- Store namespace name in VDCs table

#### 2.2 Namespace Labeling Strategy
```yaml
metadata:
  name: "vdc-{org-name}-{vdc-name}"
  labels:
    ssvirt.io/organization: "{org-name}"
    ssvirt.io/organization-id: "{org-uuid}"
    ssvirt.io/vdc: "{vdc-name}"
    ssvirt.io/vdc-id: "{vdc-uuid}"
    k8s.ovn.org/primary-user-defined-network: ""
    app.kubernetes.io/managed-by: "ssvirt"
```

#### 2.3 Namespace Cleanup
- Implement VDC deletion to remove corresponding Kubernetes namespace
- Add cleanup logic for orphaned namespaces
- Ensure proper garbage collection of VMs and resources within VDC namespaces

### Phase 3: Controller Updates

#### 3.1 Organization Controller
- Remove namespace creation/deletion logic
- Focus only on PostgreSQL operations for organizations
- Update organization validation to not check Kubernetes namespace availability

#### 3.2 VDC Controller
- Implement namespace lifecycle management
- Watch for VDC creation/update/deletion events
- Reconcile namespace state with VDC configuration
- Apply resource quotas, network policies, and RBAC to VDC namespaces

#### 3.3 VM Controller
- Update VM placement logic to use VDC namespaces instead of organization namespaces
- Ensure VMs are created in the correct `vdc-{org-name}-{vdc-name}` namespace
- Update VM queries to search across VDC namespaces within an organization

### Phase 4: API Updates

#### 4.1 Organization API Endpoints
- Update organization creation to not create Kubernetes resources
- Remove organization namespace references from API responses
- Update organization listing to show VDCs and their namespaces

#### 4.2 VDC API Endpoints
- Update VDC creation to trigger namespace creation
- Add namespace information to VDC API responses
- Implement VDC status reporting based on namespace health

#### 4.3 VM API Endpoints
- Update VM creation to target VDC namespaces
- Update VM listing and filtering to work with new namespace structure
- Ensure VM operations work within VDC namespace context

### Phase 5: RBAC and Security Updates

#### 5.1 Service Account Updates
- Update service accounts to work with VDC namespaces
- Ensure proper permissions for cross-namespace operations (organization-level queries)
- Update cluster roles and role bindings

#### 5.2 Network Policies
- Update network policies to work with VDC-level isolation
- Ensure proper inter-VDC communication rules within organizations
- Implement organization-level network isolation if required

### Phase 6: Helm Chart Updates

#### 6.1 RBAC Templates
- Update cluster roles to include namespace management permissions
- Add permissions for labeling and managing VDC namespaces
- Ensure controllers can watch and manage namespaces cluster-wide

#### 6.2 Controller Configuration
- Update controller deployment with new namespace management logic
- Add environment variables for namespace naming patterns
- Update health checks to validate namespace management capabilities

### Phase 7: Documentation Updates

#### 7.1 Administrator Setup Guide
- Update organization creation procedures
- Document new VDC namespace creation process
- Update troubleshooting sections for new namespace structure
- Revise RBAC configuration examples

#### 7.2 User Guide
- Update API examples to reflect new namespace structure
- Clarify organization vs VDC concepts
- Update VM creation examples with new namespace targeting

#### 7.3 Architecture Documentation
- Update plan.md with new organization/VDC mapping
- Document namespace naming conventions
- Update database schema documentation

### Phase 8: Testing and Validation

#### 8.1 Integration Tests
- Test organization creation (database-only)
- Test VDC creation with namespace generation
- Test VM creation in VDC namespaces
- Test resource quota enforcement at VDC level

#### 8.2 Performance Tests
- Validate performance with new namespace structure
- Test cross-namespace queries for organization-level operations
- Benchmark namespace creation/deletion performance

## Implementation Order

1. **Phase 1**: Database schema updates
2. **Phase 2**: VDC namespace management logic
3. **Phase 3**: Controller updates
4. **Phase 4**: API endpoint updates
5. **Phase 5**: RBAC and security updates
6. **Phase 6**: Helm chart updates
7. **Phase 7**: Documentation updates
8. **Phase 8**: Testing and validation

## Benefits of New Architecture

1. **Clearer Separation**: Organizations are purely logical, VDCs map to physical Kubernetes resources
2. **Better Resource Management**: VDC-level resource quotas and policies are more intuitive
3. **Improved Multi-tenancy**: Each VDC gets its own isolated namespace
4. **Simplified Operations**: Easier to manage VDC-level permissions and resources
5. **Network Isolation**: Better support for VDC-level networking with User Defined Networks

## Risks and Considerations

1. **Namespace Limits**: Kubernetes clusters have limits on number of namespaces
2. **RBAC Complexity**: Cross-namespace operations may require more complex permissions
3. **Performance**: Organization-level queries may need to aggregate across multiple namespaces

## Success Criteria

- [ ] Organizations exist only in PostgreSQL
- [ ] VDCs create corresponding Kubernetes namespaces with proper naming
- [ ] Namespaces have required labels for organization tracking and networking
- [ ] VMs are created in VDC namespaces
- [ ] Resource quotas and policies work at VDC level
- [ ] All API endpoints work with new architecture
- [ ] Documentation is updated to reflect new model
- [ ] Performance is maintained or improved