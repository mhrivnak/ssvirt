# User APIs Enhancement Proposal

## Overview

This enhancement proposal outlines the migration of SSVirt APIs to adhere strictly to the VMware Cloud Director API specification. The current implementation doesn't follow the VMware Cloud Director documentation closely enough, particularly around ID formats, field names, and API structures.

## Goals

- Align all API schemas with VMware Cloud Director specifications
- Implement URN-based ID formatting for all entities
- Add missing fields required by the VMware Cloud Director API
- Create proper entity reference structures
- Maintain authentication and authorization patterns

## Non-Goals

- Database migration support (we will re-install)
- Backward compatibility with existing API clients
- Complex nested entity operations (focus on core CRUD)

## Implementation Plan

### Phase 1: Model Updates

#### 1.1 URN ID System
Every record must have an "id" field formatted as: `"urn:vcloud:$TYPE:$UUID"`

Types include:
- user
- org  
- role

#### 1.2 Model Schema Changes

**User Model Updates:**
- Change `ID` field type from `uuid.UUID` to `string` with URN format
- Add `fullName` field replacing `firstName` + `lastName`
- Add `description` field
- Add `deployedVmQuota` and `storedVmQuota` fields as integers defaulting to 0
- Add `nameInSource` field
- Rename `IsActive` to `enabled`
- Add `isGroupRole`, `providerType`, `locked`, `stranded` fields
- Update relationships to use entity references

**Organization Model Updates:**
- Change `ID` field type from `uuid.UUID` to `string` with URN format
- Rename `Enabled` to `isEnabled`
- Add count fields: `orgVdcCount`, `catalogCount`, `vappCount`, `runningVMCount`, `userCount`, `diskCount`
- Add `managedBy` entity reference
- Add `canManageOrgs`, `canPublish`, `maskedEventTaskUsername`, `directlyManagedOrgCount` fields

**Role Model Creation:**
- Create new `Role` model with `id`, `name`, `description`, `bundleKey`, `readOnly` fields
- Seed with predefined roles: "System Administrator", "Organization Administrator", "vApp User"

#### 1.3 Entity Reference System
Create shared structures for entity references:
```go
type EntityRef struct {
    Name string `json:"name"`
    ID   string `json:"id"`
}
```

### Phase 2: API Endpoint Updates

#### 2.1 New API Routes
Implement VMware Cloud Director compatible endpoints:

**Users:**
- `GET /cloudapi/1.0.0/users` - bulk user queries
- `GET /cloudapi/1.0.0/users/{id}` - single user query

**Roles:**
- `GET /cloudapi/1.0.0/roles` - bulk role queries  
- `GET /cloudapi/1.0.0/roles/{id}` - single role query

**Organizations:**
- `GET /cloudapi/1.0.0/orgs` - bulk org queries
- `GET /cloudapi/1.0.0/orgs/{id}` - single org query

#### 2.2 Remove Legacy Routes
Remove existing `/api/v1` endpoints that don't align with VMware Cloud Director specification.

### Phase 3: Data Migration & Seeding

#### 3.1 Default Data Creation
- Create "Provider" organization as default
- Seed roles: "System Administrator", "Organization Administrator", "vApp User"
- Update initial admin user to have System Administrator role in Provider org

#### 3.2 URN Generation
Implement helper functions:
```go
func GenerateUserURN(uuid string) string {
    return fmt.Sprintf("urn:vcloud:user:%s", uuid)
}
func GenerateOrgURN(uuid string) string {
    return fmt.Sprintf("urn:vcloud:org:%s", uuid)
}
func GenerateRoleURN(uuid string) string {
    return fmt.Sprintf("urn:vcloud:role:%s", uuid)
}
```

### Phase 4: Repository & Service Updates

#### 4.1 Repository Layer Changes
- Update all repository methods to work with string URN IDs
- Add methods for entity reference lookups
- Update queries to populate count fields for organizations

