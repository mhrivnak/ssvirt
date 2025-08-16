# VM Instance Details Synchronization Enhancement

## Overview

This enhancement proposes extending the existing VM Status Controller to watch VirtualMachineInstance resources and populate the existing VM database fields with accurate runtime information. This ensures fields like `cpu_count`, `memory_mb`, and `guest_os` reflect actual running VM specifications rather than remaining empty or containing template defaults.

## Background

Currently, the VM Status Controller only synchronizes basic status information between OpenShift VirtualMachine resources and PostgreSQL VM records. The VM database model includes fields for `cpu_count`, `memory_mb`, and `guest_os`, but these are often empty or contain only template-level information rather than actual runtime details.

When a VirtualMachine is running, there is a corresponding VirtualMachineInstance (VMI) resource that contains accurate runtime details about the actual running VM instance.

### Current State of VM Fields

The existing VM model includes several fields that could be populated from VirtualMachineInstance data:

```go
type VM struct {
    ID          string         // URN identifier
    Name        string         // Display name
    Description string         // User description
    VAppID      string         // Parent vApp reference
    VMName      string         // OpenShift VM resource name
    Namespace   string         // OpenShift namespace
    Status      string         // VM status (already synchronized)
    CPUCount    *int           // CPU count - CURRENTLY EMPTY/INACCURATE
    MemoryMB    *int           // Memory in MB - CURRENTLY EMPTY/INACCURATE  
    GuestOS     string         // Guest OS info - CURRENTLY EMPTY
    CreatedAt   time.Time      // Creation timestamp
    UpdatedAt   time.Time      // Last update timestamp
}
```

### Current Limitations

1. **Empty CPU Count**: `cpu_count` field is often null or contains template defaults
2. **Inaccurate Memory**: `memory_mb` field is often null or doesn't reflect actual allocation
3. **Missing Guest OS**: `guest_os` field is empty as there's no mechanism to detect the actual OS
4. **Stale Information**: Fields may contain template values that don't match runtime configuration
5. **Inconsistent Data**: Database values don't reflect actual running VM specifications

### Rich Data Available in VirtualMachineInstance

VirtualMachineInstance resources provide the exact runtime information needed:

- **CPU Information**: `status.currentCPUTopology.cores` for actual CPU count
- **Memory Information**: `status.memory.guestCurrent` for actual memory allocation
- **Guest OS Information**: `status.guestOSInfo` with OS type, name, and version
- **Runtime Accuracy**: Reflects the actual running VM configuration, not template defaults

## Goals

1. **Accurate CPU Data**: Populate `cpu_count` with actual running CPU cores from VMI
2. **Accurate Memory Data**: Populate `memory_mb` with actual memory allocation from VMI
3. **Guest OS Detection**: Populate `guest_os` with detected OS information from VMI guest agent
4. **Real-time Updates**: Synchronize VMI changes to database in real-time using watch events
5. **Graceful Handling**: Handle VM states where VirtualMachineInstance may not exist
6. **Zero Schema Changes**: Use only existing database fields and structure

## Non-Goals

- Adding new database fields or schema changes
- Modifying VirtualMachineInstance resources based on database changes
- Historical tracking of VM specifications over time
- Real-time performance monitoring (CPU usage, memory utilization)
- Managing VM lifecycle operations through VMI manipulation

## Architecture Overview

### Enhanced Controller Design

The existing VM Status Controller will be extended to also watch VirtualMachineInstance resources and extract specific data to populate the existing `cpu_count`, `memory_mb`, and `guest_os` fields.

