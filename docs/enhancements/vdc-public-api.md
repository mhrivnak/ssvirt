# VDC Public API Enhancement

## Overview

This enhancement proposes adding a new read-only VDC API for non-admin users at `/cloudapi/1.0.0/vdcs`. This API will provide standard CloudAPI-compliant endpoints for listing and retrieving VDCs without requiring System Administrator privileges, enabling regular users to view VDCs they have access to.

## Background

Currently, VDC access is limited to System Administrators through the `/api/admin/org/{orgId}/vdcs` endpoints. This creates a significant limitation for regular users who need to view VDCs as part of their normal workflow, such as when deploying vApps or managing resources within a VDC.

## Goals

1. **Enable non-admin VDC access**: Allow regular authenticated users to view VDCs without requiring System Administrator privileges
2. **CloudAPI compliance**: Implement standard CloudAPI endpoints following VMware Cloud Director patterns
3. **Consistent data model**: Reuse the existing VDC data model and response format from the admin API
4. **Standard pagination**: Implement consistent pagination matching other CloudAPI endpoints
5. **Security**: Ensure users can only see VDCs they have access to based on their organization membership

## Non-Goals

- Modifying, creating, or deleting VDCs (remains admin-only)
- Changing the existing admin VDC API
- Organization-scoped VDC listing (users see all accessible VDCs across organizations)
- Backward compatibility concerns (new endpoints)

## Proposed API Endpoints

### 1. List VDCs
**Endpoint**: `GET /cloudapi/1.0.0/vdcs`
**Description**: Returns a paginated list of VDCs accessible to the authenticated user
**Authentication**: JWT token required
**Authorization**: Any authenticated user

**Query Parameters**:
- `page` (optional): Page number (default: 1)
- `pageSize` (optional): Number of items per page (default: 25, max: 100)

**Response**: `200 OK`
```json
{
  "resultTotal": 42,
  "pageCount": 2,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
      "name": "Production VDC",
      "description": "Primary production environment",
      "allocationModel": "PayAsYouGo",
      "computeCapacity": {
        "cpu": {
          "allocated": 2000,
          "limit": 4000,
          "units": "MHz"
        },
        "memory": {
          "allocated": 2048,
          "limit": 4096,
          "units": "MB"
        }
      },
      "providerVdc": {
        "id": "urn:vcloud:providervdc:87654321-4321-4321-4321-cba987654321"
      },
      "nicQuota": 100,
      "networkQuota": 50,
      "vdcStorageProfiles": {},
      "isThinProvision": false,
      "isEnabled": true
    }
  ]
}
```

**Error Responses**:
- `401 Unauthorized`: Missing or invalid authentication token
- `500 Internal Server Error`: Database or server errors

### 2. Get VDC
**Endpoint**: `GET /cloudapi/1.0.0/vdcs/{vdc_id}`
**Description**: Returns details of a specific VDC by ID
**Authentication**: JWT token required
**Authorization**: User must have access to the VDC

**Path Parameters**:
- `vdc_id`: VDC URN identifier (e.g., `urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc`)

**Response**: `200 OK`
```json
{
  "id": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "name": "Production VDC",
  "description": "Primary production environment",
  "allocationModel": "PayAsYouGo",
  "computeCapacity": {
    "cpu": {
      "allocated": 2000,
      "limit": 4000,
      "units": "MHz"
    },
    "memory": {
      "allocated": 2048,
      "limit": 4096,
      "units": "MB"
    }
  },
  "providerVdc": {
    "id": "urn:vcloud:providervdc:87654321-4321-4321-4321-cba987654321"
  },
  "nicQuota": 100,
  "networkQuota": 50,
  "vdcStorageProfiles": {},
  "isThinProvision": false,
  "isEnabled": true
}
```

**Error Responses**:
- `400 Bad Request`: Invalid VDC URN format
- `401 Unauthorized`: Missing or invalid authentication token
- `403 Forbidden`: User does not have access to this VDC
- `404 Not Found`: VDC does not exist
- `500 Internal Server Error`: Database or server errors

## Data Model

### VDC Response Structure
The VDC response will use the existing `VDCResponse` structure from the admin API to ensure consistency:

```go
type VDCResponse struct {
    ID                 string                    `json:"id"`
    Name               string                    `json:"name"`
    Description        string                    `json:"description"`
    AllocationModel    models.AllocationModel    `json:"allocationModel"`
    ComputeCapacity    models.ComputeCapacity    `json:"computeCapacity"`
    ProviderVdc        models.ProviderVdc        `json:"providerVdc"`
    NicQuota           int                       `json:"nicQuota"`
    NetworkQuota       int                       `json:"networkQuota"`
    VdcStorageProfiles models.VdcStorageProfiles `json:"vdcStorageProfiles"`
    IsThinProvision    bool                      `json:"isThinProvision"`
    IsEnabled          bool                      `json:"isEnabled"`
}
```

