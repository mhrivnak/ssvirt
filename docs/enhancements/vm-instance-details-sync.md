# VM Instance Details Synchronization Enhancement

## Overview

This enhancement proposes extending the existing VM Status Controller to watch VirtualMachineInstance resources and populate detailed runtime information in the PostgreSQL VM records. This provides comprehensive VM details including hardware specifications, IP addresses, guest OS information, and resource utilization when VMs are running.

## Background

Currently, the VM Status Controller only synchronizes basic status information between OpenShift VirtualMachine resources and PostgreSQL VM records. However, when a VirtualMachine is running, there is a corresponding VirtualMachineInstance (VMI) resource that contains rich runtime details about the actual running VM instance.

### Current Limitations

1. **Limited VM Details**: Database only stores basic VM information (name, status, vApp association)
2. **Missing Runtime Data**: No visibility into actual running VM specifications (CPU, memory, IP addresses)
3. **No Guest OS Info**: No information about the operating system running inside the VM
4. **Resource Tracking Gaps**: Cannot track actual resource usage vs. requested resources
5. **Network Information Missing**: No IP addresses or network interface details
6. **Hardware Details Absent**: Missing actual CPU topology, memory allocation, and disk information

### Rich Data Available in VirtualMachineInstance

VirtualMachineInstance resources provide extensive runtime information including:

- **Hardware Configuration**: CPU cores/sockets/threads, memory allocation, disk mappings
- **Network Details**: IP addresses, MAC addresses, interface names, network connectivity
- **Guest OS Information**: Operating system type, version, kernel details from guest agent
- **Resource Status**: Actual vs. requested resources, QoS class, node placement
- **Runtime State**: Phase transitions, conditions, migration status
- **Volume Information**: Persistent volume claims, disk targets, filesystem details

## Goals

1. **Comprehensive VM Data**: Populate database with detailed runtime information from VirtualMachineInstance
2. **Real-time Updates**: Synchronize VMI changes to database in real-time using watch events
3. **Graceful Handling**: Handle VM states where VirtualMachineInstance may not exist
4. **Backward Compatibility**: Maintain existing VM record structure while adding new details
5. **Performance Efficiency**: Minimize database writes and optimize for frequently changing data
6. **API Enhancement**: Make detailed VM information available through existing VM API endpoints

## Non-Goals

- Modifying VirtualMachineInstance resources based on database changes
- Historical tracking of VM runtime metrics (use monitoring systems for that)
- Real-time performance monitoring (CPU usage, memory utilization over time)
- Managing VM lifecycle operations through VMI manipulation
- Replacing existing VM power management functionality

## Architecture Overview

### Enhanced Controller Design

The existing VM Status Controller will be extended to also watch VirtualMachineInstance resources and extract detailed information to populate additional fields in the VM database records.

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
│  │  │  - Detail Extractor     ││  │  │  ┌─────────────────────┐││ │
│  │  │  - Database Updater     ││  │  │  │ VirtualMachine-     │││ │
│  │  │  - Leader Election      ││  │  │  │ Instance Resources  │││ │
│  │  └─────────────────────────┘│  │  │  │                     │││ │
│  │             │               │  │  │  │ - Runtime details   │││ │
│  └─────────────┼───────────────┘  │  │  │ - Guest OS info     │││ │
│               │                  │  │  │ - Network details   │││ │
│               │                  │  │  │ - Resource usage    │││ │
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
      │ │ Enhanced    │ │
      │ │ VM Records  │ │
      │ │             │ │
      │ │ - Basic info│ │
      │ │ - Status    │ │
      │ │ - Runtime   │ │
      │ │   details   │ │
      │ │ - Network   │ │
      │ │   info      │ │
      │ │ - Guest OS  │ │
      │ │ - Resources │ │
      │ └─────────────┘ │
      └─────────────────┘
