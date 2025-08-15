# VM Power Management API Enhancement

## Overview

This enhancement proposes adding power management capabilities to the SSVirt VM
API, enabling users to start and stop virtual machines through RESTful API
endpoints. The implementation will directly control KubeVirt VirtualMachine
resources by modifying their `spec.runStrategy` field to manage VM power states.

## Background

Currently, the SSVirt system provides VM management through database records and
status synchronization, but lacks the ability to actually control VM power
states. Users cannot start or stop their virtual machines through the API,
limiting the system's functionality for basic VM lifecycle management.

This enhancement addresses a fundamental requirement for any VM management platform: the ability to control VM power states programmatically.

## Goals

1. **Power Control API**: Provide RESTful endpoints for VM power management operations
2. **KubeVirt Integration**: Direct integration with KubeVirt VirtualMachine resources for power control
3. **VMware Cloud Director Compatibility**: API endpoints that match VMware Cloud Director patterns
4. **Immediate Effect**: Changes should take effect immediately in the OpenShift cluster
5. **Status Synchronization**: Leverage existing VM Status Controller for state updates
6. **Security**: Proper authorization and validation for power operations

## Non-Goals

- Advanced power operations (restart, suspend, resume) - future enhancements
- Scheduled power operations or automation
- Bulk power operations across multiple VMs
- VM snapshots or state preservation during power operations
- Complex power policies or dependencies

## API Design

### Power On Endpoint

```
POST /cloudapi/1.0.0/vms/{vm-id}/actions/powerOn
```

**Request:**
- Method: `POST`
- Path: `/cloudapi/1.0.0/vms/{vm-id}/actions/powerOn`
- Headers: `Authorization: Bearer <token>`
- Body: Empty JSON object `{}`

**Response:**
```json
{
  "id": "urn:vcloud:vm:12345678-1234-1234-1234-123456789abc",
  "name": "my-vm",
  "status": "POWERING_ON",
  "powerState": "POWERING_ON",
  "href": "/cloudapi/1.0.0/vms/12345678-1234-1234-1234-123456789abc"
}
```

**HTTP Status Codes:**
- `202 Accepted` - Power on operation initiated successfully
- `400 Bad Request` - VM is already powered on or in invalid state
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - VM not found
- `409 Conflict` - VM is in a conflicting state (e.g., being deleted)
- `500 Internal Server Error` - Server error during operation

### Power Off Endpoint

```
POST /cloudapi/1.0.0/vms/{vm-id}/actions/powerOff
```

**Request:**
- Method: `POST`
- Path: `/cloudapi/1.0.0/vms/{vm-id}/actions/powerOff`
- Headers: `Authorization: Bearer <token>`
- Body: Empty JSON object `{}`

**Response:**
```json
{
  "id": "urn:vcloud:vm:12345678-1234-1234-1234-123456789abc",
  "name": "my-vm",
  "status": "POWERING_OFF",
  "powerState": "POWERING_OFF",
  "href": "/cloudapi/1.0.0/vms/12345678-1234-1234-1234-123456789abc"
}
```

**HTTP Status Codes:**
- `202 Accepted` - Power off operation initiated successfully
- `400 Bad Request` - VM is already powered off or in invalid state
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - VM not found
- `409 Conflict` - VM is in a conflicting state (e.g., being deleted)
- `500 Internal Server Error` - Server error during operation

## Implementation Architecture