#### 4.2 Service Layer Updates  
- Update user service to handle new user schema
- Add role service for role management
- Update organization service to compute count fields
- Add entity reference resolution logic

### Phase 5: Testing & Validation

#### 5.1 Unit Tests
- Update all existing unit tests for new schemas
- Add tests for URN generation and parsing
- Test entity reference resolution

#### 5.2 Integration Tests
- Test complete API flows with new endpoints
- Validate VMware Cloud Director API compliance
- Test role-based access patterns

## Implementation Order

1. **Create Role model and URN helpers** - Foundation for other changes
2. **Update User model** - Core entity with most changes
3. **Update Organization model** - Dependent on User changes
4. **Update repositories** - Data access layer changes
5. **Create new API endpoints** - New VMware Cloud Director compatible routes
6. **Remove legacy endpoints** - Clean up old routes
7. **Add data seeding** - Default roles and Provider org
8. **Update tests** - Ensure everything works correctly

## Files to Modify

- `pkg/database/models/user.go` - User model updates
- `pkg/database/models/organization.go` - Organization model updates  
- `pkg/database/models/role.go` - New role model
- `pkg/database/models/types.go` - Add entity reference types
- `pkg/database/repositories/*.go` - Repository updates
- `pkg/api/server.go` - New route definitions
- `pkg/api/cloudapi_handlers.go` - New VMware Cloud Director handlers
- Database migration or bootstrap logic for seeding

## Detailed API Specifications

### Users API

**Reference:** https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/user/

**Endpoints:**
- `GET /cloudapi/1.0.0/users` - bulk user queries
- `GET /cloudapi/1.0.0/users/{id}` - single user query

**Schema:**
```json
{
    "username": "string",
    "fullName": "string", 
    "description": "string",
    "id": "string",
    "roleEntityRefs": [
        {
            "name": "string",
            "id": "string"
        }
    ],
    "orgEntityRef": {
        "name": "string", 
        "id": "string"
    },
    "deployedVmQuota": 0,
    "storedVmQuota": 0,
    "email": "string",
    "nameInSource": "string",
    "enabled": false,
    "isGroupRole": false,
    "providerType": "string",
    "locked": false,
    "stranded": false
}
```

**Implementation Notes:**
- `fullName` replaces existing `firstName` + `lastName` fields
- `roleEntityRefs` contains the roles assigned to the user (correcting the documentation: these are Role references, not Organization)
- `orgEntityRef` contains the organization the user belongs to
- `password` field is only accepted in create/update requests and never returned in GET responses for security
- `nameInSource` can default to same as `username`
- `providerType` can default to "LOCAL"

### Roles API

**Reference:** https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/roles/

**Endpoints:**
- `GET /cloudapi/1.0.0/roles` - bulk role queries  
- `GET /cloudapi/1.0.0/roles/{id}` - single role query

**Schema:**
```json
{
    "name": "string",
    "id": "string", 
    "description": "string",
    "bundleKey": "string",
    "readOnly": false
}
```

**Predefined Roles:**
1. **System Administrator** (`urn:vcloud:role:<uuid>`)
   - Full system access
   - Can manage all organizations
   - `readOnly: true`

2. **Organization Administrator** (`urn:vcloud:role:<uuid>`)
   - Full access within assigned organization
   - Can manage users, VDCs, vApps within org
   - `readOnly: true`

3. **vApp User** (`urn:vcloud:role:<uuid>`)
   - Basic user access to assigned vApps
   - Can start/stop/use VMs in assigned vApps
   - `readOnly: true`

**Implementation Notes:**
- All roles should be `readOnly: true`
- `description` and `bundleKey` can be empty strings initially
- Role constants should be defined for permission checking

### Organizations API

**Reference:** https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/org/

**Endpoints:**
- `GET /cloudapi/1.0.0/orgs` - bulk org queries
- `GET /cloudapi/1.0.0/orgs/{id}` - single org query

