# Proposal: Re-enable Kubernetes Integration in SSVirt

## Executive Summary

This proposal outlines the re-enablement of Kubernetes integration in SSVirt by embedding a controller-runtime client directly into the API server process, eliminating the need for a separate controller component. This approach will provide the required Kubernetes functionality while simplifying deployment and reducing operational complexity.

## Current State Analysis

### Disabled Components

Currently, the following Kubernetes-related components are disabled:

1. **Separate Controller Process** (`cmd/controller.disabled/`)
   - VDC reconciler that manages namespace lifecycle
   - Separate deployment with controller-runtime manager
   - Database polling and Kubernetes event handling

2. **Kubernetes Client Library** (`pkg/k8s.disabled/`)
   - Controller-runtime based client with caching
   - VM management operations
   - VM translation between VCD and KubeVirt formats

3. **Helm Chart Controller Components**
   - Controller deployment template
   - RBAC resources for controller
   - Service account and permissions

### Current Limitations

- Organizations and VDCs exist only as database records
- No actual Kubernetes namespaces are created
- Catalog items cannot query OpenShift templates
- VM instantiation creates only database records, not actual VMs

## Requirements

The re-enabled Kubernetes integration must support:

1. **VDC Namespace Management**
   - Create Kubernetes namespace when VDC is created
   - Apply appropriate labels and annotations
   - Handle VDC deletion (namespace cleanup)

2. **Template Discovery**
   - Query templates in the "openshift" namespace
   - Present templates as catalog items in VCD API responses

3. **VM Instantiation**
   - Create TemplateInstance in VDC's namespace
   - Handle template instantiation lifecycle
   - Sync status between Kubernetes and database

## Proposed Architecture

### Embedded Kubernetes Client

Instead of a separate controller process, embed a controller-runtime client directly in the API server:

```
┌─────────────────────────────────────┐
│           API Server Process        │
│  ┌─────────────────┐ ┌─────────────┐│
│  │   HTTP Handlers │ │   K8s Client││
│  │                 │ │   (embedded)││
│  │   ┌─────────────┤ │             ││
│  │   │ VDC Handler │◄┤   Namespace ││
│  │   │             │ │   Manager   ││
│  │   ├─────────────┤ │             ││
│  │   │Catalog Hndlr│◄┤   Template  ││
│  │   │             │ │   Discovery ││
│  │   ├─────────────┤ │             ││
│  │   │Instant Hndlr│◄┤   Template  ││
│  │   │             │ │   Instance  ││
│  │   └─────────────┘ │   Manager   ││
│  └─────────────────┐ └─────────────┘│
│                    │                 │
│  ┌─────────────────▼┐               │
│  │    Database      │               │
│  │   Repositories   │               │
│  └──────────────────┘               │
└─────────────────────────────────────┘
```

### Key Components

1. **Kubernetes Service**
   - Embedded controller-runtime client with caching
   - Namespace operations (create, update, delete)
   - Template discovery and querying
   - TemplateInstance management

2. **Enhanced Handlers**
   - VDC handler triggers namespace creation
   - Catalog handler queries OpenShift templates
   - InstantiateTemplate handler creates TemplateInstance

3. **Simplified Deployment**
   - Single process (API server only)
   - Reduced RBAC complexity
   - Fewer moving parts to manage

## Implementation Plan

### Phase 1: Core Infrastructure

1. **Move and Enable Kubernetes Client**
   ```bash
   pkg/k8s.disabled/ → pkg/k8s/
   ```
   - Remove `//go:build ignore` directive
   - Adapt client for embedded use in API server
   - Add health checks and graceful shutdown

2. **Create Kubernetes Service Layer**
   ```go
   pkg/services/kubernetes.go
   ```
   - Namespace management operations
   - Template discovery from "openshift" namespace
   - TemplateInstance lifecycle management

3. **Update API Server Initialization**
   ```go
   cmd/api-server/main.go
   ```
   - Initialize Kubernetes client alongside database
   - Pass client to handlers via dependency injection
   - Handle client startup and shutdown