### Overview Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     SSVirt API Server                          │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Power Management Endpoints                     ││
│  │                                                             ││
│  │  POST /cloudapi/1.0.0/vms/{id}/actions/powerOn            ││
│  │  POST /cloudapi/1.0.0/vms/{id}/actions/powerOff           ││
│  │                                                             ││
│  │  ┌─────────────────┐  ┌─────────────────┐                 ││
│  │  │ Power On Handler│  │Power Off Handler│                 ││
│  │  │                 │  │                 │                 ││
│  │  │ - Validate VM   │  │ - Validate VM   │                 ││
│  │  │ - Check Auth    │  │ - Check Auth    │                 ││
│  │  │ - Update K8s    │  │ - Update K8s    │                 ││
│  │  └─────────────────┘  └─────────────────┘                 ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                               │                                 │
└───────────────────────────────┼─────────────────────────────────┘
                                │
                                ▼
                   ┌─────────────────────────────┐
                   │    Kubernetes API Server    │
                   │                             │
                   │  VirtualMachine Resources   │
                   │                             │
                   │  spec.runStrategy:          │
                   │  - "Always"   (Power On)    │
                   │  - "Halted"   (Power Off)   │
                   │                             │
                   └─────────────────────────────┘
                                │
                                ▼
                   ┌─────────────────────────────┐
                   │       KubeVirt/CDI         │
                   │                             │
                   │   VM Lifecycle Management   │
                   │                             │
                   │  - Start/Stop VMs          │
                   │  - Status Updates          │
                   │  - Resource Management     │
                   │                             │
                   └─────────────────────────────┘
                                │
                                ▼
                   ┌─────────────────────────────┐
                   │    VM Status Controller     │
                   │                             │
                   │   Database Synchronization  │
                   │                             │
                   │  - Watches VM Status       │
                   │  - Updates PostgreSQL      │
                   │  - Real-time Sync          │
                   │                             │
                   └─────────────────────────────┘
```

### Component Integration

1. **API Handlers**: New REST endpoints that validate requests and perform power operations
2. **Kubernetes Client**: Direct interaction with VirtualMachine resources
3. **Status Synchronization**: Leverage existing VM Status Controller for immediate database updates
4. **Authorization**: Integrate with existing RBAC and authentication systems

## Implementation Details

### 1. API Handler Structure

```go
// Required imports for implementation
import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "gorm.io/gorm"
    k8serrors "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/types"
    kubevirtv1 "kubevirt.io/api/core/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

// PowerManagementHandler handles VM power operations
type PowerManagementHandler struct {
    vmRepo          *repositories.VMRepository
    k8sClient       client.Client
    logger          *slog.Logger
    authMiddleware  *auth.Middleware
}

// PowerOnRequest represents a power on operation request
type PowerOnRequest struct {
    // Empty for now, but allows for future parameters
}

// PowerOffRequest represents a power off operation request  
type PowerOffRequest struct {
    // Empty for now, but allows for future parameters
}