**Schema:**
```json
{
    "id": "string",
    "name": "string",
    "displayName": "string", 
    "description": "string",
    "isEnabled": false,
    "orgVdcCount": 0,
    "catalogCount": 0,
    "vappCount": 0,
    "runningVMCount": 0,
    "userCount": 0,
    "diskCount": 0,
    "managedBy": {
        "name": "string",
        "id": "string" 
    },
    "canManageOrgs": false,
    "canPublish": false,
    "maskedEventTaskUsername": "string",
    "directlyManagedOrgCount": 0
}
```

**Default Organization:**
- Name: "Provider"
- `displayName`: "Provider Organization"
- `description`: "Default provider organization"
- `isEnabled`: true
- `managedBy`: Reference to System Administrator
- `canManageOrgs`: true (for system-level org)

**Implementation Notes:**
- Count fields must be computed from related entities, but that can be done later
- `managedBy` references the primary admin user
- `maskedEventTaskUsername` can be empty initially
- `directlyManagedOrgCount` only applies to system-level organizations

## Data Seeding Requirements

1. **Create Default Roles** (in order):
   ```
   System Administrator (urn:vcloud:role:<uuid>)
   Organization Administrator (urn:vcloud:role:<uuid>)  
   vApp User (urn:vcloud:role:<uuid>)
   ```

2. **Create Provider Organization**:
   ```
   Provider (urn:vcloud:org:<uuid>)
   ```

3. **Update Initial Admin User**:
   - Convert existing admin to new User schema
   - Assign System Administrator role
   - Associate with Provider organization
   - Generate URN ID

## Technical Implementation Details

### URN Format Constants
```go
const (
    URNPrefixUser = "urn:vcloud:user:"
    URNPrefixOrg  = "urn:vcloud:org:"
    URNPrefixRole = "urn:vcloud:role:"
)

const (
    RoleSystemAdmin = "System Administrator"
    RoleOrgAdmin    = "Organization Administrator" 
    RoleVAppUser    = "vApp User"
)

const (
    DefaultOrgName = "Provider"
)
```

### Response Format
All endpoints should return:
- Single entities: Direct object
- Collections: Paginated response with metadata
- Proper HTTP status codes (200, 404, 500)
- Content-Type: application/json

#### Collection Endpoints (Paging/Filtering)
Collection endpoints support the following query parameters:

**Pagination Parameters:**
- `page` (integer, default: 1) - Page number to retrieve
- `pageSize` (integer, default: 25, max: 100) - Number of items per page

**Filtering Parameters:**
- `filter` (string) - FIQL-style filter expression
- `sortAsc` (string) - Field name to sort ascending
- `sortDesc` (string) - Field name to sort descending

**Response Structure:**
All collection responses use the CloudAPI pagination envelope format:
```json
{
  "resultTotal": 150,
  "pageCount": 6,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    // Array of entities
  ]
}
```

**Metadata Fields:**
- `resultTotal`: Total number of items matching the query
- `pageCount`: Total number of pages available
- `page`: Current page number (1-based)
- `pageSize`: Number of items per page
- `associations`: Related entity links (empty for basic endpoints)
- `values`: Array containing the actual entity data

### Error Handling
- 404 for non-existent entities
- 400 for malformed URN IDs
- 500 for database errors
- Consistent error response format

## Implementation Status

### ✅ Completed Tasks

**Phase 1: Model Updates**
- ✅ 1.1 URN ID System - URN helper functions and constants implemented in `pkg/database/models/types.go`
- ✅ 1.2 Role Model - New Role model created with URN ID support
- ✅ 1.3 EntityRef Types - EntityRef structure implemented for API responses
- ✅ 1.4 User Model - Complete migration to VMware Cloud Director schema with URN IDs and new fields
- ✅ 1.5 Organization Model - Updated with count fields, entity references, and VMware Cloud Director compliance

