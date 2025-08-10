# Catalog Items API Enhancement

## Summary

This enhancement proposes implementing VCD-compliant catalogItems API endpoints
that integrate with OpenShift Templates using a caching Kubernetes client. The
API will provide read-only access to catalog items backed by OpenShift Template
resources.

## Motivation

The existing catalog API provides VCD-compliant endpoints for managing catalogs,
but lacks the ability to list and retrieve individual catalog items. In a
cloud-native environment, these catalog items should be backed by OpenShift
Templates, which contain VM specifications that can be deployed as virtual
machines.

## Goals

- Implement VCD-compliant catalogItems API endpoints
- Integrate with OpenShift Templates via Kubernetes controller-runtime caching client
- Provide efficient querying and filtering of catalog items
- Map OpenShift Template metadata to VCD catalogItem fields
- Support pagination for large template collections

## Non-Goals

- Creating or modifying OpenShift Templates through the API (read-only)
- Supporting other Kubernetes resources beyond Templates
- Implementing catalog item deployment functionality

## Proposal

### API Endpoints

The following VCD CloudAPI endpoints will be implemented:

1. **List Catalog Items**: `GET /cloudapi/1.0.0/catalogs/{catalog_id}/catalogItems`
   - Query parameters: `page`, `pageSize` (pagination)
   - Returns paginated list of catalog items in the specified catalog

2. **Get Catalog Item**: `GET /cloudapi/1.0.0/catalogs/{catalog_id}/catalogItems/{item_id}`
   - Returns detailed information about a specific catalog item

### Data Model

#### CatalogItem Structure

The catalogItem will map OpenShift Template fields to VCD-compliant structure:

```go
type CatalogItem struct {
    ID                string              `json:"id"`                // Template UID with URN prefix
    Name              string              `json:"name"`              // Template metadata.name
    Description       string              `json:"description"`       // Template metadata.annotations["description"]
    CatalogID         string              `json:"catalogId"`         // Parent catalog URN
    IsPublished       bool                `json:"isPublished"`       // Based on Template labels
    IsExpired         bool                `json:"isExpired"`         // Always false for Templates
    CreationDate      string              `json:"creationDate"`      // Template metadata.creationTimestamp
    Size              int64               `json:"size"`              // Computed/estimated size
    Status            string              `json:"status"`            // Always "RESOLVED" for Templates
    Entity            CatalogItemEntity   `json:"entity"`            // Template details
    Owner             EntityRef           `json:"owner"`             // Parent catalog's owner
    Catalog           EntityRef           `json:"catalog"`           // Parent catalog reference
}

type CatalogItemEntity struct {
    Name              string    `json:"name"`
    Description       string    `json:"description"`
    Type              string    `json:"type"`              // "application/vnd.vmware.vcloud.vAppTemplate+xml"
    NumberOfVMs       int       `json:"numberOfVMs"`       // Count from template objects
    NumberOfCpus      int       `json:"numberOfCpus"`      // Sum from template parameters
    MemoryAllocation  int64     `json:"memoryAllocation"`  // Sum from template parameters
    StorageAllocation int64     `json:"storageAllocation"` // Sum from template parameters
}
```

#### Template to CatalogItem Mapping

| CatalogItem Field | OpenShift Template Source |
|------------------|---------------------------|
| `id` | `urn:vcloud:catalogitem:{metadata.uid}` |
| `name` | `metadata.name` |
| `description` | `metadata.annotations["description"]` or `metadata.annotations["template.openshift.io/long-description"]` |
| `catalogId` | From query context (catalog URN) |
| `isPublished` | `metadata.labels["catalog.ssvirt.io/published"] == "true"` |
| `creationDate` | `metadata.creationTimestamp` |
| `entity.numberOfVMs` | Count of VM objects in template |
| `entity.numberOfCpus` | Sum of CPU parameters from template |
| `entity.memoryAllocation` | Sum of memory parameters from template |
| `entity.storageAllocation` | Sum of storage parameters from template |

### Kubernetes Integration

#### Caching Client Setup

The implementation will use controller-runtime's caching client for efficient OpenShift Template access:

```go
import (
    "k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/cache"
    "sigs.k8s.io/controller-runtime/pkg/client"
    templatev1 "github.com/openshift/api/template/v1"
)

type TemplateService struct {
    client client.Client
    cache  cache.Cache
}
```

#### Template Discovery and Filtering

For the initial implementation, we will use a simplified approach that can be enhanced later:

- **Fixed Namespace**: All catalog item queries will return Templates from the `openshift` namespace only, regardless of which catalog is being queried

- **Universal Template Set**: The same set of Templates will be returned for every catalog - there is no catalog-specific filtering at this stage

- **Template Selection Criteria**: Templates must meet the following requirements to be included:
  - Must be located in the `openshift` namespace
  - Must have the label `template.kubevirt.io/version` with any value
  - Must have the annotation `template.kubevirt.io/containerdisks` with any value
  - Must contain valid Template objects and metadata

The CatalogItemRepository will just query the caching client directly to get results.

This simplified approach allows us to get the basic functionality working quickly, with the understanding that catalog-specific filtering and more sophisticated discovery mechanisms will be added in future iterations.

#### Template Validation

Templates must contain specific metadata to be considered valid catalog items:
- Valid template objects with VM specifications

### Implementation Architecture

#### Repository Layer

