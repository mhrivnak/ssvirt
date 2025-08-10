# VM Creation API Enhancement

## Overview

This enhancement proposes adding a new VM creation API endpoint at `/cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate` that enables authenticated users to create virtual machines by instantiating catalog item templates. This implementation maps to OpenShift TemplateInstance creation in the VDC's corresponding namespace.

## Background

Currently, the system supports VDC and catalog browsing through read-only APIs, but lacks the capability for users to actually create virtual machines. The existing plan.md outlines VM creation through vApp instantiation, but this enhancement focuses on direct template instantiation which is more aligned with modern cloud-native patterns.

## Goals

1. **Enable VM provisioning**: Allow authenticated users to create VMs from catalog items
2. **OpenShift integration**: Map VM creation to OpenShift TemplateInstance creation
3. **Access control**: Ensure proper organization-based authorization
4. **VCD API compliance**: Follow VMware Cloud Director API patterns where applicable
5. **Resource tracking**: Create proper vApp containers for VM instances
6. **Template integration**: Leverage existing catalog item and template infrastructure

## Non-Goals

- Template parameter customization (will use defaults initially)
- Complex vApp orchestration (single VM per vApp for now)
- Cross-organization template access
- Backward compatibility with existing VCD APIs (this endpoint doesn't exist in actual VCD)

## Architecture Overview

### Core Concepts Mapping

- **VDC** → Kubernetes Namespace
- **vApp** → OpenShift TemplateInstance (container for VM resources)
- **VM** → OpenShift VirtualMachine (created by TemplateInstance)
- **Catalog Item** → OpenShift Template reference

### Implementation Flow

1. **Request Validation**: Validate VDC access and catalog item permissions
2. **Template Resolution**: Find the OpenShift Template corresponding to the catalog item
3. **TemplateInstance Creation**: Create TemplateInstance in the VDC's namespace
4. **vApp Record Creation**: Create database record for the resulting vApp
5. **Response Generation**: Return vApp object with instantiation status

## Proposed API Endpoint

### Instantiate Template

**Endpoint**: `POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate`
**Description**: Creates a new VM by instantiating a catalog item template
**Authentication**: JWT token required
**Authorization**: User must be member of VDC's organization and catalog item's organization

**Path Parameters**:
- `vdc_id` (required): VDC URN (e.g., `urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc`)

**Request Body**:
```json
{
  "name": "my-vm-instance",
  "description": "Development VM for testing",
  "catalogItem": {
    "id": "my-template",
    "name": "Ubuntu 22.04 Template"
  }
}
```

**Request Schema**:
- `name` (required, string): Name for the new vApp/VM instance
- `description` (optional, string): Description for the vApp
- `catalogItem` (required, object): Reference to catalog item to instantiate
  - `id` (required, string): Catalog item ID/name
  - `name` (optional, string): Catalog item name for validation

**Response**: `201 Created`
```json
{
  "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "name": "my-vm-instance",
  "description": "Development VM for testing",
  "status": "INSTANTIATING",
  "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "numberOfVMs": 1,
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
}
```

**Error Responses**:
- `400 Bad Request`: Invalid request format or missing required fields
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: User not authorized to access VDC or catalog item
- `404 Not Found`: VDC or catalog item not found
- `409 Conflict`: Name already in use within VDC
- `500 Internal Server Error`: Template instantiation failed

## Supporting vApps API Endpoints

To support the VM creation workflow and provide complete vApp management capabilities, the following vApps API endpoints are required:

### 1. Query vApps in VDC

**Endpoint**: `GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps`
**Description**: Returns a paginated list of vApps in the specified VDC
**Authentication**: JWT token required
**Authorization**: User must be member of VDC's organization

**Query Parameters**:
- `page` (optional): Page number (default: 1)
- `pageSize` (optional): Number of items per page (default: 25, max: 100)
- `filter` (optional): Filter expression (e.g., `name==my-vapp`)
- `sortAsc` (optional): Sort field in ascending order
- `sortDesc` (optional): Sort field in descending order

**Response**: `200 OK`
```json
{
  "resultTotal": 15,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
      "name": "my-vm-instance",
      "description": "Development VM for testing",
      "status": "DEPLOYED",
      "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
      "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
      "createdAt": "2024-01-15T10:30:00Z",
      "numberOfVMs": 1,
      "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
    }
  ]
}
```

### 2. Get vApp Details

**Endpoint**: `GET /cloudapi/1.0.0/vapps/{vapp_id}`
**Description**: Returns detailed information about a specific vApp
**Authentication**: JWT token required
**Authorization**: User must be member of vApp's VDC organization

**Path Parameters**:
- `vapp_id` (required): vApp URN (e.g., `urn:vcloud:vapp:87654321-4321-4321-4321-210987654321`)

**Response**: `200 OK`
```json
{
  "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "name": "my-vm-instance",
  "description": "Development VM for testing",
  "status": "DEPLOYED",
  "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:35:00Z",
  "numberOfVMs": 1,
  "vms": [
    {
      "id": "urn:vcloud:vm:11111111-2222-3333-4444-555555555555",
      "name": "my-vm-instance-vm1",
      "status": "POWERED_ON",
      "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:11111111-2222-3333-4444-555555555555"
    }
  ],
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
}
```

### 3. Delete vApp

**Endpoint**: `DELETE /cloudapi/1.0.0/vapps/{vapp_id}`
**Description**: Deletes a vApp and all its contained VMs
**Authentication**: JWT token required
**Authorization**: User must be member of vApp's VDC organization

**Path Parameters**:
- `vapp_id` (required): vApp URN

**Query Parameters**:
- `force` (optional): Force deletion even if VMs are powered on (default: false)

**Response**: `204 No Content` (successful deletion)

**Error Responses**:
- `400 Bad Request`: vApp contains powered-on VMs and force=false
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: User not authorized to delete this vApp
- `404 Not Found`: vApp not found
- `409 Conflict`: vApp cannot be deleted due to dependent resources
- `500 Internal Server Error`: Deletion failed

### vApps API Implementation Details

#### Database Operations
```go
// Query vApps in VDC with pagination and filtering
func (r *VAppRepository) ListByVDCWithPagination(ctx context.Context, vdcID string, limit, offset int, filter string) ([]models.VApp, error)

// Count vApps in VDC (for pagination)
func (r *VAppRepository) CountByVDC(ctx context.Context, vdcID string, filter string) (int64, error)

// Get vApp with VMs preloaded
func (r *VAppRepository) GetWithVMs(ctx context.Context, vappID string) (*models.VApp, error)

// Delete vApp and associated resources
func (r *VAppRepository) DeleteWithValidation(ctx context.Context, vappID string, force bool) error
```

#### Access Control for vApps
```go
// Validate user access to vApp through VDC organization membership
func (h *VAppHandlers) validateVAppAccess(ctx context.Context, userID, vappID string) (*models.VApp, error) {
    vapp, err := h.vappRepo.GetWithVDC(ctx, vappID)
    if err != nil {
        return nil, fmt.Errorf("vApp access denied: %w", err)
    }
    
    // Check if user has access to the VDC containing this vApp
    err = h.vdcHandlers.validateVDCAccess(ctx, userID, vapp.VDCID)
    if err != nil {
        return nil, fmt.Errorf("VDC access denied: %w", err)
    }
    
    return vapp, nil
}
```

#### OpenShift Integration for vApp Deletion
When deleting a vApp, the system must:
1. **Stop VMs**: Power off any running VirtualMachines
2. **Delete TemplateInstance**: Remove the TemplateInstance that created the vApp
3. **Clean up Resources**: Remove any associated ConfigMaps, Secrets, etc.
4. **Update Database**: Delete vApp and VM records

```go
func (s *VAppService) DeleteVApp(ctx context.Context, vappID string, force bool) error {
    // Get vApp with VMs
    vapp, err := s.vappRepo.GetWithVMs(ctx, vappID)
    if err != nil {
        return err
    }
    
    // Check if VMs are powered on
    if !force && s.hasRunningVMs(vapp.VMs) {
        return fmt.Errorf("vApp contains running VMs, use force=true to power off")
    }
    
    // Power off VMs if needed
    if force {
        for _, vm := range vapp.VMs {
            err = s.vmService.PowerOff(ctx, vm.ID)
            if err != nil {
                return fmt.Errorf("failed to power off VM %s: %w", vm.ID, err)
            }
        }
    }
    
    // Delete TemplateInstance in OpenShift
    err = s.k8sClient.Delete(ctx, &templatev1.TemplateInstance{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("vapp-%s", extractUUID(vappID)),
            Namespace: vapp.VDC.Namespace,
        },
    })
    if err != nil && !errors.IsNotFound(err) {
        return fmt.Errorf("failed to delete TemplateInstance: %w", err)
    }
    
    // Delete database records
    return s.vappRepo.DeleteWithValidation(ctx, vappID, force)
}
```

#### Route Configuration
```go
// Add vApps routes to CloudAPI group
cloudAPI.GET("/vdcs/:vdc_id/vapps", s.vappHandlers.ListVApps)
cloudAPI.GET("/vapps/:vapp_id", s.vappHandlers.GetVApp)
cloudAPI.DELETE("/vapps/:vapp_id", s.vappHandlers.DeleteVApp)
```

## Supporting VM API Endpoint

To support VM resource access referenced in vApp responses, the following VM API endpoint is required:

### Get VM Details

**Endpoint**: `GET /cloudapi/1.0.0/vms/{vm_id}`
**Description**: Returns detailed information about a specific virtual machine
**Authentication**: JWT token required
**Authorization**: User must be member of VM's VDC organization

**Path Parameters**:
- `vm_id` (required): VM URN (e.g., `urn:vcloud:vm:11111111-2222-3333-4444-555555555555`)

**Response**: `200 OK`
```json
{
  "id": "urn:vcloud:vm:11111111-2222-3333-4444-555555555555",
  "name": "my-vm-instance-vm1",
  "description": "Virtual machine created from Ubuntu template",
  "status": "POWERED_ON",
  "vappId": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:35:00Z",
  "guestOs": "Ubuntu Linux (64-bit)",
  "vmTools": {
    "status": "RUNNING",
    "version": "12.1.5"
  },
  "hardware": {
    "numCpus": 2,
    "numCoresPerSocket": 1,
    "memoryMB": 4096
  },
  "storageProfile": {
    "name": "default-storage-policy",
    "href": "/cloudapi/1.0.0/storageProfiles/default-storage-policy"
  },
  "networkConnections": [
    {
      "networkName": "default-network",
      "ipAddress": "192.168.1.100",
      "macAddress": "00:50:56:12:34:56",
      "connected": true
    }
  ],
  "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:11111111-2222-3333-4444-555555555555"
}
```

**Error Responses**:
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: User not authorized to access this VM
- `404 Not Found`: VM not found
- `500 Internal Server Error`: Failed to retrieve VM information

### VM API Implementation Details

#### Database Operations
```go
// Get VM with vApp and VDC information for access control
func (r *VMRepository) GetWithVApp(ctx context.Context, vmID string) (*models.VM, error) {
    var vm models.VM
    err := r.db.WithContext(ctx).
        Preload("VApp").
        Preload("VApp.VDC").
        Where("id = ?", vmID).
        First(&vm).Error
    
    if err != nil {
        return nil, err
    }
    
    return &vm, nil
}
```

#### Access Control for VMs
```go
// Validate user access to VM through vApp's VDC organization membership
func (h *VMHandlers) validateVMAccess(ctx context.Context, userID, vmID string) (*models.VM, error) {
    vm, err := h.vmRepo.GetWithVApp(ctx, vmID)
    if err != nil {
        return nil, fmt.Errorf("VM access denied: %w", err)
    }
    
    // Check if user has access to the VDC containing this VM's vApp
    err = h.vdcHandlers.validateVDCAccess(ctx, userID, vm.VApp.VDCID)
    if err != nil {
        return nil, fmt.Errorf("VDC access denied: %w", err)
    }
    
    return vm, nil
}
```

#### OpenShift Integration for VM Details
The VM details are retrieved from both the database and the OpenShift VirtualMachine resource:

```go
func (s *VMService) GetVMDetails(ctx context.Context, vmID string) (*VMResponse, error) {
    // Get VM from database
    vm, err := s.vmRepo.GetWithVApp(ctx, vmID)
    if err != nil {
        return nil, err
    }
    
    // Get VirtualMachine from OpenShift for runtime status
    k8sVM := &kubevirtv1.VirtualMachine{}
    err = s.k8sClient.Get(ctx, types.NamespacedName{
        Name:      vm.Name, // VM name in OpenShift
        Namespace: vm.VApp.VDC.Namespace,
    }, k8sVM)
    if err != nil {
        return nil, fmt.Errorf("failed to get VM from OpenShift: %w", err)
    }
    
    // Combine database and OpenShift information
    return &VMResponse{
        ID:          vm.ID,
        Name:        vm.Name,
        Description: vm.Description,
        Status:      mapVMStatus(k8sVM.Status.Phase),
        VAppID:      vm.VAppID,
        TemplateID:  vm.TemplateID,
        CreatedAt:   vm.CreatedAt.Format(time.RFC3339),
        UpdatedAt:   vm.UpdatedAt.Format(time.RFC3339),
        GuestOS:     extractGuestOS(k8sVM),
        VMTools:     extractVMToolsInfo(k8sVM),
        Hardware:    extractHardwareInfo(k8sVM),
        StorageProfile: extractStorageProfile(k8sVM),
        NetworkConnections: extractNetworkInfo(k8sVM),
        Href:        fmt.Sprintf("/cloudapi/1.0.0/vms/%s", vm.ID),
    }, nil
}

// Map OpenShift VM phase to VCD status
func mapVMStatus(phase kubevirtv1.VirtualMachinePhase) string {
    switch phase {
    case kubevirtv1.VirtualMachinePhaseRunning:
        return "POWERED_ON"
    case kubevirtv1.VirtualMachinePhaseStopped:
        return "POWERED_OFF"
    case kubevirtv1.VirtualMachinePhaseStarting:
        return "POWERING_ON"
    case kubevirtv1.VirtualMachinePhaseStopping:
        return "POWERING_OFF"
    case kubevirtv1.VirtualMachinePhaseUnknown:
        return "UNKNOWN"
    default:
        return "UNKNOWN"
    }
}
```

#### VM Response Model
```go
type VMResponse struct {
    ID                 string                `json:"id"`
    Name               string                `json:"name"`
    Description        string                `json:"description"`
    Status             string                `json:"status"`
    VAppID             string                `json:"vappId"`
    TemplateID         *string               `json:"templateId,omitempty"`
    CreatedAt          string                `json:"createdAt"`
    UpdatedAt          string                `json:"updatedAt"`
    GuestOS            string                `json:"guestOs"`
    VMTools            VMToolsInfo           `json:"vmTools"`
    Hardware           HardwareInfo          `json:"hardware"`
    StorageProfile     StorageProfileInfo    `json:"storageProfile"`
    NetworkConnections []NetworkConnection   `json:"networkConnections"`
    Href               string                `json:"href"`
}

type VMToolsInfo struct {
    Status  string `json:"status"`
    Version string `json:"version"`
}

type HardwareInfo struct {
    NumCPUs           int `json:"numCpus"`
    NumCoresPerSocket int `json:"numCoresPerSocket"`
    MemoryMB          int `json:"memoryMB"`
}

type StorageProfileInfo struct {
    Name string `json:"name"`
    Href string `json:"href"`
}

type NetworkConnection struct {
    NetworkName string `json:"networkName"`
    IPAddress   string `json:"ipAddress"`
    MACAddress  string `json:"macAddress"`
    Connected   bool   `json:"connected"`
}
```

#### Route Configuration
```go
// Add VM route to CloudAPI group
cloudAPI.GET("/vms/:vm_id", s.vmHandlers.GetVM)
```

#### Database Model Updates
The existing VM model may need additional fields to support the full response:

```go
type VM struct {
    ID          string         `gorm:"type:varchar(255);primary_key" json:"id"`
    Name        string         `gorm:"not null" json:"name"`
    Description string         `json:"description"`
    VAppID      string         `gorm:"type:varchar(255);not null;index" json:"vapp_id"`
    TemplateID  *string        `gorm:"type:varchar(255);index" json:"template_id"`
    GuestOS     string         `json:"guest_os"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

    // Relationships
    VApp     *VApp         `gorm:"foreignKey:VAppID;references:ID" json:"vapp,omitempty"`
    Template *VAppTemplate `gorm:"foreignKey:TemplateID;references:ID" json:"template,omitempty"`
}
```

## Implementation Details

### 1. Access Control Validation

```go
// Validate user access to VDC
func (h *VMCreationHandlers) validateVDCAccess(ctx context.Context, userID, vdcID string) error {
    vdc, err := h.vdcRepo.GetAccessibleVDC(ctx, userID, vdcID)
    if err != nil {
        return fmt.Errorf("VDC access denied: %w", err)
    }
    return nil
}

