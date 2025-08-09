# Catalog API VMware Cloud Director Compliance Enhancement

## Overview

This enhancement proposal outlines the changes needed to update the catalog API to conform exactly to the VMware Cloud Director (VCD) API specification. The current catalog implementation uses a legacy REST API pattern and needs to be replaced with VCD-compliant endpoints and data structures.

## Current State

### Current API Endpoints
- No active catalog endpoints (all commented out in server.go)
- Legacy structure planned for `/api/catalog/{catalog-id}` and `/api/org/{org-id}/catalogs/query`

### Current Data Model
```go
type Catalog struct {
    ID             string         `json:"id"`
    Name           string         `json:"name"`
    OrganizationID string         `json:"organization_id"`
    Description    string         `json:"description"`
    IsShared       bool           `json:"is_shared"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
    DeletedAt      gorm.DeletedAt `json:"deleted_at,omitempty"`
    // Relationships
    Organization  *Organization  `json:"organization,omitempty"`
    VAppTemplates []VAppTemplate `json:"vapp_templates,omitempty"`
}
```

## Target VCD API Specification

### New API Endpoints
1. `GET /cloudapi/1.0.0/catalogs/{catalogUrn}` - Get single catalog
2. `GET /cloudapi/1.0.0/catalogs` - List all catalogs with pagination
3. `POST /cloudapi/1.0.0/catalogs` - Create a new catalog
4. `DELETE /cloudapi/1.0.0/catalogs/{catalogUrn}` - Delete a catalog

**Note**: The CREATE and DELETE endpoints are not part of the official VMware Cloud Director documentation that we have access to. The implementation for these endpoints represents our best guess at how they would work based on standard REST API patterns and the structure observed in other VCD endpoints.

### VCD Catalog Data Structure
Based on VMware Cloud Director API documentation, the catalog object should include:

```json
{
    "id": "urn:vcloud:catalog:12345678-1234-1234-1234-123456789012",
    "name": "string",
    "description": "string",
    "org": {
        "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789012"
    },
    "isPublished": false,
    "isSubscribed": false,
    "creationDate": "2023-01-01T00:00:00.000Z",
    "numberOfVAppTemplates": 0,
    "numberOfMedia": 0,
    "catalogStorageProfiles": [],
    "publishConfig": {
        "isPublished": false
    },
    "subscriptionConfig": {
        "isSubscribed": false
    },
    "distributedCatalogConfig": {},
    "owner": {
        "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789012"
    },
    "isLocal": true,
    "version": 1
}
```

### Additional Endpoints (Best Guess Implementation)

Since the CREATE and DELETE endpoints are not documented in the available VMware Cloud Director API documentation, we will implement them based on standard REST patterns and VCD conventions:

#### POST /cloudapi/1.0.0/catalogs (Create Catalog)
- **Request Body**:
  ```json
  {
      "name": "My Catalog",
      "description": "Catalog description",
      "orgId": "urn:vcloud:org:12345678-1234-1234-1234-123456789012",
      "isPublished": false
  }
  ```
- **Response**: 201 Created with full catalog object (same as GET response)
- **Validation**: Name required, organization must exist
- **Defaults**: isPublished=false, isSubscribed=false, isLocal=true, version=1

#### DELETE /cloudapi/1.0.0/catalogs/{catalogUrn} (Delete Catalog)
- **Response**: 204 No Content on success
- **Validation**: Catalog must exist, no dependent vApp templates
- **Error**: 409 Conflict if catalog has dependent templates
- **Behavior**: Soft delete using GORM's DeletedAt field

## Implementation Plan

### 1. Update Database Model

#### 1.1 Modify Catalog Model (`pkg/database/models/catalog.go`)
- **Change ID format**: Use VCD URN format (`urn:vcloud:catalog:uuid`)
- **Add VCD-compliant fields**:
  - `isPublished` (bool) - Whether catalog is externally published
  - `isSubscribed` (bool) - Whether catalog is subscribed from external source
  - `numberOfVAppTemplates` (int) - Count of vApp templates (computed)
  - `numberOfMedia` (int) - Count of media items (computed, default 0)
  - `isLocal` (bool) - Whether catalog is local (default true)
  - `version` (int) - Catalog version (default 1)
  - `ownerID` (string) - Owner user URN
- **Hide internal fields from JSON**:
  - `OrganizationID` should use `json:"-"` tag
  - `CreatedAt`, `UpdatedAt`, `DeletedAt` should use `json:"-"` tag
- **Add computed methods**:
  - `Org()` - Returns organization reference object
  - `Owner()` - Returns owner reference object
  - `PublishConfig()` - Returns publish configuration object
  - `SubscriptionConfig()` - Returns subscription configuration object
  - `DistributedCatalogConfig()` - Returns empty object
  - `CatalogStorageProfiles()` - Returns empty array
- **Update BeforeCreate hook**: Generate catalog URN using new `GenerateCatalogURN()` function

#### 1.2 Add URN Generation (`pkg/database/models/types.go`)
```go
const URNPrefixCatalog = "urn:vcloud:catalog:"

