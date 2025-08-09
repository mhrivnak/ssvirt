# VDC API VMware Cloud Director Compliance Enhancement

## Overview

This enhancement implements the VMware Cloud Director (VCD) compliant VDC API to replace the current legacy VDC endpoints. The new API will conform exactly to the VMware Cloud Director API specification and provide full CRUD operations for Virtual Data Centers (VDCs) with proper authorization and data modeling.

## Goals

1. **Complete VCD API Compliance**: Implement VDC endpoints that match VMware Cloud Director API specification exactly
2. **Proper Authorization**: Restrict VDC management to System Administrators only  
3. **Enhanced Data Model**: Support the full VCD VDC data structure including compute capacity, quotas, and storage profiles
4. **Pagination Support**: Follow the same pagination pattern as other CloudAPI endpoints
5. **Legacy Cleanup**: Remove all old VDC API endpoints and handlers

## Current State Analysis

### Existing VDC Model
The current VDC model (`pkg/database/models/vdc.go`) has:
- Basic fields: ID, Name, OrganizationID, AllocationModel
- Resource limits: CPULimit, MemoryLimitMB, StorageLimitMB  
- Kubernetes integration: Namespace field for K8s namespace mapping
- Relationships: Organization and VApps

### Current API State
- Legacy endpoints are commented out in `pkg/api/server.go`
- No current VDC handlers in `pkg/api/handlers/`
- Repository exists but doesn't support pagination or VCD-compliant queries

## Target VCD Data Model

Based on VMware Cloud Director API specification, VDCs should have:

```json
{
  "name": "your-new-vdc-name",
  "description": "Description of the new VDC", 
  "allocationModel": "Flex",
  "computeCapacity": {
    "cpu": {
      "allocated": 10000,
      "limit": 20000,
      "units": "MHz"
    },
    "memory": {
      "allocated": 16384,
      "limit": 32768,
      "units": "MB"
    }
  },
  "providerVdc": {
    "id": "<id-of-provider-vdc>"
  },
  "nicQuota": 100,
  "networkQuota": 50,
  "vdcStorageProfiles": {
    "providerVdcStorageProfile": {
      "id": "<id-of-provider-storage-profile>",
      "limit": 1024000,
      "units": "MB", 
      "default": true
    }
  },
  "isThinProvision": false,
  "isEnabled": true
}
```

## API Specification

### Base Path
All VDC endpoints will be served under:
```
/api/admin/org/{orgId}/vdcs
```

### Endpoints

| Method | Path | Description | Authorization |
|--------|------|-------------|---------------|
| GET | `/api/admin/org/{orgId}/vdcs` | List VDCs in organization | System Admin |
| POST | `/api/admin/org/{orgId}/vdcs` | Create new VDC | System Admin |
| GET | `/api/admin/org/{orgId}/vdcs/{vdcId}` | Get VDC details | System Admin |
| PUT | `/api/admin/org/{orgId}/vdcs/{vdcId}` | Update VDC | System Admin |
| DELETE | `/api/admin/org/{orgId}/vdcs/{vdcId}` | Delete VDC | System Admin |

### Request/Response Patterns

#### List VDCs (GET /api/admin/org/{orgId}/vdcs)
- **Query Parameters**: `page`, `page_size`, `filter`, `sort`
- **Response**: Paginated list following `types.Page[VDC]` pattern
- **Status Codes**: 200 (success), 401 (unauthorized), 403 (forbidden), 404 (org not found)

#### Get VDC (GET /api/admin/org/{orgId}/vdcs/{vdcId})
- **Path Parameters**: `orgId` (organization URN), `vdcId` (VDC URN)
- **Response**: Complete VDC object with all fields
- **Status Codes**: 200 (success), 401 (unauthorized), 403 (forbidden), 404 (not found)

#### Create VDC (POST /api/admin/org/{orgId}/vdcs)
- **Request Body**: VDC creation object (subset of full VDC model)
- **Response**: Created VDC object with generated ID and timestamps
- **Status Codes**: 201 (created), 400 (bad request), 401 (unauthorized), 403 (forbidden), 409 (conflict)

#### Update VDC (PUT /api/admin/org/{orgId}/vdcs/{vdcId})
- **Request Body**: Complete VDC object with modifications
- **Response**: Updated VDC object
- **Status Codes**: 200 (success), 400 (bad request), 401 (unauthorized), 403 (forbidden), 404 (not found)

#### Delete VDC (DELETE /api/admin/org/{orgId}/vdcs/{vdcId})
- **Response**: Empty body
- **Status Codes**: 204 (no content), 401 (unauthorized), 403 (forbidden), 404 (not found), 409 (conflict if has dependent resources)

## Implementation Plan

### Phase 1: Data Model Updates