### Phase 2: VDC Namespace Integration

1. **Enhance VDC Repository**
   - Add post-create hook for namespace creation
   - Add pre-delete hook for namespace cleanup
   - Handle VDC enable/disable states

2. **Update VDC Handlers**
   - Integrate namespace operations in create/update/delete flows
   - Add error handling for Kubernetes failures
   - Ensure database and Kubernetes consistency

3. **Namespace Labeling Strategy**
   ```yaml
   labels:
     ssvirt.io/organization: "org-name"
     ssvirt.io/organization-id: "urn:vcloud:org:..."
     ssvirt.io/vdc: "vdc-name"
     ssvirt.io/vdc-id: "urn:vcloud:vdc:..."
     app.kubernetes.io/managed-by: "ssvirt"
   ```

### Phase 3: Template Discovery

1. **Template Service Implementation**
   - Query OpenShift templates in "openshift" namespace
   - Cache template metadata for performance
   - Transform templates to VCD catalog item format

2. **Catalog Handler Enhancement**
   - Integrate template discovery in catalog item responses
   - Filter templates based on user permissions
   - Handle template availability and status

### Phase 4: Template Instantiation

1. **TemplateInstance Manager**
   - Create TemplateInstance resources in VDC namespaces
   - Monitor instantiation progress and status
   - Handle success and failure scenarios

2. **InstantiateTemplate Handler**
   - Validate template availability
   - Create TemplateInstance with appropriate parameters
   - Update vApp status based on instantiation progress

### Phase 5: Cleanup and Optimization

1. **Remove Disabled Components**
   - Delete `pkg/controllers.disabled/`
   - Delete `cmd/controller.disabled/`
   - Update Helm chart to remove controller deployment

2. **Documentation Updates**
   - Update README.md
   - Update architecture documentation
   - Update deployment guides

## Detailed Technical Design

### Kubernetes Service Interface

```go
type KubernetesService interface {
    // Namespace Management
    CreateNamespaceForVDC(ctx context.Context, vdc *models.VDC) error
    DeleteNamespaceForVDC(ctx context.Context, vdc *models.VDC) error
    UpdateNamespaceForVDC(ctx context.Context, vdc *models.VDC) error
    
    // Template Discovery
    ListTemplatesInNamespace(ctx context.Context, namespace string) ([]*Template, error)
    GetTemplate(ctx context.Context, namespace, name string) (*Template, error)
    
    // Template Instantiation
    CreateTemplateInstance(ctx context.Context, req *TemplateInstanceRequest) (*TemplateInstance, error)
    GetTemplateInstance(ctx context.Context, namespace, name string) (*TemplateInstance, error)
    DeleteTemplateInstance(ctx context.Context, namespace, name string) error
    
    // Health and Status
    HealthCheck(ctx context.Context) error
}
```

### VDC Lifecycle Integration

```go
// In VDC repository
func (r *VDCRepository) Create(vdc *models.VDC) error {
    // Create database record
    if err := r.db.Create(vdc).Error; err != nil {
        return err
    }
    
    // Create Kubernetes namespace
    if r.k8sService != nil {
        if err := r.k8sService.CreateNamespaceForVDC(context.Background(), vdc); err != nil {
            // Rollback database creation
            r.db.Delete(vdc)
            return fmt.Errorf("failed to create namespace: %w", err)
        }
    }
    
    return nil
}
```

### Template Integration in Catalog Handlers

```go
func (h *CatalogHandlers) ListCatalogItems(c *gin.Context) {
    // Get database catalog items
    dbItems, err := h.catalogItemRepo.List()
    if err != nil {
        // handle error
    }
    
    // Get OpenShift templates
    templates, err := h.k8sService.ListTemplatesInNamespace(c.Request.Context(), "openshift")
    if err != nil {
        // Log warning but continue with database items only
        log.Warn("Failed to query OpenShift templates", "error", err)
    }
    
    // Merge and transform for response
    response := mergeCatalogItemsAndTemplates(dbItems, templates)
    c.JSON(http.StatusOK, response)
}
```