```

## Database Schema Enhancements

### Extended VM Model

Add new fields to the existing VM model to store VirtualMachineInstance details:

```go
type VM struct {
    // Existing fields
    ID               string    `gorm:"type:varchar(255);primaryKey" json:"id"`
    Name             string    `gorm:"type:varchar(255);not null" json:"name"`
    VMName           string    `gorm:"type:varchar(255)" json:"vmName"`
    Status           string    `gorm:"type:varchar(50)" json:"status"`
    Namespace        string    `gorm:"type:varchar(255)" json:"namespace"`
    VAppID           string    `gorm:"type:varchar(255)" json:"vappId"`
    Description      string    `gorm:"type:text" json:"description"`
    CreatedAt        time.Time `json:"createdAt"`
    UpdatedAt        time.Time `json:"updatedAt"`
    
    // NEW: Runtime details from VirtualMachineInstance
    RuntimeDetails   *VMRuntimeDetails `gorm:"embedded;embeddedPrefix:runtime_" json:"runtimeDetails,omitempty"`
}

type VMRuntimeDetails struct {
    // Hardware Configuration
    CPUCores            *int32  `gorm:"column:cpu_cores" json:"cpuCores,omitempty"`
    CPUSockets          *int32  `gorm:"column:cpu_sockets" json:"cpuSockets,omitempty"`
    CPUThreads          *int32  `gorm:"column:cpu_threads" json:"cpuThreads,omitempty"`
    CPUModel            *string `gorm:"column:cpu_model;type:varchar(255)" json:"cpuModel,omitempty"`
    
    // Memory Information  
    MemoryGuest         *string `gorm:"column:memory_guest;type:varchar(50)" json:"memoryGuest,omitempty"`
    MemoryCurrent       *string `gorm:"column:memory_current;type:varchar(50)" json:"memoryCurrent,omitempty"`
    MemoryRequested     *string `gorm:"column:memory_requested;type:varchar(50)" json:"memoryRequested,omitempty"`
    
    // Network Details
    PrimaryIPAddress    *string `gorm:"column:primary_ip;type:varchar(45)" json:"primaryIPAddress,omitempty"`
    NetworkInterfaces   *string `gorm:"column:network_interfaces;type:text" json:"networkInterfaces,omitempty"` // JSON array
    
    // Guest OS Information
    GuestOSID           *string `gorm:"column:guest_os_id;type:varchar(100)" json:"guestOSID,omitempty"`
    GuestOSName         *string `gorm:"column:guest_os_name;type:varchar(255)" json:"guestOSName,omitempty"`
    GuestOSVersion      *string `gorm:"column:guest_os_version;type:varchar(100)" json:"guestOSVersion,omitempty"`
    GuestOSKernel       *string `gorm:"column:guest_os_kernel;type:varchar(255)" json:"guestOSKernel,omitempty"`
    
    // Runtime Status
    Phase               *string `gorm:"column:phase;type:varchar(50)" json:"phase,omitempty"`
    NodeName            *string `gorm:"column:node_name;type:varchar(255)" json:"nodeName,omitempty"`
    QOSClass            *string `gorm:"column:qos_class;type:varchar(50)" json:"qosClass,omitempty"`
    
    // Machine Configuration
    MachineType         *string `gorm:"column:machine_type;type:varchar(255)" json:"machineType,omitempty"`
    Architecture        *string `gorm:"column:architecture;type:varchar(50)" json:"architecture,omitempty"`
    
    // Volume Information
    VolumeDetails       *string `gorm:"column:volume_details;type:text" json:"volumeDetails,omitempty"` // JSON array
    
    // Agent Status
    AgentConnected      *bool   `gorm:"column:agent_connected" json:"agentConnected,omitempty"`
    
    // Last VMI Update
    VMIUpdatedAt        *time.Time `gorm:"column:vmi_updated_at" json:"vmiUpdatedAt,omitempty"`
}
```

### Database Migration

```sql
-- Add new columns to VMs table for runtime details
ALTER TABLE vms ADD COLUMN runtime_cpu_cores INTEGER;
ALTER TABLE vms ADD COLUMN runtime_cpu_sockets INTEGER;
ALTER TABLE vms ADD COLUMN runtime_cpu_threads INTEGER;
ALTER TABLE vms ADD COLUMN runtime_cpu_model VARCHAR(255);

ALTER TABLE vms ADD COLUMN runtime_memory_guest VARCHAR(50);
ALTER TABLE vms ADD COLUMN runtime_memory_current VARCHAR(50);
ALTER TABLE vms ADD COLUMN runtime_memory_requested VARCHAR(50);