// Validate user access to catalog item
func (h *VMCreationHandlers) validateCatalogItemAccess(ctx context.Context, userID, catalogItemID string) error {
    catalogItem, err := h.catalogItemRepo.GetAccessibleCatalogItem(ctx, userID, catalogItemID)
    if err != nil {
        return fmt.Errorf("catalog item access denied: %w", err)
    }
    return nil
}
```

### 2. Template Resolution

The OpenShift Template will be resolved using the catalog item name:
- **Template Name**: `catalogItem.Name`
- **Template Namespace**: `"openshift"` (fixed namespace for templates)

### 3. TemplateInstance Creation

```yaml
apiVersion: template.openshift.io/v1
kind: TemplateInstance
metadata:
  name: "vapp-{generated-id}"
  namespace: "{vdc-namespace}"
  labels:
    vdc.ssvirt.io/vdc-id: "{vdc-id}"
    vapp.ssvirt.io/vapp-id: "{vapp-id}"
    catalog.ssvirt.io/item-id: "{catalog-item-id}"
spec:
  template:
    name: "{catalog-item-name}"
    namespace: "openshift"
  # No parameters - using template defaults
```

### 4. Database Records

#### VApp Record
```go
vapp := &models.VApp{
    ID:          generateVAppURN(),
    Name:        request.Name,
    Description: request.Description,
    VDCID:       vdcID,
    TemplateID:  &templateID,
    Status:      "INSTANTIATING",
}
```


### 5. Error Handling

- **Template Not Found**: Return 404 if OpenShift Template doesn't exist
- **Namespace Access**: Return 403 if service account can't access VDC namespace
- **Resource Conflicts**: Return 409 if vApp name already exists in VDC
- **Template Instantiation Failure**: Return 500 with OpenShift error details

## Security Considerations

### Authentication & Authorization
1. **JWT Authentication**: Ensure valid JWT token with user claims
2. **Organization Membership**: Verify user belongs to VDC's organization
3. **Catalog Access**: Verify user belongs to catalog item's organization
4. **Cross-Organization Prevention**: Block access to templates from other organizations

### Resource Isolation
1. **Namespace Isolation**: TemplateInstance created only in user's accessible VDC namespace
2. **RBAC**: Service account has minimal required permissions
3. **Resource Quotas**: Respect VDC resource limits

### Input Validation
1. **URN Format**: Validate VDC ID follows proper URN format
2. **Name Constraints**: Enforce Kubernetes-compatible naming rules
3. **Length Limits**: Prevent excessively long names or descriptions
4. **Content Filtering**: Sanitize user input to prevent injection attacks

## OpenShift Integration

### Required Permissions

The service account needs permissions to:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ssvirt-vm-creator
rules:
- apiGroups: ["template.openshift.io"]
  resources: ["templates", "templateinstances"]
  verbs: ["get", "list", "create", "watch"]
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["get", "list", "watch"]
```