// PowerOperationResponse represents the response from power operations
type PowerOperationResponse struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    Status     string `json:"status"`
    PowerState string `json:"powerState"`
    Href       string `json:"href"`
}
```

### 2. Power On Implementation

```go
// PowerOn handles VM power on requests
func (h *PowerManagementHandler) PowerOn(c *gin.Context) {
    ctx := c.Request.Context()
    vmID := c.Param("id")
    
    // Validate VM ID format
    if !isValidUUID(vmID) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid VM ID format",
        })
        return
    }
    
    // Find VM record in database
    vm, err := h.vmRepo.GetByID(ctx, vmID)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusNotFound, gin.H{
                "error": "VM not found",
            })
            return
        }
        h.logger.Error("Failed to find VM", "vmID", vmID, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    // Check if VM is already powered on
    if vm.Status == "POWERED_ON" || vm.Status == "POWERING_ON" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "VM is already powered on or powering on",
        })
        return
    }
    
    // Check for conflicting states
    if vm.Status == "DELETING" || vm.Status == "DELETED" {
        c.JSON(http.StatusConflict, gin.H{
            "error": "VM is in a conflicting state",
        })
        return
    }
    
    // Get the VirtualMachine resource from Kubernetes
    vmResource := &kubevirtv1.VirtualMachine{}
    vmKey := types.NamespacedName{
        Name:      vm.VMName,
        Namespace: vm.Namespace,
    }
    
    err = h.k8sClient.Get(ctx, vmKey, vmResource)
    if err != nil {
        if k8serrors.IsNotFound(err) {
            c.JSON(http.StatusNotFound, gin.H{
                "error": "VirtualMachine resource not found in cluster",
            })
            return
        }
        h.logger.Error("Failed to get VirtualMachine resource", 
            "vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to access VM resource",
        })
        return
    }
    
    // Patch the VirtualMachine spec to power on using strategic merge patch
    runStrategy := kubevirtv1.RunStrategyAlways
    
    // Create patch to update only the runStrategy field
    patchData := map[string]interface{}{
        "spec": map[string]interface{}{
            "runStrategy": runStrategy,
        },
    }
    
    patchBytes, err := json.Marshal(patchData)
    if err != nil {
        h.logger.Error("Failed to marshal patch data", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to prepare VM update",
        })
        return
    }
    
    err = h.k8sClient.Patch(ctx, vmResource, client.RawPatch(types.MergePatchType, patchBytes))
    if err != nil {
        h.logger.Error("Failed to patch VirtualMachine run strategy", 
            "vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to power on VM",
        })
        return
    }
    
    h.logger.Info("VM power on initiated", 
        "vmID", vmID, "vmName", vm.VMName, "namespace", vm.Namespace)
    
    // Return response (status will be updated by VM Status Controller)
    response := PowerOperationResponse{
        ID:         formatVMURN(vmID),
        Name:       vm.Name,
        Status:     "POWERING_ON",
        PowerState: "POWERING_ON", 
        Href:       fmt.Sprintf("/cloudapi/1.0.0/vms/%s", vmID),
    }
    
    c.JSON(http.StatusAccepted, response)
}
```

### 3. Power Off Implementation

```go
// PowerOff handles VM power off requests
func (h *PowerManagementHandler) PowerOff(c *gin.Context) {
    ctx := c.Request.Context()
    vmID := c.Param("id")
    
    // Validate VM ID format
    if !isValidUUID(vmID) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid VM ID format",
        })
        return
    }
    
    // Find VM record in database
    vm, err := h.vmRepo.GetByID(ctx, vmID)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            c.JSON(http.StatusNotFound, gin.H{
                "error": "VM not found",
            })
            return
        }
        h.logger.Error("Failed to find VM", "vmID", vmID, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    // Check if VM is already powered off
    if vm.Status == "POWERED_OFF" || vm.Status == "POWERING_OFF" || vm.Status == "STOPPED" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "VM is already powered off or powering off",
        })
        return
    }
    
    // Check for conflicting states
    if vm.Status == "DELETING" || vm.Status == "DELETED" {
        c.JSON(http.StatusConflict, gin.H{
            "error": "VM is in a conflicting state",
        })
        return
    }
    
    // Get the VirtualMachine resource from Kubernetes
    vmResource := &kubevirtv1.VirtualMachine{}
    vmKey := types.NamespacedName{
        Name:      vm.VMName,
        Namespace: vm.Namespace,
    }
    
    err = h.k8sClient.Get(ctx, vmKey, vmResource)
    if err != nil {
        if k8serrors.IsNotFound(err) {
            c.JSON(http.StatusNotFound, gin.H{
                "error": "VirtualMachine resource not found in cluster",
            })
            return
        }
        h.logger.Error("Failed to get VirtualMachine resource", 
            "vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to access VM resource",
        })
        return
    }
    
    // Patch the VirtualMachine spec to power off using strategic merge patch
    runStrategy := kubevirtv1.RunStrategyHalted
    
    // Create patch to update only the runStrategy field
    patchData := map[string]interface{}{
        "spec": map[string]interface{}{
            "runStrategy": runStrategy,
        },
    }
    
    patchBytes, err := json.Marshal(patchData)
    if err != nil {
        h.logger.Error("Failed to marshal patch data", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to prepare VM update",
        })
        return
    }
    
    err = h.k8sClient.Patch(ctx, vmResource, client.RawPatch(types.MergePatchType, patchBytes))
    if err != nil {
        h.logger.Error("Failed to patch VirtualMachine run strategy", 
            "vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to power off VM",
        })
        return
    }
    
    h.logger.Info("VM power off initiated", 
        "vmID", vmID, "vmName", vm.VMName, "namespace", vm.Namespace)
    
    // Return response (status will be updated by VM Status Controller)
    response := PowerOperationResponse{
        ID:         formatVMURN(vmID),
        Name:       vm.Name,
        Status:     "POWERING_OFF",
        PowerState: "POWERING_OFF",
        Href:       fmt.Sprintf("/cloudapi/1.0.0/vms/%s", vmID),
    }
    
    c.JSON(http.StatusAccepted, response)
}
```

### 4. Helper Functions

```go
// isValidUUID validates UUID format
func isValidUUID(u string) bool {
    _, err := uuid.Parse(u)
    return err == nil
}