func GenerateCatalogURN() string {
    return URNPrefixCatalog + uuid.New().String()
}
```

### 2. Update Repository Layer

#### 2.1 Enhance Catalog Repository (`pkg/database/repositories/catalog.go`)
- **Add VCD-compliant methods**:
  - `ListWithPagination(limit, offset int) ([]models.Catalog, error)`
  - `CountAll() (int64, error)`
  - `GetByURN(urn string) (*models.Catalog, error)`
  - `GetWithCounts(id string) (*models.Catalog, error)` - Preload template counts
  - `Create(catalog *models.Catalog) error` - Enhanced create with validation
  - `DeleteWithValidation(urn string) error` - Check for dependent templates
  - `HasDependentTemplates(catalogID string) (bool, error)` - Template dependency check
- **Update existing methods**:
  - Modify queries to work with URN-based IDs
  - Add template counting capabilities
- **Add stable pagination ordering**: Use `created_at DESC, id DESC` for consistent results

### 3. Create New API Handlers

#### 3.1 New Catalog Handlers (`pkg/api/handlers/catalogs.go`)
- **Create `CatalogHandlers` struct**:
  - Dependency inject catalog repository and organization repository
- **Implement VCD endpoints**:
  - `GetCatalog(c *gin.Context)` - GET `/cloudapi/1.0.0/catalogs/{catalogUrn}`
  - `ListCatalogs(c *gin.Context)` - GET `/cloudapi/1.0.0/catalogs`
  - `CreateCatalog(c *gin.Context)` - POST `/cloudapi/1.0.0/catalogs`
  - `DeleteCatalog(c *gin.Context)` - DELETE `/cloudapi/1.0.0/catalogs/{catalogUrn}`
- **Add VCD response transformation**:
  - `toCatalogResponse(catalog models.Catalog) CatalogResponse`
  - Handle computed fields (numberOfVAppTemplates, etc.)
  - Format creation date as ISO-8601
- **Implement pagination support**:
  - Parse `page` and `pageSize` query parameters
  - Default page size: 25, max: 128
  - Return paginated response using `types.Page[T]`
- **Add URN validation**:
  - Validate catalog URN format in path parameters
  - Return appropriate 400 errors for invalid URNs
- **Add error handling**:
  - 404 for catalog not found
  - 400 for invalid request body or URN format
  - 409 for catalog deletion with dependent templates
  - 500 for database errors
  - Use consistent `NewAPIError` format
- **Implement CRUD operations**:
  - Create: Validate organization exists, generate URN, set defaults
  - Delete: Check for dependent vApp templates, prevent deletion if templates exist

#### 3.2 Request and Response DTOs
```go
// Request DTO for catalog creation
type CatalogCreateRequest struct {
    Name        string `json:"name" binding:"required"`
    Description string `json:"description"`
    OrgID       string `json:"orgId" binding:"required"` // Organization URN
    IsPublished bool   `json:"isPublished"`
}

// Response DTO for catalog operations
type CatalogResponse struct {
    ID                      string                    `json:"id"`
    Name                    string                    `json:"name"`
    Description             string                    `json:"description"`
    Org                     OrgReference              `json:"org"`
    IsPublished             bool                      `json:"isPublished"`
    IsSubscribed            bool                      `json:"isSubscribed"`
    CreationDate            string                    `json:"creationDate"`
    NumberOfVAppTemplates   int                       `json:"numberOfVAppTemplates"`
    NumberOfMedia           int                       `json:"numberOfMedia"`
    CatalogStorageProfiles  []interface{}             `json:"catalogStorageProfiles"`
    PublishConfig           PublishConfig             `json:"publishConfig"`
    SubscriptionConfig      SubscriptionConfig        `json:"subscriptionConfig"`
    DistributedCatalogConfig interface{}              `json:"distributedCatalogConfig"`
    Owner                   OwnerReference            `json:"owner"`
    IsLocal                 bool                      `json:"isLocal"`
    Version                 int                       `json:"version"`
}

type OrgReference struct {
    ID string `json:"id"`
}

type OwnerReference struct {
    ID string `json:"id"`
}

type PublishConfig struct {
    IsPublished bool `json:"isPublished"`
}