### Access Control Strategy
Users will have access to VDCs based on their organization membership:
1. **Organization Members**: Users can see VDCs that belong to their organization
2. **Multi-org Users**: Users associated with multiple organizations can see VDCs from all their organizations
3. **System Administrators**: Can see all VDCs (but would typically use admin endpoints)

## Implementation Plan

### Phase 1: Core Infrastructure
1. **Create new handler**: `VDCPublicHandlers` for non-admin VDC operations
2. **Add repository methods**: 
   - `ListAccessibleVDCs(userID, limit, offset)` - get VDCs accessible to user with pagination
   - `CountAccessibleVDCs(userID)` - count VDCs accessible to user
   - `GetAccessibleVDC(userID, vdcID)` - get specific VDC if user has access
3. **Add routes**: Register new CloudAPI endpoints in server configuration
4. **Access control**: Implement user-to-VDC access logic based on organization membership

### Phase 2: Handler Implementation
1. **List VDCs handler**: Implement pagination, filtering, and response formatting
2. **Get VDC handler**: Implement access control and error handling
3. **Error handling**: Consistent error responses following CloudAPI patterns
4. **Response formatting**: Reuse existing `VDCResponse` structure and conversion logic

### Phase 3: Testing & Documentation
1. **Unit tests**: Comprehensive test coverage for new handlers and repository methods
2. **Integration tests**: End-to-end testing of CloudAPI endpoints
3. **Access control tests**: Verify users can only access authorized VDCs
4. **Documentation**: API documentation and user guide

### File Changes

#### New Files
- `pkg/api/handlers/vdc_public.go` - New public VDC handlers
- `test/unit/vdc_public_api_test.go` - Unit tests for public VDC API
- `docs/vdc-public-api-guide.md` - User documentation

#### Modified Files
- `pkg/api/server.go` - Add new CloudAPI routes
- `pkg/database/repositories/vdc.go` - Add access-controlled repository methods
- `test/unit/api_test.go` - Update test setup if needed

## Security Considerations

### Authentication
- All endpoints require valid JWT authentication
- No anonymous access allowed

### Authorization
- Users can only access VDCs from organizations they belong to
- No privilege escalation - cannot access admin-only VDC operations
- Read-only access only - no create, update, or delete operations

### Data Exposure
- Same data model as admin API but filtered by user access
- No sensitive internal fields exposed (namespace, timestamps hidden)
- Consistent with existing CloudAPI data exposure patterns

## Testing Strategy

### Unit Tests
1. **Handler tests**: Mock repository calls and test response formatting
2. **Repository tests**: Test access control logic with various user scenarios
3. **Pagination tests**: Verify correct pagination behavior
4. **Error handling tests**: Test all error conditions and response formats

### Integration Tests
1. **End-to-end API tests**: Full request/response cycle testing
2. **Access control tests**: Multi-user scenarios with different organization memberships
3. **Pagination integration**: Test pagination with real data
4. **Error scenario tests**: Invalid URNs, unauthorized access, missing resources

### Test Data Scenarios
1. **Single organization user**: User belongs to one organization with multiple VDCs
2. **Multi-organization user**: User belongs to multiple organizations
3. **No VDC access**: User belongs to organization with no VDCs
4. **Mixed permissions**: Users with different organization memberships

## Documentation Notes

### VMware Cloud Director API Inconsistency
The VMware Cloud Director OpenAPI documentation at https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/vdc/ shows a different data schema for VDCs than what we implement. However, the official documentation is inconsistent across different sections, and our implementation follows the more complete and practical data model used in the admin API.

**Key Differences**:
- Our model includes detailed `computeCapacity` with CPU and memory allocation/limits
- Our model includes `providerVdc` references for infrastructure mapping
- Our model includes quotas (`nicQuota`, `networkQuota`) for resource management
- Our model uses consistent URN-based identifiers throughout

We are maintaining our current data model because:
1. It provides more useful information for practical VDC management
2. It maintains consistency with our existing admin API
3. It aligns with real-world VDC usage patterns
4. The official documentation conflicts with itself in various sections

### Migration and Compatibility
This is a new API with no backward compatibility concerns. Existing admin endpoints remain unchanged, and new public endpoints are additive only.

## Success Metrics

1. **Functionality**: All endpoints work correctly with proper pagination and error handling
2. **Security**: Users can only access VDCs they're authorized to see
3. **Performance**: List operations perform well with pagination
4. **Consistency**: Response format matches existing CloudAPI patterns
5. **Documentation**: Clear API documentation and examples

## Timeline

- **Week 1**: Core infrastructure and repository methods
- **Week 2**: Handler implementation and basic testing
- **Week 3**: Comprehensive testing and access control validation
- **Week 4**: Documentation and integration testing

## Future Enhancements

While not part of this proposal, future enhancements could include:
1. **Advanced filtering**: Filter VDCs by allocation model, status, or organization
2. **Sorting options**: Sort VDCs by name, creation date, or usage
3. **Resource usage metrics**: Add real-time resource utilization data
4. **Search functionality**: Search VDCs by name or description
5. **Batch operations**: Retrieve multiple VDCs in a single request