```
┌─────────────────────────────────────────────────────────────────┐
│                        OpenShift Cluster                       │
│                                                                 │
│  ┌─────────────────────────────┐  ┌─────────────────────────────┐ │
│  │      SSVirt Namespace       │  │      VDC Namespaces        │ │
│  │                             │  │                             │ │
│  │  ┌─────────────────────────┐│  │  ┌─────────────────────────┐│ │
│  │  │  Enhanced VM Controller ││  │  │   VirtualMachine        ││ │
│  │  │  (ssvirt-vm-controller) ││  │  │   Resources             ││ │
│  │  │                         ││  │  │                         ││ │
│  │  │  - VM Resource Watcher  ││  │  │  - VM definitions       ││ │
│  │  │  - VMI Resource Watcher ││◄─┼─┼──│  - Status & specs      ││ │
│  │  │  - Status Reconciler    ││  │  │                         ││ │
│  │  │  - VMI Data Extractor   ││  │  │  ┌─────────────────────┐││ │
│  │  │  - Database Updater     ││  │  │  │ VirtualMachine-     │││ │
│  │  │  - Leader Election      ││  │  │  │ Instance Resources  │││ │
│  │  └─────────────────────────┘│  │  │  │                     │││ │
│  │             │               │  │  │  │ - CPU topology      │││ │
│  └─────────────┼───────────────┘  │  │  │ - Memory allocation │││ │
│               │                  │  │  │ - Guest OS info     │││ │
│               │                  │  │  └─────────────────────┘││ │
│               │                  │  └─────────────────────────┘│ │
└───────────────┼─────────────────────────────────────────────────┘
                │
                ▼
      ┌─────────────────┐
      │   PostgreSQL    │
      │   Database      │
      │                 │
      │ ┌─────────────┐ │
      │ │ VM Records  │ │
      │ │             │ │
      │ │ - cpu_count │ │
      │ │ - memory_mb │ │
      │ │ - guest_os  │ │
      │ │ - status    │ │
      │ └─────────────┘ │
      └─────────────────┘
```

## Implementation Details

### 1. Enhanced Controller Structure

Extend the existing VMStatusController to also watch VirtualMachineInstance resources:

```go
// Enhanced VMStatusController watches both VirtualMachine and VirtualMachineInstance resources
type VMStatusController struct {
    client.Client
    Scheme    *runtime.Scheme
    VMRepo    *repositories.VMRepository
    Recorder  record.EventRecorder
    Log       logr.Logger
}

// SetupVMStatusController configures watches for both VM and VMI resources
func SetupVMStatusController(mgr ctrl.Manager, vmRepo *repositories.VMRepository) error {
    controller := &VMStatusController{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        VMRepo:   vmRepo,
        Recorder: mgr.GetEventRecorderFor("vm-status-controller"),
        Log:      ctrl.Log.WithName("controllers").WithName("VMStatus"),
    }
    
    return ctrl.NewControllerManagedBy(mgr).
        For(&kubevirtv1.VirtualMachine{}).                    // Watch VMs
        Watches(&kubevirtv1.VirtualMachineInstance{},         // Watch VMIs
            handler.EnqueueRequestsFromMapFunc(controller.mapVMIToVM)).
        WithOptions(controller.Options{
            MaxConcurrentReconciles: 5,
            RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
                time.Second,
                time.Minute*5,
            ),
        }).
        Complete(controller)
}
```

### 2. VMI to VM Mapping

Map VirtualMachineInstance events to corresponding VirtualMachine reconcile requests:

```go
// mapVMIToVM maps VirtualMachineInstance events to VirtualMachine reconcile requests
func (r *VMStatusController) mapVMIToVM(ctx context.Context, obj client.Object) []reconcile.Request {
    vmi, ok := obj.(*kubevirtv1.VirtualMachineInstance)
    if !ok {
        return nil
    }
    
    // VMI should have OwnerReference to VirtualMachine
    for _, owner := range vmi.OwnerReferences {
        if owner.Kind == "VirtualMachine" && owner.APIVersion == "kubevirt.io/v1" {
            return []reconcile.Request{
                {
                    NamespacedName: types.NamespacedName{
                        Namespace: vmi.Namespace,
                        Name:      owner.Name,
                    },
                },
            }
        }
    }
    
    // Fallback: assume VMI name matches VM name
    return []reconcile.Request{
        {
            NamespacedName: types.NamespacedName{
                Namespace: vmi.Namespace,
                Name:      vmi.Name,
            },
        },
    }
}
```

### 3. Enhanced Reconciliation Logic

Extend the reconcile method to handle both VM status and VMI data:

```go
// Reconcile handles both VirtualMachine and VirtualMachineInstance changes
func (r *VMStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := r.Log.WithValues("virtualmachine", req.NamespacedName)
    
    // Fetch the VirtualMachine resource
    vm := &kubevirtv1.VirtualMachine{}
    err := r.Get(ctx, req.NamespacedName, vm)
    if err != nil {
        if errors.IsNotFound(err) {
            // Handle VM deletion
            return r.handleVMDeletion(ctx, req.NamespacedName)
        }
        return ctrl.Result{}, err
    }
    
    // Find corresponding database record
    vmRecord, err := r.findVMRecord(ctx, vm)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // VM not managed by SSVirt, skip
            log.V(1).Info("VirtualMachine not managed by SSVirt, skipping")
            return ctrl.Result{}, nil
        }
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    // Handle VM status update (existing logic)
    statusResult, err := r.handleVMStatusUpdate(ctx, vm, vmRecord)
    if err != nil {
        return statusResult, err
    }
    
    // Handle VMI data update (new logic)
    vmiResult, err := r.handleVMIDataUpdate(ctx, vm, vmRecord)
    if err != nil {
        return vmiResult, err
    }
    
    // Return the more restrictive result
    if statusResult.RequeueAfter > 0 || vmiResult.RequeueAfter > 0 {
        requeue := statusResult.RequeueAfter
        if vmiResult.RequeueAfter > 0 && vmiResult.RequeueAfter < requeue {
            requeue = vmiResult.RequeueAfter
        }
        return ctrl.Result{RequeueAfter: requeue}, nil
    }
    
    return ctrl.Result{}, nil
}
```

### 4. VMI Data Extraction

Extract specific data from VirtualMachineInstance to populate existing VM fields:

```go
// VMIData represents the data we extract from VirtualMachineInstance
type VMIData struct {
    CPUCount *int    // From status.currentCPUTopology.cores
    MemoryMB *int    // From status.memory.guestCurrent (converted to MB)
    GuestOS  string  // From status.guestOSInfo (formatted string)
}

// handleVMIDataUpdate processes VirtualMachineInstance data for existing fields
func (r *VMStatusController) handleVMIDataUpdate(ctx context.Context, vm *kubevirtv1.VirtualMachine, vmRecord *models.VM) (ctrl.Result, error) {
    // Try to find corresponding VMI
    vmi := &kubevirtv1.VirtualMachineInstance{}
    err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, vmi)
    
    if err != nil {
        if errors.IsNotFound(err) {
            // VMI doesn't exist - VM is not running, use VM spec defaults
            return r.handleVMSpecData(ctx, vm, vmRecord)
        }
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    // Extract data from VMI
    vmiData := extractVMIData(vmi)
    
    // Check if update is needed
    if !r.needsVMDataUpdate(vmRecord, vmiData) {
        return ctrl.Result{}, nil
    }
    
    // Update database record with VMI data
    err = r.VMRepo.UpdateVMData(ctx, vmRecord.ID, vmiData.CPUCount, vmiData.MemoryMB, vmiData.GuestOS)
    if err != nil {
        r.Recorder.Event(vm, corev1.EventTypeWarning, "VMDataUpdateFailed", 
            fmt.Sprintf("Failed to update VM data: %v", err))
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Updated VM data from VMI", 
        "vm", vm.Name, 
        "namespace", vm.Namespace, 
        "vmID", vmRecord.ID,
        "cpuCount", vmiData.CPUCount,
        "memoryMB", vmiData.MemoryMB,
        "guestOS", vmiData.GuestOS)
    
    return ctrl.Result{}, nil
}

// extractVMIData extracts relevant data from VirtualMachineInstance
func extractVMIData(vmi *kubevirtv1.VirtualMachineInstance) VMIData {
    data := VMIData{}
    
    // Extract CPU count from current topology
    if vmi.Status.CurrentCPUTopology != nil {
        totalCores := vmi.Status.CurrentCPUTopology.Cores * 
                     vmi.Status.CurrentCPUTopology.Sockets * 
                     vmi.Status.CurrentCPUTopology.Threads
        data.CPUCount = &totalCores
    }
    
    // Extract memory from current allocation
    if vmi.Status.Memory != nil && vmi.Status.Memory.GuestCurrent != nil {
        // Convert from resource.Quantity to MB
        memoryBytes := vmi.Status.Memory.GuestCurrent.Value()
        memoryMB := int(memoryBytes / (1024 * 1024))
        data.MemoryMB = &memoryMB
    }
    
    // Extract guest OS information
    if vmi.Status.GuestOSInfo != nil {
        guestOS := formatGuestOS(vmi.Status.GuestOSInfo)
        data.GuestOS = guestOS
    }
    
    return data
}

// formatGuestOS creates a formatted string from guest OS info
func formatGuestOS(osInfo *kubevirtv1.VirtualMachineInstanceGuestOSInfo) string {
    if osInfo.PrettyName != "" {
        return osInfo.PrettyName
    }
    
    if osInfo.Name != "" && osInfo.Version != "" {
        return fmt.Sprintf("%s %s", osInfo.Name, osInfo.Version)
    }
    
    if osInfo.Name != "" {
        return osInfo.Name
    }
    
    if osInfo.ID != "" {
        return osInfo.ID
    }
    
    return "Unknown"
}
```