// formatVMURN formats VM ID as VMware Cloud Director URN
func formatVMURN(vmID string) string {
    return fmt.Sprintf("urn:vcloud:vm:%s", vmID)
}

// validateVMPowerState checks if the VM is in a valid state for power operations
func validateVMPowerState(status string, operation string) error {
    switch operation {
    case "powerOn":
        if status == "POWERED_ON" || status == "POWERING_ON" {
            return fmt.Errorf("VM is already powered on or powering on")
        }
    case "powerOff":
        if status == "POWERED_OFF" || status == "POWERING_OFF" || status == "STOPPED" {
            return fmt.Errorf("VM is already powered off or powering off")
        }
    }
    
    // Check for conflicting states
    if status == "DELETING" || status == "DELETED" {
        return fmt.Errorf("VM is in a conflicting state: %s", status)
    }
    
    return nil
}
```

### 5. Router Integration

```go
// RegisterPowerManagementRoutes adds power management endpoints to the router
func RegisterPowerManagementRoutes(router *gin.RouterGroup, handler *PowerManagementHandler, authMiddleware *auth.Middleware) {
    powerGroup := router.Group("/vms/:id/actions")
    powerGroup.Use(authMiddleware.RequireAuth())
    
    powerGroup.POST("/powerOn", handler.PowerOn)
    powerGroup.POST("/powerOff", handler.PowerOff)
}
```

## Database Changes

**No database schema changes are required** for this enhancement. The existing VM table structure is sufficient:

- Power operations update KubeVirt VirtualMachine resources directly
- The VM Status Controller automatically synchronizes status changes back to the database
- Existing `status` field captures the power state transitions

## RBAC Requirements

The API server needs additional Kubernetes permissions to manage VirtualMachine resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ssvirt-api-server-vm-power-management
rules:
# Read VirtualMachine resources
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["get", "list", "watch"]
# Patch VirtualMachine specs for power management (runStrategy field only)
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["patch"]
# Read VirtualMachine status
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines/status"]
  verbs: ["get"]
```

## Configuration

Add power management configuration options:

```go
type PowerManagementConfig struct {
    Enabled              bool          `yaml:"enabled" env:"POWER_MANAGEMENT_ENABLED" envDefault:"true"`
    OperationTimeout     time.Duration `yaml:"operationTimeout" env:"POWER_OPERATION_TIMEOUT" envDefault:"5m"`
    MaxConcurrentOps     int           `yaml:"maxConcurrentOps" env:"MAX_CONCURRENT_POWER_OPS" envDefault:"10"`
    EnableMetrics        bool          `yaml:"enableMetrics" env:"POWER_METRICS_ENABLED" envDefault:"true"`
}
```

## Monitoring and Observability

### Metrics

Add Prometheus metrics for power operations:

```go
var (
    powerOperationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ssvirt_vm_power_operations_total",
            Help: "Total number of VM power operations",
        },
        []string{"operation", "result", "namespace"},
    )
    
    powerOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ssvirt_vm_power_operation_duration_seconds",
            Help: "Time taken for VM power operations",
        },
        []string{"operation", "namespace"},
    )
    
    concurrentPowerOperations = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ssvirt_vm_concurrent_power_operations",
            Help: "Number of concurrent VM power operations",
        },
        []string{"operation"},
    )
)
```

### Logging

Structured logging for power operations:

```go
logger.Info("VM power operation initiated",
    "operation", "powerOn",
    "vmID", vmID,
    "vmName", vm.VMName,
    "namespace", vm.Namespace,
    "userID", userID,
    "requestID", requestID)

logger.Error("VM power operation failed",
    "operation", "powerOn", 
    "vmID", vmID,
    "vmName", vm.VMName,
    "namespace", vm.Namespace,
    "error", err,
    "duration", duration)
```

## Error Handling

### Comprehensive Error Scenarios

1. **VM Not Found**: Return 404 with clear error message
2. **Invalid State**: Return 400 for operations on VMs in incompatible states
3. **Kubernetes Errors**: Handle connection failures, permission errors, and API server issues
4. **Concurrent Operations**: Prevent conflicting power operations on the same VM
5. **Timeout Handling**: Set reasonable timeouts for Kubernetes operations

### Error Response Format

```json
{
  "error": "VM is already powered on",
  "code": "VM_ALREADY_POWERED_ON",
  "details": {
    "vmId": "12345678-1234-1234-1234-123456789abc",
    "currentStatus": "POWERED_ON",
    "requestedOperation": "powerOn"
  }
}
```

## Testing Strategy

### Unit Tests

```go
func TestPowerOnHandler_Success(t *testing.T) {
    // Setup test environment with mock repositories and Kubernetes client
    // Test successful power on operation
}

func TestPowerOnHandler_VMAlreadyPoweredOn(t *testing.T) {
    // Test handling of VM already in powered on state
}

func TestPowerOnHandler_VMNotFound(t *testing.T) {
    // Test handling of non-existent VM
}

func TestPowerOffHandler_Success(t *testing.T) {
    // Test successful power off operation
}

func TestPowerManagement_Authorization(t *testing.T) {
    // Test that unauthorized users cannot perform power operations
}

func TestPowerManagement_InvalidUUID(t *testing.T) {
    // Test handling of malformed VM IDs
}
```

### Integration Tests

```go
func TestPowerManagement_EndToEnd(t *testing.T) {
    // Create test VM in database and Kubernetes
    // Test complete power on/off cycle
    // Verify status synchronization through VM Status Controller
}

func TestPowerManagement_KubernetesIntegration(t *testing.T) {
    // Test actual interaction with KubeVirt VirtualMachine resources
    // Verify spec.runStrategy updates
}
```

## Security Considerations

### Authorization

1. **RBAC Integration**: Use existing SSVirt RBAC system
2. **Resource Access**: Users can only power manage VMs they own
3. **Namespace Isolation**: Respect VDC namespace boundaries
4. **Audit Logging**: Log all power operations for compliance

### Input Validation

1. **UUID Validation**: Strict validation of VM ID format
2. **Request Validation**: Validate JSON request bodies
3. **State Validation**: Ensure VM is in valid state for operation
4. **Rate Limiting**: Prevent abuse through rate limiting

## Performance Considerations

### Efficiency Optimizations

1. **Direct Kubernetes Updates**: Minimal overhead through direct VirtualMachine updates
2. **Asynchronous Operations**: Power operations return immediately with accepted status
3. **Status Synchronization**: Leverage existing VM Status Controller for efficient updates
4. **Connection Pooling**: Reuse Kubernetes client connections

### Scalability

1. **Concurrent Operations**: Support multiple simultaneous power operations
2. **Resource Limits**: Configurable limits on concurrent operations
3. **Timeout Management**: Appropriate timeouts for Kubernetes operations
4. **Error Recovery**: Robust error handling and retry mechanisms

## Implementation Plan