## Removed Components

### Helm Chart Changes

**Files to Remove:**
- `chart/ssvirt/templates/controller-deployment.yaml`
- Controller-specific sections in `values.yaml`
- Controller RBAC resources (if controller-specific)

**Files to Modify:**
- `chart/ssvirt/values.yaml` - Remove controller configuration
- `chart/ssvirt/templates/rbac.yaml` - Update for API server needs only
- `chart/ssvirt/README.md` - Update documentation

### Codebase Changes

**Directories to Remove:**
- `pkg/controllers.disabled/` → Delete entirely
- `cmd/controller.disabled/` → Delete entirely

**Files to Move/Enable:**
- `pkg/k8s.disabled/` → `pkg/k8s/`
- Remove `//go:build ignore` directives
- Update imports throughout codebase

## RBAC Requirements

The API server will need the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ssvirt-api-server
rules:
# Namespace management
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]

# Template discovery
- apiGroups: ["template.openshift.io"]
  resources: ["templates"]
  verbs: ["get", "list"]

# Template instantiation
- apiGroups: ["template.openshift.io"]
  resources: ["templateinstances"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]

# Resource quota and network policies (for VDC namespaces)
- apiGroups: [""]
  resources: ["resourcequotas"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
```

## Error Handling Strategy

### Database-Kubernetes Consistency

1. **Create Operations**: Database first, then Kubernetes
   - On Kubernetes failure: Rollback database transaction
   - Maintain strong consistency

2. **Delete Operations**: Kubernetes first, then database
   - On database failure: Log error but continue
   - Prefer orphaned database records over orphaned Kubernetes resources

3. **Update Operations**: Update both simultaneously
   - Use database transactions where possible
   - Implement compensating actions for failures

### Graceful Degradation

When Kubernetes is unavailable:
- VDC operations create database records only
- Catalog handlers return database items only
- Template instantiation returns appropriate errors
- Health checks report degraded state

## Implementation Approach

Since no production deployments exist yet, we can implement this as a direct code change without migration concerns.

## Testing Strategy

### Unit Tests
- Kubernetes service interface mocking
- VDC lifecycle with namespace operations
- Template discovery and transformation
- Error handling scenarios

### Integration Tests
- End-to-end VDC creation with namespace
- Template instantiation workflow
- Failure recovery and rollback
- RBAC permission validation

### Performance Tests
- Template discovery latency
- Namespace creation throughput
- Memory usage of embedded client

## Benefits

1. **Simplified Architecture**
   - Single process to deploy and manage
   - Reduced operational complexity
   - Fewer failure points

2. **Improved Consistency**
   - Synchronous operations between database and Kubernetes
   - Better error handling and rollback capabilities
   - No eventual consistency issues

3. **Enhanced Performance**
   - No inter-process communication overhead
   - Direct access to cached Kubernetes resources
   - Reduced latency for operations

4. **Operational Benefits**
   - Single image to build and deploy
   - Simplified monitoring and logging
   - Easier debugging and troubleshooting

## Risks and Mitigation

### Risk: API Server Resource Usage
**Mitigation**: Configure controller-runtime client with appropriate cache limits and memory constraints

### Risk: Kubernetes Permissions
**Mitigation**: Implement least-privilege RBAC with comprehensive testing

### Risk: Failure Recovery
**Mitigation**: Implement robust error handling and compensating transactions

## Timeline

- **Week 1**: Core infrastructure and client enablement
- **Week 2**: VDC namespace integration
- **Week 3**: Template discovery implementation
- **Week 4**: Template instantiation functionality
- **Week 5**: Testing, cleanup, and documentation
- **Week 6**: Migration support and final validation

## Conclusion

This proposal provides a path to re-enable Kubernetes integration in SSVirt while simplifying the overall architecture. By embedding the Kubernetes client in the API server, we eliminate the complexity of a separate controller process while maintaining all required functionality.

The approach ensures strong consistency between database and Kubernetes resources, provides better error handling, and reduces operational overhead.