### 5. Handle VM Not Running

When VirtualMachineInstance doesn't exist, extract data from VirtualMachine spec:

```go
// handleVMSpecData extracts data from VirtualMachine spec when VMI doesn't exist
func (r *VMStatusController) handleVMSpecData(ctx context.Context, vm *kubevirtv1.VirtualMachine, vmRecord *models.VM) (ctrl.Result, error) {
    // Extract data from VM spec
    specData := extractVMSpecData(vm)
    
    // Check if update is needed
    if !r.needsVMDataUpdate(vmRecord, specData) {
        return ctrl.Result{}, nil
    }
    
    // Update database record with VM spec data
    err := r.VMRepo.UpdateVMData(ctx, vmRecord.ID, specData.CPUCount, specData.MemoryMB, specData.GuestOS)
    if err != nil {
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Updated VM data from VM spec", 
        "vm", vm.Name, 
        "namespace", vm.Namespace, 
        "vmID", vmRecord.ID)
    
    return ctrl.Result{}, nil
}

// extractVMSpecData extracts data from VirtualMachine specification
func extractVMSpecData(vm *kubevirtv1.VirtualMachine) VMIData {
    data := VMIData{}
    
    // Extract CPU from VM spec
    if vm.Spec.Template.Spec.Domain.CPU != nil {
        cores := vm.Spec.Template.Spec.Domain.CPU.Cores
        sockets := vm.Spec.Template.Spec.Domain.CPU.Sockets
        threads := vm.Spec.Template.Spec.Domain.CPU.Threads
        
        totalCores := cores * sockets * threads
        data.CPUCount = &totalCores
    }
    
    // Extract memory from VM spec
    if vm.Spec.Template.Spec.Domain.Memory != nil && vm.Spec.Template.Spec.Domain.Memory.Guest != nil {
        memoryBytes := vm.Spec.Template.Spec.Domain.Memory.Guest.Value()
        memoryMB := int(memoryBytes / (1024 * 1024))
        data.MemoryMB = &memoryMB
    }
    
    // For guest OS, check annotations or labels for OS hints
    if vm.Annotations != nil {
        if osHint, exists := vm.Annotations["vm.kubevirt.io/os"]; exists {
            data.GuestOS = osHint
        }
    }
    
    return data
}
```

### 6. Repository Updates

Add method to VMRepository for updating VM data fields:

```go
// UpdateVMData updates the CPU, memory, and guest OS fields for a VM
func (r *VMRepository) UpdateVMData(ctx context.Context, vmID string, cpuCount *int, memoryMB *int, guestOS string) error {
    updates := map[string]interface{}{
        "updated_at": time.Now(),
    }
    
    // Only update non-nil values
    if cpuCount != nil {
        updates["cpu_count"] = *cpuCount
    }
    if memoryMB != nil {
        updates["memory_mb"] = *memoryMB
    }
    if guestOS != "" {
        updates["guest_os"] = guestOS
    }
    
    return r.db.WithContext(ctx).
        Model(&models.VM{}).
        Where("id = ?", vmID).
        Updates(updates).Error
}
```

### 7. Change Detection

Implement efficient change detection to minimize database writes:

```go
// needsVMDataUpdate checks if database update is actually needed
func (r *VMStatusController) needsVMDataUpdate(vmRecord *models.VM, newData VMIData) bool {
    // Check CPU count change
    if newData.CPUCount != nil {
        if vmRecord.CPUCount == nil || *vmRecord.CPUCount != *newData.CPUCount {
            return true
        }
    }
    
    // Check memory change
    if newData.MemoryMB != nil {
        if vmRecord.MemoryMB == nil || *vmRecord.MemoryMB != *newData.MemoryMB {
            return true
        }
    }
    
    // Check guest OS change
    if newData.GuestOS != "" && vmRecord.GuestOS != newData.GuestOS {
        return true
    }
    
    return false
}
```

## RBAC Requirements

Add VirtualMachineInstance permissions to the existing controller role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ssvirt-vm-status-controller
rules:
# Existing VM permissions
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines/status"]
  verbs: ["get", "list"]

# NEW: VirtualMachineInstance permissions
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachineinstances"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachineinstances/status"]
  verbs: ["get", "list"]

# Existing permissions
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```

## API Response Enhancement

The existing VM API endpoints will automatically include the populated fields:

```json
{
  "id": "urn:vcloud:vm:550e8400-e29b-41d4-a716-446655440000",
  "name": "web-server-01",
  "vmName": "centos-stream9-t2cuhsdg42mv5lf3",
  "status": "POWERED_ON",
  "namespace": "vdc-clowns-east1",
  "vappId": "urn:vcloud:vapp:550e8400-e29b-41d4-a716-446655440001",
  "description": "Web server VM",
  "cpu_count": 2,
  "memory_mb": 4096,
  "guest_os": "CentOS Stream 9",
  "createdAt": "2025-08-16T15:20:00Z",
  "updatedAt": "2025-08-16T15:25:30Z",
  "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:550e8400-e29b-41d4-a716-446655440000"
}
```

## Performance Considerations

### Optimizations

1. **Change Detection**: Only update database when values actually change
2. **Selective Updates**: Update only the fields that have changed
3. **Efficient Queries**: Use targeted database updates instead of full record replacement
4. **Minimal Processing**: Extract only the three needed fields from VMI

### Data Sources Priority

1. **Running VM**: Use VirtualMachineInstance data for accurate runtime information
2. **Stopped VM**: Use VirtualMachine spec data for configured values
3. **Fallback**: Preserve existing database values if no source is available

## Monitoring and Metrics

### Enhanced Metrics

Add metrics for VMI data processing:

```go
var (
    vmiDataUpdates = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ssvirt_vmi_data_updates_total",
            Help: "Total number of VM data updates from VMI",
        },
        []string{"namespace", "field", "result"},
    )
    
    vmDataAccuracy = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ssvirt_vm_data_populated_ratio",
            Help: "Ratio of VMs with populated data fields",
        },
        []string{"field"},
    )
)
```

## Error Handling

### VMI-Specific Error Cases

```go
// Enhanced error handling for VMI operations
func (r *VMStatusController) handleVMIError(ctx context.Context, vm *kubevirtv1.VirtualMachine, err error) (ctrl.Result, error) {
    if errors.IsNotFound(err) {
        // VMI not found is normal for stopped VMs - use VM spec instead
        return r.handleVMSpecData(ctx, vm, vmRecord)
    }
    
    if errors.IsForbidden(err) {
        // Permission issue - log and skip
        r.Log.Error("Permission denied accessing VMI", "vm", vm.Name, "namespace", vm.Namespace, "error", err)
        return ctrl.Result{RequeueAfter: time.Hour}, nil
    }
    
    if errors.IsTimeout(err) || errors.IsServerTimeout(err) {
        // Temporary issue - retry sooner
        return ctrl.Result{RequeueAfter: 30 * time.Second}, err
    }
    
    // Other errors
    return ctrl.Result{RequeueAfter: time.Minute}, err
}
```

## Testing Strategy

### Unit Tests

```go
func TestVMIDataExtraction(t *testing.T) {
    tests := []struct {
        name     string
        vmi      *kubevirtv1.VirtualMachineInstance
        expected VMIData
    }{
        {
            name: "Complete VMI with all data",
            vmi: &kubevirtv1.VirtualMachineInstance{
                Status: kubevirtv1.VirtualMachineInstanceStatus{
                    CurrentCPUTopology: &kubevirtv1.CPUTopology{
                        Cores:   2,
                        Sockets: 1,
                        Threads: 1,
                    },
                    Memory: &kubevirtv1.MemoryStatus{
                        GuestCurrent: resource.NewQuantity(4*1024*1024*1024, resource.BinarySI), // 4GB
                    },
                    GuestOSInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
                        PrettyName: "CentOS Stream 9",
                    },
                },
            },
            expected: VMIData{
                CPUCount: ptr.Int(2),
                MemoryMB: ptr.Int(4096),
                GuestOS:  "CentOS Stream 9",
            },
        },
        // Additional test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := extractVMIData(tt.vmi)
            assert.Equal(t, tt.expected.CPUCount, result.CPUCount)
            assert.Equal(t, tt.expected.MemoryMB, result.MemoryMB)
            assert.Equal(t, tt.expected.GuestOS, result.GuestOS)
        })
    }
}
```

## Implementation Plan

### Phase 1: Core VMI Watching (Week 1)
1. **Extend Controller**: Add VirtualMachineInstance watching to existing controller
2. **VMI to VM Mapping**: Implement mapVMIToVM function for event routing
3. **Basic Data Extraction**: Implement extractVMIData for CPU, memory, guest OS
4. **Repository Method**: Add UpdateVMData method to VMRepository
5. **Unit Tests**: Test data extraction and change detection logic

### Phase 2: Integration and Reconciliation (Week 2)
1. **Enhanced Reconciliation**: Integrate VMI data into existing reconcile loop
2. **VM Spec Fallback**: Implement extractVMSpecData for non-running VMs
3. **Change Detection**: Implement smart update detection to minimize database writes
4. **Error Handling**: Add VMI-specific error handling and recovery
5. **Integration Testing**: End-to-end testing of VMI to database synchronization

### Phase 3: RBAC and Performance (Week 3)
1. **RBAC Updates**: Add VirtualMachineInstance permissions to controller role
2. **Performance Optimization**: Implement selective updates and change detection
3. **Metrics Addition**: Add VMI-specific Prometheus metrics
4. **Load Testing**: Validate performance with high VM counts
5. **Documentation**: Update API documentation with populated field examples

## Dependencies

### Required Packages

No additional Go dependencies required - uses existing KubeVirt and controller-runtime packages.

### OpenShift Version Requirements

- **Required**: OpenShift 4.19+ with OpenShift Virtualization enabled
- **KubeVirt**: v1.6.0+ for stable VirtualMachineInstance API
- **Guest Agent**: Required for guest OS information (optional but recommended)

## Success Criteria

### Functional Requirements

1. **Accurate CPU Data**: `cpu_count` field reflects actual running CPU configuration
2. **Accurate Memory Data**: `memory_mb` field reflects actual memory allocation  
3. **Guest OS Detection**: `guest_os` field populated with detected OS information
4. **Real-time Updates**: Changes reflect in database within 30 seconds
5. **Graceful Fallback**: Proper handling when VMI doesn't exist (use VM spec)

### Quality Requirements

1. **Data Accuracy**: 99%+ accuracy in CPU/memory values vs. actual VMI
2. **Performance**: VMI processing adds <50ms to reconciliation time
3. **Reliability**: 99.9% successful field population rate
4. **Error Recovery**: Automatic recovery from transient VMI access failures
5. **Resource Usage**: Controller memory usage increases <10%

## Conclusion

This enhancement populates the existing VM database fields with accurate runtime information from VirtualMachineInstance resources. The implementation leverages the existing controller architecture and database schema while providing significantly more accurate VM data.

### Key Benefits

1. **No Schema Changes**: Uses existing `cpu_count`, `memory_mb`, and `guest_os` fields
2. **Accurate Runtime Data**: Reflects actual running VM specifications instead of templates
3. **Intelligent Fallback**: Uses VM spec data when VMI doesn't exist (VM stopped)
4. **Minimal Infrastructure Impact**: Extends existing controller without architectural changes
5. **Enhanced API Value**: Makes existing VM API responses more accurate and useful

The solution provides accurate VM information using the current database structure while maintaining the simplicity and reliability of the existing VM Status Controller design.