```go
type CatalogItemRepository interface {
    ListByCatalogID(catalogID string, limit, offset int) ([]CatalogItem, error)
    CountByCatalogID(catalogID string) (int64, error)
    GetByID(catalogID, itemID string) (*CatalogItem, error)
}

type TemplateCatalogItemRepository struct {
    templateService *TemplateService
    catalogRepo     *CatalogRepository
}
```

#### Handler Layer

```go
type CatalogItemHandler struct {
    catalogItemRepo CatalogItemRepository
    catalogRepo     *CatalogRepository
}

func (h *CatalogItemHandler) ListCatalogItems(c *gin.Context)
func (h *CatalogItemHandler) GetCatalogItem(c *gin.Context)
```

#### Service Layer

```go
type TemplateService struct {
    client    client.Client
    cache     cache.Cache
    mapper    *TemplateMapper
}

type TemplateMapper struct{}

func (m *TemplateMapper) TemplateToCatalogItem(template *templatev1.Template, catalogID string) *CatalogItem
func (m *TemplateMapper) ExtractVMCount(template *templatev1.Template) int
func (m *TemplateMapper) ExtractResourceRequirements(template *templatev1.Template) (cpus int, memory int64, storage int64)
```

### Error Handling

- **Catalog Not Found**: Return 404 when catalog URN is invalid or not found
- **Template Access Errors**: Log and skip inaccessible templates, don't fail entire requests
- **Invalid Template UID**: Return 404 for malformed or non-existent template UIDs
- **Kubernetes API Errors**: Return 503 for temporary API unavailability

### Pagination Implementation

The API will support VCD-style pagination:
- Default page size: 25 items
- Maximum page size: 100 items
- Response format matches existing catalog API pagination structure

### Configuration

### 

#### Required Environment Variables

```bash
# Kubernetes configuration
KUBECONFIG=/path/to/kubeconfig
```

Or use the ServiceAccount for k8s access if provided. The controller-runtime
client should handle this part on its own.

### Route Configuration

Routes will be added to the existing CloudAPI router:

```go
catalogItems := cloudapi.Group("/catalogs/:catalogId/catalogItems")
catalogItems.Use(middleware.AuthRequired())
{
    catalogItems.GET("", catalogItemHandler.ListCatalogItems)
    catalogItems.GET("/:itemId", catalogItemHandler.GetCatalogItem)
}
```

### Testing Strategy

#### Unit Tests

- Template to CatalogItem mapping functionality
- Repository methods with mock Kubernetes client
- Handler endpoint behavior with test templates
- Error handling scenarios

#### Integration Tests

- End-to-end API tests with real Templates in test namespace
- Cache behavior and performance testing
- Pagination accuracy and consistency
- Authentication and authorization

### Performance Considerations

- **Caching**: Controller-runtime cache will minimize API calls to Kubernetes
- **Indexing**: Templates will be indexed by catalog-id label for efficient filtering
- **Batch Processing**: Template queries will be batched when possible
- **Resource Limits**: Template processing will respect memory and CPU limits

### Security Considerations

- **RBAC**: Service account must have read-only access to Template resources
- **Namespace Isolation**: Templates will only be accessible within configured namespaces
- **Input Validation**: All catalog and item IDs will be validated as proper URNs
- **Rate Limiting**: API endpoints will respect existing authentication and rate limiting

### Migration and Compatibility

- **Backward Compatibility**: Existing catalog API remains unchanged
- **Template Requirements**: Existing Templates may need label updates for catalog association
- **Gradual Rollout**: Can be deployed alongside existing catalog functionality

### Limitations and Disclaimers

1. **Template UID Stability**: OpenShift Template UIDs may change if templates are recreated, which would break catalogItem references. This is a known limitation of using Template resources as backing store.

2. **Incomplete VCD Documentation**: Some catalogItem fields may not have exact VCD API documentation equivalents, requiring best-effort mapping based on available VMware resources.

3. **Read-Only Access**: The API provides read-only access to catalog items. Template creation and modification must be done through standard Kubernetes tooling.

4. **Resource Estimation**: VM resource requirements (CPU, memory, storage) are estimated from template parameters and may not reflect actual deployment requirements.

5. **Template Validation**: The system assumes Templates contain valid VM specifications but does not validate deployability.

### Future Enhancements

- Support for template versioning and updates
- Integration with additional Kubernetes resources (e.g., VirtualMachine CRDs)
- Catalog item deployment functionality
- Advanced filtering and search capabilities
- Template validation and health checking

## Implementation Plan

### Phase 1: Core Infrastructure
1. Set up Kubernetes client and caching infrastructure
2. Implement Template service and mapper
3. Create CatalogItem repository interface and implementation

### Phase 2: API Endpoints
1. Implement list catalogItems endpoint with pagination
2. Implement get catalogItem endpoint
3. Add comprehensive error handling

### Phase 3: Integration and Testing
1. Integrate with existing authentication and authorization
2. Add unit and integration tests
3. Performance testing and optimization

### Phase 4: Documentation and Deployment
1. Update API documentation
2. Create deployment configuration
3. Add monitoring and observability

## Acceptance Criteria

- [ ] List catalogItems endpoint returns paginated VCD-compliant responses
- [ ] Get catalogItem endpoint returns detailed item information
- [ ] Templates are correctly mapped to catalogItem structure
- [ ] Pagination works correctly with various page sizes
- [ ] Authentication and authorization are properly enforced
- [ ] Error handling covers all edge cases
- [ ] Performance meets requirements (< 500ms for list, < 200ms for get)
- [ ] Integration tests pass with real OpenShift Templates
- [ ] Documentation is complete and accurate