#### 1.1 Update VDC Model (`pkg/database/models/vdc.go`)
- [ ] Add new fields to match VCD specification:
  - `Description string`
  - `ComputeCapacity` struct for CPU/Memory with allocated/limit/units
  - `ProviderVdc` EntityRef
  - `NicQuota int`
  - `NetworkQuota int`  
  - `VdcStorageProfiles` (empty for now)
  - `IsThinProvision bool`
  - `IsEnabled bool` (rename from `Enabled`)
- [ ] Update `AllocationModel` to support "Flex" value
- [ ] Add proper JSON tags for VCD compliance
- [ ] Update URN generation to use VDC-specific prefix
- [ ] Keep existing Kubernetes namespace integration

#### 1.2 Update VDC Repository (`pkg/database/repositories/vdc.go`)
- [ ] Add pagination support: `ListByOrgWithPagination(orgID string, limit, offset int) ([]VDC, int64, error)`
- [ ] Add organization filtering: `CountByOrganization(orgID string) (int64, error)`
- [ ] Add VCD-compliant methods:
  - `GetByURN(urn string) (*VDC, error)`
  - `GetByOrgAndVDCURN(orgURN, vdcURN string) (*VDC, error)`
- [ ] Update existing methods to handle new fields
- [ ] Add validation for required fields

### Phase 2: Handler Implementation

#### 2.1 Create VDC Handlers (`pkg/api/handlers/vdcs.go`)
- [ ] Implement `VDCHandlers` struct with repository dependency
- [ ] Implement `NewVDCHandlers(vdcRepo, orgRepo)` constructor
- [ ] Implement all CRUD operations:
  - `ListVDCs(c *gin.Context)` - GET /api/admin/org/{orgId}/vdcs
  - `GetVDC(c *gin.Context)` - GET /api/admin/org/{orgId}/vdcs/{vdcId}
  - `CreateVDC(c *gin.Context)` - POST /api/admin/org/{orgId}/vdcs
  - `UpdateVDC(c *gin.Context)` - PUT /api/admin/org/{orgId}/vdcs/{vdcId}
  - `DeleteVDC(c *gin.Context)` - DELETE /api/admin/org/{orgId}/vdcs/{vdcId}
- [ ] Add proper error handling and validation
- [ ] Add URN validation for path parameters
- [ ] Add organization existence validation

#### 2.2 Authorization Integration
- [ ] Create middleware or helper for System Administrator role validation
- [ ] Ensure all VDC endpoints require System Administrator role
- [ ] Add organization access validation (user can access the specified org)

### Phase 3: API Integration

#### 3.1 Update Server Routes (`pkg/api/server.go`)
- [ ] Add VDC handler initialization in `NewServer()`
- [ ] Add VDC routes under `/api/admin/org/:orgId/vdcs`
- [ ] Apply System Administrator authorization middleware
- [ ] Remove commented legacy VDC endpoints

#### 3.2 Request/Response Types
- [ ] Create VDC request types in `pkg/api/types/`:
  - `VDCCreateRequest`
  - `VDCUpdateRequest`  
- [ ] Add VCD-compliant response structures
- [ ] Ensure proper EntityRef handling for related objects

### Phase 4: Testing

#### 4.1 Unit Tests (`test/unit/vdc_api_test.go`)
- [ ] Test all CRUD operations with proper authorization
- [ ] Test pagination with various page sizes and offsets
- [ ] Test error scenarios:
  - Invalid URNs
  - Non-existent organizations/VDCs
  - Unauthorized access (non-admin users)
  - Validation failures
- [ ] Test organization filtering
- [ ] Test VDC deletion with dependent vApps (should fail)

#### 4.2 Integration Tests
- [ ] Test complete VDC lifecycle: create -> read -> update -> delete
- [ ] Test VDC creation triggers Kubernetes namespace creation
- [ ] Test VDC updates preserve namespace integration
- [ ] Test VDC deletion cleans up Kubernetes resources

### Phase 5: Documentation and Cleanup

#### 5.1 Remove Legacy Code
- [ ] Remove any existing VDC handler code from `pkg/api/handlers.go`
- [ ] Remove legacy VDC tests from `test/unit/api_test.go`
- [ ] Update `pkg/api/README.md` to document new VDC endpoints

#### 5.2 Update Documentation
- [ ] Update API documentation in `pkg/api/README.md`
- [ ] Update `plan.md` with new VDC API details
- [ ] Add VDC API examples to user guide

## Database Schema Changes

### Required Model Changes
No database migrations required - only model struct updates:

```go
type VDC struct {
    // Existing fields (keep)
    ID              string          `gorm:"type:varchar(255);primaryKey" json:"id"`
    Name            string          `gorm:"not null" json:"name"`
    OrganizationID  string          `gorm:"type:varchar(255);not null;index" json:"-"`
    Namespace       string          `gorm:"uniqueIndex;size:253" json:"-"`
    CreatedAt       time.Time       `json:"createdAt"`
    UpdatedAt       time.Time       `json:"updatedAt"`
    DeletedAt       gorm.DeletedAt  `gorm:"index" json:"-"`
    
    // New VCD-compliant fields
    Description     string          `json:"description"`
    AllocationModel AllocationModel `gorm:"type:varchar(20);check:allocation_model IN ('PayAsYouGo', 'AllocationPool', 'ReservationPool', 'Flex')" json:"allocationModel"`
    ComputeCapacity ComputeCapacity `gorm:"embedded" json:"computeCapacity"`
    ProviderVdc     EntityRef       `gorm:"embedded;embeddedPrefix:provider_vdc_" json:"providerVdc"`
    NicQuota        int             `gorm:"default:100" json:"nicQuota"`
    NetworkQuota    int             `gorm:"default:50" json:"networkQuota"`
    IsThinProvision bool            `gorm:"default:false" json:"isThinProvision"`
    IsEnabled       bool            `gorm:"default:true" json:"isEnabled"`
    
    // Relationships
    Organization *Organization `gorm:"foreignKey:OrganizationID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
    VApps        []VApp        `gorm:"foreignKey:VDCID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

type ComputeCapacity struct {
    CPU    ComputeResource `gorm:"embedded;embeddedPrefix:cpu_" json:"cpu"`
    Memory ComputeResource `gorm:"embedded;embeddedPrefix:memory_" json:"memory"`
}

type ComputeResource struct {
    Allocated int    `json:"allocated"`
    Limit     int    `json:"limit"`
    Units     string `json:"units"`
}
```

## Error Handling

### Standard Error Responses
All endpoints will return consistent error responses:

```json
{
  "error": "Error Type",
  "message": "Human readable error message",
  "details": "Additional error details (optional)"
}
```

### Common Error Scenarios
- **400 Bad Request**: Invalid request body, invalid URN format
- **401 Unauthorized**: Missing or invalid authentication
- **403 Forbidden**: User is not System Administrator
- **404 Not Found**: Organization or VDC not found
- **409 Conflict**: VDC name conflicts, deletion blocked by dependencies

## Backwards Compatibility

**This enhancement breaks backwards compatibility intentionally.**

All legacy VDC endpoints will be removed:
- ~~GET /api/vdc/{vdc-id}~~
- ~~GET /api/org/{org-id}/vdcs/query~~
- ~~POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate~~

New endpoints are served under `/api/admin/org/{orgId}/vdcs` path.

## Security Considerations

1. **Authorization**: Only System Administrators can manage VDCs
2. **Organization Validation**: Ensure users can only access VDCs in organizations they have access to  
3. **URN Validation**: Validate all URN parameters to prevent injection attacks
4. **Resource Validation**: Validate compute capacity and quota limits are reasonable
5. **Cascade Protection**: Prevent VDC deletion if it contains active vApps

## Testing Strategy

### Test Categories
1. **Authorization Tests**: Verify System Administrator requirement
2. **CRUD Tests**: Test all operations with valid data
3. **Validation Tests**: Test input validation and error handling  
4. **Pagination Tests**: Test various page sizes and navigation
5. **Integration Tests**: Test Kubernetes namespace lifecycle
6. **Performance Tests**: Test pagination with large datasets

### Test Data Requirements
- Multiple test organizations with different access levels
- Test users with and without System Administrator role
- VDCs with various configurations (different allocation models, quotas)
- VDCs with and without dependent vApps

## Migration Plan

### Phase 1: Implementation (2-3 days)
- Implement data model updates
- Create VDC handlers with full CRUD operations
- Add authorization middleware
- Update server routes

### Phase 2: Testing (1-2 days)  
- Comprehensive unit tests
- Integration tests
- Performance testing with pagination

### Phase 3: Documentation and Cleanup (1 day)
- Remove legacy code
- Update documentation
- API reference updates

### Total Estimated Effort: 4-6 days

## Success Criteria

1. ✅ All VDC endpoints conform to VMware Cloud Director API specification
2. ✅ Only System Administrators can access VDC management endpoints
3. ✅ Pagination works correctly with configurable page sizes
4. ✅ All CRUD operations work with proper validation and error handling
5. ✅ VDC creation/deletion properly manages Kubernetes namespaces
6. ✅ All legacy VDC code is removed
7. ✅ 100% test coverage for new VDC endpoints
8. ✅ API documentation is complete and accurate

## Future Enhancements

1. **Storage Profiles**: Implement vdcStorageProfiles data model and management
2. **Provider VDC Integration**: Add actual provider VDC management
3. **Advanced Quotas**: Implement network and NIC quota enforcement
4. **VDC Templates**: Support for VDC template-based provisioning
5. **Multi-site Support**: Extend for multi-cluster VDC distribution via ACM

## References

- [VMware Cloud Director OpenAPI](https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/)
- [VMware Cloud Director Documentation](https://docs.vmware.com/en/VMware-Cloud-Director/)
- [Current Implementation Plan](./plan.md)
- [Sessions API Enhancement](./sessions-api-vcd-compliance.md)