### Template Requirements

OpenShift Templates must:
1. Be located in the `openshift` namespace
2. Have names matching catalog item names exactly
3. Create VirtualMachine resources when instantiated
4. Include proper labels for tracking and cleanup

### Monitoring Integration

- **TemplateInstance Status**: Monitor for successful/failed instantiation
- **VirtualMachine Creation**: Track VM resource creation
- **Resource Usage**: Monitor namespace resource consumption
- **Error Logging**: Capture and log OpenShift API errors

## API Response Details

### vApp Object
```json
{
  "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "name": "my-vm-instance",
  "description": "Development VM for testing",
  "status": "INSTANTIATING|RESOLVED|DEPLOYED|SUSPENDED|FAILED",
  "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "numberOfVMs": 1,
  "href": "/cloudapi/1.0.0/vapps/{vapp_id}"
}
```

## Testing Strategy

### Unit Tests
1. **Request Validation**: Test input validation and error handling
2. **Access Control**: Test organization membership and permissions
3. **Template Resolution**: Test catalog item to template mapping
4. **Database Operations**: Test vApp record creation and updates

### Integration Tests
1. **End-to-End Flow**: Test complete VM creation process
2. **OpenShift Integration**: Test TemplateInstance creation and monitoring
3. **Error Scenarios**: Test various failure modes and recovery
4. **Concurrent Operations**: Test multiple simultaneous VM creations