type SubscriptionConfig struct {
    IsSubscribed bool `json:"isSubscribed"`
}
```

### 4. Update Server Routes

#### 4.1 Modify API Server (`pkg/api/server.go`)
- **Add catalog handlers initialization**:
  - Create `catalogHandlers` field
  - Initialize in `NewServer` constructor
- **Add VCD-compliant routes**:
  ```go
  cloudAPIRoot := s.router.Group("/cloudapi/1.0.0")
  cloudAPIRoot.Use(auth.JWTMiddleware(s.jwtManager))
  {
      cloudAPIRoot.GET("/catalogs", s.catalogHandlers.ListCatalogs)
      cloudAPIRoot.POST("/catalogs", s.catalogHandlers.CreateCatalog)
      cloudAPIRoot.GET("/catalogs/:catalogUrn", s.catalogHandlers.GetCatalog)
      cloudAPIRoot.DELETE("/catalogs/:catalogUrn", s.catalogHandlers.DeleteCatalog)
  }
  ```
- **Remove legacy catalog route comments**

### 5. Update Tests

#### 5.1 Create Catalog API Tests (`test/unit/catalog_api_test.go`)
- **Test coverage**:
  - GET single catalog (success, not found, invalid URN)
  - GET catalog list (empty, with results, pagination)
  - POST create catalog (success, validation errors, invalid org)
  - DELETE catalog (success, not found, with dependent templates)
  - Response format validation (all required fields present)
  - URN validation (invalid format returns 400)
  - Pagination parameters (page, pageSize, defaults)
  - Template counting accuracy
  - CRUD operations and dependency validation
- **Test data setup**:
  - Create test organizations and catalogs
  - Create test templates to verify counting
  - Generate valid URNs for testing

#### 5.2 Update Database Tests (`test/unit/database_test.go`)
- **Update catalog repository tests**:
  - Test new URN-based methods
  - Test pagination methods
  - Test template counting
  - Verify URN generation

### 6. Remove Legacy Code

#### 6.1 Clean Up Legacy Handlers
- **Remove unused catalog handler code** from server.go comments
- **Verify no existing catalog endpoints** are active

#### 6.2 Update Documentation
- **Update API documentation** to reflect new VCD-compliant endpoints
- **Document breaking changes** from legacy catalog API
- **Add VCD URN format examples**

## Data Migration Considerations

### No Migration Required
- **Breaking change approach**: Remove old code and replace with new schema
- **Catalog IDs will change**: From legacy format to VCD URN format
- **New catalog data fields**: Will be initialized with defaults
  - `isPublished`: false
  - `isSubscribed`: false
  - `isLocal`: true
  - `version`: 1
  - `numberOfVAppTemplates`: computed from relationships
  - `numberOfMedia`: 0 (no media support yet)

## Authentication & Authorization

### Current Approach
- **Use existing JWT middleware** for authentication
- **No role-based restrictions** for catalog viewing (consistent with VCD)
- **Catalogs filtered by organization access** through JWT claims

### VCD Compliance
- **Organization-scoped access**: Users see catalogs from their organizations
- **Shared catalogs**: Visible to all users (if `isPublished` is true)
- **No System Administrator requirement**: Unlike VDC API, catalogs are generally accessible

## Testing Strategy

### Unit Tests
1. **Database model tests**: URN generation, computed methods
2. **Repository tests**: New methods, pagination, counting
3. **Handler tests**: HTTP responses, error handling, pagination
4. **Integration tests**: End-to-end API functionality

### Manual Testing
1. **Create catalogs**: Verify URN generation
2. **List catalogs**: Test pagination and filtering
3. **Get single catalog**: Verify response format
4. **Template counting**: Add templates and verify counts

## Implementation Phases

### Phase 1: Data Model Updates
1. Update `Catalog` model with VCD fields
2. Add URN generation functions
3. Update model tests

### Phase 2: Repository Layer
1. Add new repository methods
2. Update existing methods for URN support
3. Add repository tests

### Phase 3: API Handlers
1. Create new catalog handlers
2. Implement VCD-compliant endpoints
3. Add response transformation logic

### Phase 4: Server Integration
1. Update server routes
2. Remove legacy code
3. Add integration tests

### Phase 5: Testing & Documentation
1. Comprehensive test coverage
2. Update documentation
3. Manual testing and validation

## Success Criteria

1. **API Compliance**: Endpoints match VCD specification exactly
2. **Data Format**: Response format matches VCD catalog schema
3. **Pagination**: Works correctly with page/pageSize parameters
4. **URN Support**: All catalogs use proper VCD URN format
5. **Template Counting**: Accurate count of vApp templates
6. **Error Handling**: Proper HTTP status codes and error messages
7. **Test Coverage**: 100% test coverage for new code
8. **No Regressions**: All existing tests pass

## Risks & Mitigation

### Breaking Changes
- **Risk**: Existing catalog API consumers will break
- **Mitigation**: This is intentional - complete replacement approach

### Data Loss
- **Risk**: Existing catalog data format incompatibility
- **Mitigation**: Data will be preserved, only format changes

### Performance
- **Risk**: Template counting queries may be slow
- **Mitigation**: Use efficient GORM counting, consider caching if needed

## Future Enhancements

1. **CRUD Operations**: Add POST, PUT, DELETE for catalog management
2. **Advanced Filtering**: Implement FIQL-based filtering
3. **Catalog Items**: Add catalog item endpoints
4. **Media Support**: Add media item counting and management
5. **Publishing/Subscription**: Implement full publish/subscribe workflow
6. **Storage Profiles**: Add real storage profile support