ALTER TABLE vms ADD COLUMN runtime_primary_ip VARCHAR(45);
ALTER TABLE vms ADD COLUMN runtime_network_interfaces TEXT;

ALTER TABLE vms ADD COLUMN runtime_guest_os_id VARCHAR(100);
ALTER TABLE vms ADD COLUMN runtime_guest_os_name VARCHAR(255);
ALTER TABLE vms ADD COLUMN runtime_guest_os_version VARCHAR(100);
ALTER TABLE vms ADD COLUMN runtime_guest_os_kernel VARCHAR(255);

ALTER TABLE vms ADD COLUMN runtime_phase VARCHAR(50);
ALTER TABLE vms ADD COLUMN runtime_node_name VARCHAR(255);
ALTER TABLE vms ADD COLUMN runtime_qos_class VARCHAR(50);

ALTER TABLE vms ADD COLUMN runtime_machine_type VARCHAR(255);
ALTER TABLE vms ADD COLUMN runtime_architecture VARCHAR(50);

ALTER TABLE vms ADD COLUMN runtime_volume_details TEXT;
ALTER TABLE vms ADD COLUMN runtime_agent_connected BOOLEAN;
ALTER TABLE vms ADD COLUMN runtime_vmi_updated_at TIMESTAMP;

-- Create indexes for commonly queried fields
CREATE INDEX idx_vms_runtime_primary_ip ON vms(runtime_primary_ip);
CREATE INDEX idx_vms_runtime_node_name ON vms(runtime_node_name);
CREATE INDEX idx_vms_runtime_guest_os_id ON vms(runtime_guest_os_id);
CREATE INDEX idx_vms_runtime_phase ON vms(runtime_phase);
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

Extend the reconcile method to handle both VM status and VMI details:

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
    
    // Handle VMI details update (new logic)
    detailsResult, err := r.handleVMIDetailsUpdate(ctx, vm, vmRecord)
    if err != nil {
        return detailsResult, err
    }
    
    // Return the more restrictive result
    if statusResult.RequeueAfter > 0 || detailsResult.RequeueAfter > 0 {
        requeue := statusResult.RequeueAfter
        if detailsResult.RequeueAfter > 0 && detailsResult.RequeueAfter < requeue {
            requeue = detailsResult.RequeueAfter
        }
        return ctrl.Result{RequeueAfter: requeue}, nil
    }
    
    return ctrl.Result{}, nil
}
```

### 4. VMI Details Extraction

Extract detailed information from VirtualMachineInstance:

```go
// handleVMIDetailsUpdate processes VirtualMachineInstance details
func (r *VMStatusController) handleVMIDetailsUpdate(ctx context.Context, vm *kubevirtv1.VirtualMachine, vmRecord *models.VM) (ctrl.Result, error) {
    // Try to find corresponding VMI
    vmi := &kubevirtv1.VirtualMachineInstance{}
    err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, vmi)
    
    if err != nil {
        if errors.IsNotFound(err) {
            // VMI doesn't exist - VM is not running
            return r.handleVMINotFound(ctx, vmRecord)
        }
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    // Extract details from VMI
    runtimeDetails := extractVMIDetails(vmi)
    
    // Check if update is needed
    if !r.needsRuntimeDetailsUpdate(vmRecord, runtimeDetails) {
        return ctrl.Result{}, nil
    }
    
    // Update database record with runtime details
    err = r.VMRepo.UpdateRuntimeDetails(ctx, vmRecord.ID, runtimeDetails)
    if err != nil {
        r.Recorder.Event(vm, corev1.EventTypeWarning, "VMIDetailsUpdateFailed", 
            fmt.Sprintf("Failed to update VM runtime details: %v", err))
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Updated VM runtime details", 
        "vm", vm.Name, 
        "namespace", vm.Namespace, 
        "vmID", vmRecord.ID,
        "primaryIP", getStringValue(runtimeDetails.PrimaryIPAddress),
        "guestOS", getStringValue(runtimeDetails.GuestOSName))
    
    return ctrl.Result{}, nil
}