### API Tests
1. **HTTP Endpoints**: Test all success and error response codes
2. **Authentication**: Test JWT token validation
3. **Authorization**: Test organization-based access control
4. **Response Format**: Validate JSON response schemas

## OpenShift Requirements

- Requires OpenShift 4.19+ for Template API support
- Compatible with OpenShift Virtualization 4.19+
- Templates must be available in the `openshift` namespace

## Implementation Plan

### Phase 1: Core Implementation
1. Create VM creation handlers and middleware
2. Implement access control validation
3. Add OpenShift Template integration
4. Create vApp database operations
5. Add comprehensive error handling

### Phase 2: Testing and Validation
1. Implement unit test suite
2. Create integration tests
3. Add API endpoint tests
4. Performance testing and optimization

### Phase 3: Documentation and Deployment
1. API documentation updates
2. User guide creation
3. Deployment configuration
4. Monitoring and logging setup

## Conclusion

This enhancement enables the core VM provisioning capability that users need while maintaining proper security and integration with OpenShift. The design follows established patterns from the existing codebase and provides a foundation for future enhancements.

**Note**: This API endpoint does not exist in the actual VMware Cloud Director APIs as far as we can determine, but it follows the intent and patterns of the VCD API for template instantiation while being adapted for our OpenShift-based implementation.