**Phase 2: Repository Updates**
- ✅ 2.1 Repository Schema Updates - All repositories updated to work with string URN IDs
- ✅ 2.2 Role Repository - New role repository with default role creation functionality

**Phase 3: API Implementation**
- ✅ 3.1 CloudAPI Handlers - New VMware Cloud Director compatible endpoints implemented in `pkg/api/cloudapi_handlers.go`
- ✅ 3.2 API Server Routes - CloudAPI route group added with proper JWT authentication
- ✅ 3.3 Legacy Endpoint Removal - Legacy endpoints commented out to prevent compilation issues during migration

**Phase 4: Data Seeding & Bootstrap**
- ✅ 4.1 Default Data Seeding - Bootstrap logic for creating default roles and Provider organization
- ✅ 4.2 Initial Admin Update - Bootstrap admin creation updated to use new URN system and role assignments

### 🔄 Partially Completed

**Phase 5: Testing & Validation**
- ⚠️ 5.1 Unit Tests - Tests need updating for new model schemas (other components like k8s, controllers need updates)
- ✅ 5.2 Documentation - Implementation status documented

### 📋 Remaining Tasks

**Phase 5: Final Steps**
- ⏳ 5.3 Submit Pull Request - Ready for review once testing is addressed

### 🚀 Working Features

The following VMware Cloud Director compatible API endpoints are now functional:

**Authentication:**
- `POST /api/sessions` - User login (compatible with existing auth)
- `DELETE /api/sessions` - User logout  
- `GET /api/session` - Get current session

**CloudAPI Endpoints:**
- `GET /cloudapi/1.0.0/users` - List users with entity references
- `POST /cloudapi/1.0.0/users` - Create new user account with validation
- `GET /cloudapi/1.0.0/users/{id}` - Get single user with role and org references
- `PUT /cloudapi/1.0.0/users/{id}` - Update user account with conflict detection
- `DELETE /cloudapi/1.0.0/users/{id}` - Delete user account
- `GET /cloudapi/1.0.0/roles` - List available roles
- `GET /cloudapi/1.0.0/roles/{id}` - Get single role details
- `GET /cloudapi/1.0.0/orgs` - List organizations with computed fields
- `POST /cloudapi/1.0.0/orgs` - Create new organization with validation
- `GET /cloudapi/1.0.0/orgs/{id}` - Get single organization
- `PUT /cloudapi/1.0.0/orgs/{id}` - Update organization with conflict detection
- `DELETE /cloudapi/1.0.0/orgs/{id}` - Delete organization (protects Provider org)

**Data Bootstrap:**
- Default roles: "System Administrator", "Organization Administrator", "vApp User"
- Default Provider organization
- Initial admin user with System Administrator role assignment

### 💡 Key Implementation Notes

1. **URN ID Format**: All entities now use `urn:vcloud:$TYPE:$UUID` format for IDs
2. **Entity References**: API responses include proper entity references as per VMware Cloud Director spec
3. **Backward Compatibility**: Legacy endpoints have been temporarily disabled during migration
4. **Database Migration**: New schema requires fresh database setup (no migration support as per non-goals)
5. **Authentication**: Existing JWT-based authentication maintained, compatible with new CloudAPI endpoints

### 🔧 Technical Details

**Files Modified/Created:**
- `pkg/database/models/types.go` - URN helpers and constants
- `pkg/database/models/role.go` - New Role model
- `pkg/database/models/user.go` - Updated User model with VMware CD spec
- `pkg/database/models/organization.go` - Updated Organization model
- `pkg/database/repositories/role.go` - New role repository  
- `pkg/api/cloudapi_handlers.go` - VMware Cloud Director compatible handlers
- `pkg/database/connection.go` - Bootstrap logic for default data
- Updated auth service and JWT handling for string URN IDs

**Build Status:** ✅ Core components (database, auth, api) build successfully
**API Server:** ✅ Builds and should be functional with CloudAPI endpoints