// extractVMIDetails extracts runtime details from VirtualMachineInstance
func extractVMIDetails(vmi *kubevirtv1.VirtualMachineInstance) *models.VMRuntimeDetails {
    details := &models.VMRuntimeDetails{
        VMIUpdatedAt: &vmi.Status.PhaseTransitionTimestamps[len(vmi.Status.PhaseTransitionTimestamps)-1].PhaseTransitionTimestamp.Time,
    }
    
    // Extract CPU information
    if vmi.Status.CurrentCPUTopology != nil {
        details.CPUCores = &vmi.Status.CurrentCPUTopology.Cores
        details.CPUSockets = &vmi.Status.CurrentCPUTopology.Sockets
        details.CPUThreads = &vmi.Status.CurrentCPUTopology.Threads
    }
    if vmi.Spec.Domain.CPU != nil && vmi.Spec.Domain.CPU.Model != "" {
        details.CPUModel = &vmi.Spec.Domain.CPU.Model
    }
    
    // Extract memory information
    if vmi.Status.Memory != nil {
        if vmi.Status.Memory.GuestAtBoot != nil {
            guestAtBoot := vmi.Status.Memory.GuestAtBoot.String()
            details.MemoryGuest = &guestAtBoot
        }
        if vmi.Status.Memory.GuestCurrent != nil {
            guestCurrent := vmi.Status.Memory.GuestCurrent.String()
            details.MemoryCurrent = &guestCurrent
        }
        if vmi.Status.Memory.GuestRequested != nil {
            guestRequested := vmi.Status.Memory.GuestRequested.String()
            details.MemoryRequested = &guestRequested
        }
    }
    
    // Extract network information
    if len(vmi.Status.Interfaces) > 0 {
        // Set primary IP from first interface
        if len(vmi.Status.Interfaces[0].IPAddresses) > 0 {
            details.PrimaryIPAddress = &vmi.Status.Interfaces[0].IPAddresses[0]
        }
        
        // Serialize all network interfaces
        interfacesJSON, err := json.Marshal(vmi.Status.Interfaces)
        if err == nil {
            interfacesStr := string(interfacesJSON)
            details.NetworkInterfaces = &interfacesStr
        }
    }
    
    // Extract guest OS information
    if vmi.Status.GuestOSInfo != nil {
        details.GuestOSID = &vmi.Status.GuestOSInfo.ID
        details.GuestOSName = &vmi.Status.GuestOSInfo.Name
        details.GuestOSVersion = &vmi.Status.GuestOSInfo.Version
        if vmi.Status.GuestOSInfo.KernelRelease != "" {
            details.GuestOSKernel = &vmi.Status.GuestOSInfo.KernelRelease
        }
    }
    
    // Extract runtime status
    if vmi.Status.Phase != "" {
        phase := string(vmi.Status.Phase)
        details.Phase = &phase
    }
    if vmi.Status.NodeName != "" {
        details.NodeName = &vmi.Status.NodeName
    }
    if vmi.Status.QOSClass != nil {
        qosClass := string(*vmi.Status.QOSClass)
        details.QOSClass = &qosClass
    }
    
    // Extract machine information
    if vmi.Status.Machine != nil && vmi.Status.Machine.Type != "" {
        details.MachineType = &vmi.Status.Machine.Type
    }
    if vmi.Spec.Architecture != "" {
        details.Architecture = &vmi.Spec.Architecture
    }
    
    // Extract volume information
    if len(vmi.Status.VolumeStatus) > 0 {
        volumesJSON, err := json.Marshal(vmi.Status.VolumeStatus)
        if err == nil {
            volumesStr := string(volumesJSON)
            details.VolumeDetails = &volumesStr
        }
    }
    
    // Extract agent status
    agentConnected := isAgentConnected(vmi)
    details.AgentConnected = &agentConnected
    
    return details
}