### Phase 1: Core API Implementation (Week 1)
1. **API Handlers**: Implement power on/off endpoint handlers
2. **Kubernetes Integration**: Add VirtualMachine resource management
3. **Basic Validation**: Implement UUID and state validation
4. **Error Handling**: Comprehensive error response handling

### Phase 2: Integration and Testing (Week 2)
1. **RBAC Integration**: Add required Kubernetes permissions
2. **Router Setup**: Integrate endpoints into existing API routes
3. **Unit Tests**: Comprehensive test coverage for handlers
4. **Integration Tests**: End-to-end testing with real Kubernetes

### Phase 3: Observability and Production Readiness (Week 3)
1. **Metrics Integration**: Prometheus metrics for power operations
2. **Structured Logging**: Comprehensive logging for troubleshooting
3. **Configuration**: Add power management configuration options
4. **Documentation**: API documentation and operational guides

### Phase 4: Performance and Security (Week 4)
1. **Performance Testing**: Load testing and optimization
2. **Security Review**: Authorization and input validation audit
3. **Error Scenarios**: Comprehensive error handling testing
4. **Production Validation**: End-to-end testing in staging environment

## Dependencies

### Required Changes

1. **API Server**: New endpoint handlers and Kubernetes client integration
2. **RBAC**: Additional ClusterRole permissions for VirtualMachine management
3. **Configuration**: Power management configuration section

### No Changes Required

1. **Database Schema**: Existing VM table structure is sufficient
2. **VM Status Controller**: Current implementation handles status synchronization
3. **Authentication**: Existing authentication system works without changes

## KubeVirt Integration Details

### Run Strategy Values

KubeVirt VirtualMachine `spec.runStrategy` field accepts these values:

- `Always`: VM should be running (power on)
- `RerunOnFailure`: VM should restart if it fails (not used in this enhancement)
- `Manual`: VM state is manually controlled (not used in this enhancement)  
- `Halted`: VM should be stopped (power off)

### VirtualMachine Resource Updates

```yaml
# Power On: Set runStrategy to Always
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: my-vdc
spec:
  runStrategy: Always  # This starts the VM
  template:
    # VM template specification...

# Power Off: Set runStrategy to Halted  
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: my-vdc
spec:
  runStrategy: Halted  # This stops the VM
  template:
    # VM template specification...
```

### Status Synchronization Flow

1. **API Request**: User calls `/vms/{id}/actions/powerOn`
2. **Kubernetes Patch**: API server patches VirtualMachine `spec.runStrategy` field only
3. **KubeVirt Processing**: KubeVirt controller processes the change
4. **VM State Change**: Actual VM starts/stops in the cluster
5. **Status Update**: VirtualMachine `status.printableStatus` reflects new state
6. **Database Sync**: VM Status Controller updates PostgreSQL with new status

## VMware Cloud Director API Compatibility

The proposed endpoints follow VMware Cloud Director patterns:

- **URL Structure**: `/cloudapi/1.0.0/vms/{vm-id}/actions/{action}`
- **HTTP Methods**: POST for all actions (matches VMware pattern)
- **Response Format**: JSON with VM details and status
- **Status Codes**: Standard HTTP status codes for different scenarios
- **URN Format**: `urn:vcloud:vm:{uuid}` for VM identification

## Conclusion

The VM Power Management API enhancement provides essential VM lifecycle control through simple, RESTful endpoints. By leveraging KubeVirt's `runStrategy` field and the existing VM Status Controller, this implementation achieves:

1. **Immediate Power Control**: Direct VM start/stop operations
2. **Seamless Integration**: Works with existing SSVirt architecture  
3. **Real-time Synchronization**: Automatic database updates through VM Status Controller
4. **VMware Compatibility**: API patterns consistent with VMware Cloud Director
5. **Production Ready**: Comprehensive error handling, security, and observability

The enhancement requires minimal changes to existing systems while providing fundamental VM management capabilities that users expect from a cloud management platform.