// isAgentConnected checks if guest agent is connected
func isAgentConnected(vmi *kubevirtv1.VirtualMachineInstance) bool {
    for _, condition := range vmi.Status.Conditions {
        if condition.Type == kubevirtv1.VirtualMachineInstanceAgentConnected {
            return condition.Status == corev1.ConditionTrue
        }
    }
    return false
}
```

### 5. Handle VM Not Running

When VirtualMachineInstance doesn't exist, clear runtime details:

```go
// handleVMINotFound clears runtime details when VMI doesn't exist
func (r *VMStatusController) handleVMINotFound(ctx context.Context, vmRecord *models.VM) (ctrl.Result, error) {
    // Only clear if we previously had runtime details
    if vmRecord.RuntimeDetails == nil || vmRecord.RuntimeDetails.VMIUpdatedAt == nil {
        return ctrl.Result{}, nil
    }
    
    // Clear runtime details since VM is not running
    err := r.VMRepo.ClearRuntimeDetails(ctx, vmRecord.ID)
    if err != nil {
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Cleared VM runtime details - VM not running", 
        "vmID", vmRecord.ID)
    
    return ctrl.Result{}, nil
}
```

### 6. Repository Updates

Add methods to VMRepository for handling runtime details:

```go
// UpdateRuntimeDetails updates VM runtime details from VirtualMachineInstance
func (r *VMRepository) UpdateRuntimeDetails(ctx context.Context, vmID string, details *models.VMRuntimeDetails) error {
    updates := make(map[string]interface{})
    
    // CPU details
    if details.CPUCores != nil {
        updates["runtime_cpu_cores"] = *details.CPUCores
    }
    if details.CPUSockets != nil {
        updates["runtime_cpu_sockets"] = *details.CPUSockets
    }
    if details.CPUThreads != nil {
        updates["runtime_cpu_threads"] = *details.CPUThreads
    }
    if details.CPUModel != nil {
        updates["runtime_cpu_model"] = *details.CPUModel
    }
    
    // Memory details
    if details.MemoryGuest != nil {
        updates["runtime_memory_guest"] = *details.MemoryGuest
    }
    if details.MemoryCurrent != nil {
        updates["runtime_memory_current"] = *details.MemoryCurrent
    }
    if details.MemoryRequested != nil {
        updates["runtime_memory_requested"] = *details.MemoryRequested
    }
    
    // Network details
    if details.PrimaryIPAddress != nil {
        updates["runtime_primary_ip"] = *details.PrimaryIPAddress
    }
    if details.NetworkInterfaces != nil {
        updates["runtime_network_interfaces"] = *details.NetworkInterfaces
    }
    
    // Guest OS details
    if details.GuestOSID != nil {
        updates["runtime_guest_os_id"] = *details.GuestOSID
    }
    if details.GuestOSName != nil {
        updates["runtime_guest_os_name"] = *details.GuestOSName
    }
    if details.GuestOSVersion != nil {
        updates["runtime_guest_os_version"] = *details.GuestOSVersion
    }
    if details.GuestOSKernel != nil {
        updates["runtime_guest_os_kernel"] = *details.GuestOSKernel
    }
    
    // Runtime status
    if details.Phase != nil {
        updates["runtime_phase"] = *details.Phase
    }
    if details.NodeName != nil {
        updates["runtime_node_name"] = *details.NodeName
    }
    if details.QOSClass != nil {
        updates["runtime_qos_class"] = *details.QOSClass
    }
    
    // Machine details
    if details.MachineType != nil {
        updates["runtime_machine_type"] = *details.MachineType
    }
    if details.Architecture != nil {
        updates["runtime_architecture"] = *details.Architecture
    }
    
    // Volume and agent details
    if details.VolumeDetails != nil {
        updates["runtime_volume_details"] = *details.VolumeDetails
    }
    if details.AgentConnected != nil {
        updates["runtime_agent_connected"] = *details.AgentConnected
    }
    
    // Update timestamp
    updates["runtime_vmi_updated_at"] = time.Now()
    updates["updated_at"] = time.Now()
    
    return r.db.WithContext(ctx).
        Model(&models.VM{}).
        Where("id = ?", vmID).
        Updates(updates).Error
}

// ClearRuntimeDetails clears runtime details when VM is not running
func (r *VMRepository) ClearRuntimeDetails(ctx context.Context, vmID string) error {
    updates := map[string]interface{}{
        "runtime_cpu_cores":            nil,
        "runtime_cpu_sockets":          nil,
        "runtime_cpu_threads":          nil,
        "runtime_cpu_model":            nil,
        "runtime_memory_guest":         nil,
        "runtime_memory_current":       nil,
        "runtime_memory_requested":     nil,
        "runtime_primary_ip":           nil,
        "runtime_network_interfaces":   nil,
        "runtime_guest_os_id":          nil,
        "runtime_guest_os_name":        nil,
        "runtime_guest_os_version":     nil,
        "runtime_guest_os_kernel":      nil,
        "runtime_phase":                nil,
        "runtime_node_name":            nil,
        "runtime_qos_class":            nil,
        "runtime_machine_type":         nil,
        "runtime_architecture":         nil,
        "runtime_volume_details":       nil,
        "runtime_agent_connected":      nil,
        "runtime_vmi_updated_at":       nil,
        "updated_at":                   time.Now(),
    }
    
    return r.db.WithContext(ctx).
        Model(&models.VM{}).
        Where("id = ?", vmID).
        Updates(updates).Error
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

The existing VM API endpoints will automatically include the new runtime details:

```json
{
  "id": "urn:vcloud:vm:550e8400-e29b-41d4-a716-446655440000",
  "name": "web-server-01",
  "vmName": "centos-stream9-t2cuhsdg42mv5lf3",
  "status": "POWERED_ON",
  "namespace": "vdc-clowns-east1",
  "vappId": "urn:vcloud:vapp:550e8400-e29b-41d4-a716-446655440001",
  "description": "Web server VM",
  "createdAt": "2025-08-16T15:20:00Z",
  "updatedAt": "2025-08-16T15:25:30Z",
  "runtimeDetails": {
    "cpuCores": 1,
    "cpuSockets": 1,
    "cpuThreads": 1,
    "cpuModel": "host-model",
    "memoryGuest": "4Gi",
    "memoryCurrent": "4Gi",
    "memoryRequested": "4Gi",
    "primaryIPAddress": "10.128.1.131",
    "networkInterfaces": "[{\"interfaceName\":\"eth0\",\"ipAddress\":\"10.128.1.131\",\"mac\":\"02:b3:34:00:00:07\"}]",
    "guestOSID": "centos",
    "guestOSName": "CentOS Stream",
    "guestOSVersion": "9",
    "guestOSKernel": "5.14.0-604.el9.x86_64",
    "phase": "Running",
    "nodeName": "api.qe1.kni.eng.rdu2.dc.redhat.com",
    "qosClass": "Burstable",
    "machineType": "pc-q35-rhel9.6.0",
    "architecture": "amd64",
    "volumeDetails": "[{\"name\":\"rootdisk\",\"target\":\"vda\",\"size\":\"30Gi\"}]",
    "agentConnected": true,
    "vmiUpdatedAt": "2025-08-16T15:25:30Z"
  },
  "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:550e8400-e29b-41d4-a716-446655440000"
}
```

## Performance Considerations

### Optimizations

1. **Selective Updates**: Only update changed fields in database
2. **Batch Processing**: Group multiple field updates into single database transaction
3. **Change Detection**: Compare current values before performing updates
4. **JSON Serialization**: Cache serialized network/volume JSON to avoid repeated marshaling
5. **Index Usage**: Create database indexes on commonly queried runtime fields

### Efficiency Improvements

```go
// needsRuntimeDetailsUpdate checks if database update is actually needed
func (r *VMStatusController) needsRuntimeDetailsUpdate(vmRecord *models.VM, newDetails *models.VMRuntimeDetails) bool {
    if vmRecord.RuntimeDetails == nil {
        return true // First time setting runtime details
    }
    
    current := vmRecord.RuntimeDetails
    
    // Check significant changes that warrant database update
    if !equalStringPtr(current.PrimaryIPAddress, newDetails.PrimaryIPAddress) ||
       !equalStringPtr(current.GuestOSName, newDetails.GuestOSName) ||
       !equalStringPtr(current.Phase, newDetails.Phase) ||
       !equalStringPtr(current.NodeName, newDetails.NodeName) ||
       !equalInt32Ptr(current.CPUCores, newDetails.CPUCores) ||
       !equalStringPtr(current.MemoryCurrent, newDetails.MemoryCurrent) {
        return true
    }
    
    // Check if VMI was updated recently (within last 5 minutes)
    if current.VMIUpdatedAt != nil && newDetails.VMIUpdatedAt != nil {
        if newDetails.VMIUpdatedAt.Sub(*current.VMIUpdatedAt) > 5*time.Minute {
            return true
        }
    }
    
    return false
}
```

## Monitoring and Metrics

### Enhanced Metrics

Add new Prometheus metrics for VMI processing:

```go
var (
    vmiDetailsUpdates = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ssvirt_vmi_details_updates_total",
            Help: "Total number of VMI details updates processed",
        },
        []string{"namespace", "result"},
    )
    
    vmiDetailsProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ssvirt_vmi_details_processing_duration_seconds",
            Help: "Time taken to process VMI details",
        },
        []string{"namespace"},
    )
    
    vmInstancesWithDetails = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ssvirt_vm_instances_with_runtime_details",
            Help: "Number of VMs with populated runtime details",
        },
        []string{"namespace"},
    )
)
```

## Error Handling

### VMI-Specific Error Cases

```go
// Enhanced error handling for VMI operations
func (r *VMStatusController) handleVMIError(ctx context.Context, vm *kubevirtv1.VirtualMachine, err error) (ctrl.Result, error) {
    if errors.IsNotFound(err) {
        // VMI not found is normal for stopped VMs
        return r.handleVMINotFound(ctx, vm)
    }
    
    if errors.IsForbidden(err) {
        // Permission issue - log and skip
        r.Log.Error("Permission denied accessing VMI", "vm", vm.Name, "namespace", vm.Namespace, "error", err)
        return ctrl.Result{RequeueAfter: time.Hour}, nil // Retry much later
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
func TestVMIDetailsExtraction(t *testing.T) {
    tests := []struct {
        name     string
        vmi      *kubevirtv1.VirtualMachineInstance
        expected *models.VMRuntimeDetails
    }{
        {
            name: "Complete VMI with all details",
            vmi: &kubevirtv1.VirtualMachineInstance{
                Status: kubevirtv1.VirtualMachineInstanceStatus{
                    Phase: kubevirtv1.VirtualMachineInstanceRunning,
                    CurrentCPUTopology: &kubevirtv1.CPUTopology{
                        Cores:   1,
                        Sockets: 1,
                        Threads: 1,
                    },
                    Interfaces: []kubevirtv1.VirtualMachineInstanceNetworkInterface{
                        {
                            InterfaceName: "eth0",
                            IPAddresses:   []string{"10.128.1.131"},
                            MAC:           "02:b3:34:00:00:07",
                        },
                    },
                    GuestOSInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
                        ID:      "centos",
                        Name:    "CentOS Stream",
                        Version: "9",
                    },
                },
            },
            expected: &models.VMRuntimeDetails{
                CPUCores:         ptr.Int32(1),
                CPUSockets:       ptr.Int32(1), 
                CPUThreads:       ptr.Int32(1),
                PrimaryIPAddress: ptr.String("10.128.1.131"),
                GuestOSID:        ptr.String("centos"),
                GuestOSName:      ptr.String("CentOS Stream"),
                GuestOSVersion:   ptr.String("9"),
                Phase:            ptr.String("Running"),
            },
        },
        // Additional test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := extractVMIDetails(tt.vmi)
            assert.Equal(t, tt.expected.CPUCores, result.CPUCores)
            assert.Equal(t, tt.expected.PrimaryIPAddress, result.PrimaryIPAddress)
            // Additional assertions...
        })
    }
}
```

### Integration Tests

```go
func TestVMIDetailsSync_Integration(t *testing.T) {
    // Setup test environment with VM and VMI resources
    // Create controller with test database
    // Verify VMI details are synchronized to database
    // Test VMI deletion clears details
    // Test VMI updates trigger database updates
}
```

## Security Considerations

### Additional Permissions

1. **VMI Read Access**: Controller gains read-only access to VirtualMachineInstance resources
2. **Data Sanitization**: Network interfaces and volume details are JSON-serialized with validation
3. **Field Validation**: Runtime details undergo validation before database storage
4. **Sensitive Data**: No passwords or secrets are extracted from VMI resources

### Data Privacy

1. **Guest OS Info**: Only basic OS identification, no user data
2. **Network Details**: IP addresses and interface info, no traffic data
3. **Volume Information**: Metadata only, no filesystem contents
4. **Resource Limits**: Hardware specs and resource allocation, no usage metrics

## Implementation Plan

### Phase 1: Database Schema and Models (Week 1)
1. **Database Migration**: Add runtime details columns to VMs table
2. **Model Updates**: Extend VM model with RuntimeDetails embedded struct
3. **Repository Methods**: Implement UpdateRuntimeDetails and ClearRuntimeDetails
4. **Index Creation**: Add database indexes for commonly queried fields
5. **Migration Testing**: Validate schema changes don't break existing functionality

### Phase 2: VMI Watching and Extraction (Week 2)
1. **Controller Enhancement**: Add VirtualMachineInstance watching to existing controller
2. **VMI to VM Mapping**: Implement mapVMIToVM function for event routing
3. **Details Extraction**: Implement extractVMIDetails function
4. **Data Validation**: Add validation for extracted runtime details
5. **Unit Tests**: Comprehensive testing of extraction logic

### Phase 3: Integration and Reconciliation (Week 3)
1. **Enhanced Reconciliation**: Integrate VMI details into existing reconcile loop
2. **Change Detection**: Implement smart update detection to minimize database writes
3. **Error Handling**: Add VMI-specific error handling and recovery
4. **RBAC Updates**: Add VirtualMachineInstance permissions to controller role
5. **Integration Testing**: End-to-end testing of VMI to database synchronization

### Phase 4: Performance and Monitoring (Week 4)
1. **Performance Optimization**: Implement selective updates and change detection
2. **Metrics Addition**: Add VMI-specific Prometheus metrics
3. **Load Testing**: Validate performance with high VM counts
4. **Documentation**: Update API documentation with new runtime details fields
5. **Production Validation**: Staging environment testing and validation

### Phase 5: Production Deployment (Week 5)
1. **Deployment Preparation**: Final testing and validation
2. **Migration Planning**: Production database migration strategy
3. **Rollout Execution**: Phased production deployment
4. **Monitoring Setup**: Production monitoring and alerting
5. **Documentation Completion**: Final user and operator documentation

## Dependencies

### Required Packages

No additional Go dependencies required - uses existing KubeVirt and controller-runtime packages.

### OpenShift Version Requirements

- **Required**: OpenShift 4.19+ with OpenShift Virtualization enabled
- **KubeVirt**: v1.6.0+ for stable VirtualMachineInstance API
- **Guest Agent**: Required for guest OS information (optional enhancement)

## Success Criteria

### Functional Requirements

1. **Complete Data Sync**: All available VMI details are accurately synchronized to database
2. **Real-time Updates**: VMI changes reflect in database within 30 seconds
3. **Graceful Handling**: Proper handling when VMI doesn't exist (VM not running)
4. **Performance**: No significant impact on controller performance
5. **Backward Compatibility**: Existing API responses include new details without breaking changes

### Quality Requirements

1. **Reliability**: 99.9% successful synchronization rate
2. **Performance**: VMI processing adds <100ms to reconciliation time
3. **Data Accuracy**: Runtime details match actual VMI state 99%+ of time
4. **Error Recovery**: Automatic recovery from transient failures
5. **Resource Usage**: Controller memory usage increases <20%

## Conclusion

This enhancement significantly enriches the VM data available through SSVirt APIs by synchronizing detailed runtime information from VirtualMachineInstance resources. The implementation extends the existing VM Status Controller architecture while maintaining performance and reliability.

### Key Benefits

1. **Comprehensive VM Information**: Users get complete runtime details including hardware specs, network info, and guest OS data
2. **Real-time Synchronization**: Immediate updates when VM runtime characteristics change
3. **Minimal Infrastructure Impact**: Leverages existing controller architecture and database schema
4. **Enhanced API Value**: Makes SSVirt APIs significantly more useful for VM management and monitoring
5. **Production Ready**: Built on proven controller-runtime patterns with comprehensive error handling

The solution provides a robust foundation for rich VM detail synchronization while maintaining the simplicity and reliability of the existing VM Status Controller design.