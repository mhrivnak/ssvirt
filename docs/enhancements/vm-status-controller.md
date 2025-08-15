# VM Status Controller Enhancement

## Overview

This enhancement proposes adding a Kubernetes controller, built with
controller-runtime, that watches OpenShift VirtualMachine resources and
automatically updates the status of corresponding VM records in PostgreSQL. This
provides real-time synchronization between the Kubernetes cluster state and the
SSVirt database, ensuring accurate VM status reporting through the API.

## Background

Currently, the SSVirt system creates VM records in PostgreSQL when instantiating
templates, but there is no mechanism to keep these database records synchronized
with the actual state of VirtualMachine resources in OpenShift. This leads to
several problems:

1. **Stale Status Information**: VM status in the database may not reflect the actual runtime state
2. **Manual Synchronization**: No automatic updates when VMs change state outside of SSVirt API calls
3. **Inconsistent Data**: Database and Kubernetes cluster can become out of sync
4. **Poor User Experience**: Users see outdated status information in API responses

## Goals

1. **Real-time Synchronization**: Automatically update database VM status when OpenShift VMs change state
2. **Bidirectional Monitoring**: Watch for both VM lifecycle events and status changes
3. **Resilient Operation**: Handle controller restarts, network issues, and cluster unavailability
4. **Minimal Performance Impact**: Efficient event processing without overloading the database
5. **Comprehensive Status Mapping**: Map all relevant OpenShift VM states to SSVirt status values
6. **Error Handling**: Gracefully handle orphaned resources and reconciliation failures

## Non-Goals

- Modifying VirtualMachine resources based on database changes (unidirectional sync only)
- Managing VM lifecycle operations (power on/off, deletion, etc.)
- Synchronizing VM specifications or configuration changes
- Real-time performance metrics collection
- Historical state tracking beyond current status

## Architecture Overview

### Deployment Architecture

The VM Status Controller runs as a **separate pod** alongside the SSVirt API Server, but shares the same container image. It operates as a **singleton** to prevent duplicate processing and ensure consistent state management.

```
┌─────────────────────────────────────────────────────────────────┐
│                        OpenShift Cluster                       │
│                                                                 │
│  ┌─────────────────────────────┐  ┌─────────────────────────────┐ │
│  │      SSVirt Namespace       │  │      VDC Namespaces        │ │
│  │                             │  │                             │ │
│  │  ┌─────────────────────────┐│  │  ┌─────────────────────────┐│ │
│  │  │   API Server Pod        ││  │  │   VirtualMachine        ││ │
│  │  │   (ssvirt-api-server)   ││  │  │   Resources             ││ │
│  │  │                         ││  │  │                         ││ │
│  │  │  - HTTP API Endpoints   ││  │  │  - VM instances         ││ │
│  │  │  - Database Connection  ││  │  │  - Status changes       ││ │
│  │  │  - Authentication       ││  │  │  - Lifecycle events     ││ │
│  │  └─────────────────────────┘│  │  └─────────────────────────┘│ │
│  │                             │  │             │               │ │
│  │  ┌─────────────────────────┐│  │             │               │ │
│  │  │  VM Controller Pod      ││  │             │               │ │
│  │  │  (ssvirt-vm-controller) ││◄─┼─────────────┘               │ │
│  │  │  **SINGLETON**          ││  │                             │ │
│  │  │                         ││  │                             │ │
│  │  │  - VM Resource Watcher  ││  │                             │ │
│  │  │  - Status Reconciler    ││  │                             │ │
│  │  │  - Database Updater     ││  │                             │ │
│  │  │  - Leader Election      ││  │                             │ │
│  │  └─────────────────────────┘│  │                             │ │
│  │             │               │  │                             │ │
│  └─────────────┼───────────────┘  └─────────────────────────────┘ │
│               │                                                 │
└───────────────┼─────────────────────────────────────────────────┘
                │
                ▼
      ┌─────────────────┐
      │   PostgreSQL    │
      │   Database      │
      │                 │
      │ ┌─────────────┐ │
      │ │ VM Records  │ │
      │ │ Status Data │ │
      │ └─────────────┘ │
      └─────────────────┘
```

### Binary and Container Structure

Both the API Server and VM Controller share the same container image but run as separate binaries:

```
Container Image: ssvirt:latest
├── /usr/local/bin/
│   ├── ssvirt-api-server     # Main API server binary
│   └── ssvirt-vm-controller  # VM status controller binary (NEW)
├── /etc/ssvirt/
│   └── config.yaml          # Shared configuration
└── /app/
    └── static/              # API documentation assets
```

### Event Flow

1. **VM Creation**: When VirtualMachine resources are created in VDC namespaces
2. **Status Changes**: VirtualMachine status updates (Running, Stopped, Starting, etc.)
3. **VM Deletion**: When VirtualMachine resources are deleted
4. **Controller Processing**: Singleton controller receives watch events and processes changes
5. **Database Update**: Corresponding VM records in PostgreSQL are updated
6. **Error Handling**: Failed updates are retried with exponential backoff
7. **Leader Election**: Ensures only one controller instance processes events

## Implementation Details

### 1. Controller Structure

```go
// VMStatusController reconciles VirtualMachine resources with database VM records
type VMStatusController struct {
    client.Client
    Scheme    *runtime.Scheme
    VMRepo    *repositories.VMRepository
    Recorder  record.EventRecorder
    Log       logr.Logger
}

// Reconcile handles VirtualMachine resource changes
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
    
    // Handle VM status update
    return r.handleVMStatusUpdate(ctx, vm)
}
```

### 2. VM Discovery and Mapping

The controller needs to identify which VirtualMachines correspond to SSVirt VM records:

```go
// findVMRecord locates the database VM record for a VirtualMachine resource
func (r *VMStatusController) findVMRecord(ctx context.Context, vm *kubevirtv1.VirtualMachine) (*models.VM, error) {
    // Strategy 1: Use labels to find vApp and VM
    vappID, hasVAppLabel := vm.Labels["vapp.ssvirt.io/vapp-id"]
    
    if hasVAppLabel {
        // Find VM by vApp ID and VM name (use VMName field for OpenShift name)
        return r.VMRepo.GetByVAppAndVMName(ctx, vappID, vm.Name)
    }
    
    // Strategy 2: Search by namespace and VM name using existing fields
    return r.VMRepo.GetByNamespaceAndVMName(ctx, vm.Namespace, vm.Name)
}
```

### 3. Status Mapping

Map OpenShift VirtualMachine status to SSVirt VM status:

```go
// mapVMStatus converts OpenShift VM phase to SSVirt status
func mapVMStatus(vm *kubevirtv1.VirtualMachine) string {
    // Check if VM is deleted (has deletion timestamp)
    if !vm.DeletionTimestamp.IsZero() {
        return "DELETING"
    }
    
    // Map based on VM phase and conditions
    switch {
    case vm.Status.Phase == kubevirtv1.VirtualMachinePhaseRunning:
        return "POWERED_ON"
    case vm.Status.Phase == kubevirtv1.VirtualMachinePhaseStopped:
        return "POWERED_OFF"
    case vm.Status.Phase == kubevirtv1.VirtualMachinePhaseStarting:
        return "POWERING_ON"
    case vm.Status.Phase == kubevirtv1.VirtualMachinePhaseStopping:
        return "POWERING_OFF"
    case vm.Status.Phase == kubevirtv1.VirtualMachinePhaseUnknown:
        return "UNKNOWN"
    case vm.Status.Phase == "":
        // Check if VM spec indicates it should be running
        if vm.Spec.Running != nil && *vm.Spec.Running {
            return "STARTING"
        }
        return "STOPPED"
    default:
        return "UNKNOWN"
    }
}

// extractVMInfo extracts status information from VirtualMachine resource
func extractVMInfo(vm *kubevirtv1.VirtualMachine) VMInfo {
    info := VMInfo{
        Name:      vm.Name,
        Namespace: vm.Namespace,
        Status:    mapVMStatus(vm),
        UpdatedAt: time.Now(),
    }
    
    // Extract labels for VM identification
    if vm.Labels != nil {
        info.VAppID = vm.Labels["vapp.ssvirt.io/vapp-id"]
        info.VDCID = vm.Labels["vdc.ssvirt.io/vdc-id"]
    }
    
    return info
}

type VMInfo struct {
    Name      string
    Namespace string
    Status    string
    VAppID    string
    VDCID     string
    UpdatedAt time.Time
}
```

### 4. Database Operations

```go
// handleVMStatusUpdate processes VirtualMachine status changes
func (r *VMStatusController) handleVMStatusUpdate(ctx context.Context, vm *kubevirtv1.VirtualMachine) (ctrl.Result, error) {
    // Find corresponding database record
    vmRecord, err := r.findVMRecord(ctx, vm)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // VM not managed by SSVirt, skip
            r.Log.V(1).Info("VirtualMachine not managed by SSVirt, skipping", 
                "vm", vm.Name, "namespace", vm.Namespace)
            return ctrl.Result{}, nil
        }
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    // Extract current status and info
    vmInfo := extractVMInfo(vm)
    
    // Check if update is needed
    if vmRecord.Status == vmInfo.Status && 
       vmRecord.UpdatedAt.After(vmInfo.UpdatedAt.Add(-time.Minute)) {
        // Status unchanged and recently updated, skip
        return ctrl.Result{}, nil
    }
    
    // Update database record - only status and timestamp
    vmRecord.Status = vmInfo.Status
    vmRecord.UpdatedAt = vmInfo.UpdatedAt
    
    err = r.VMRepo.UpdateStatus(ctx, vmRecord.ID, vmInfo.Status)
    if err != nil {
        r.Recorder.Event(vm, corev1.EventTypeWarning, "DatabaseUpdateFailed", 
            fmt.Sprintf("Failed to update VM status in database: %v", err))
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Updated VM status", 
        "vm", vm.Name, 
        "namespace", vm.Namespace, 
        "status", vmInfo.Status,
        "vmID", vmRecord.ID)
    
    r.Recorder.Event(vm, corev1.EventTypeNormal, "StatusUpdated", 
        fmt.Sprintf("VM status updated to %s", vmInfo.Status))
    
    return ctrl.Result{}, nil
}

// handleVMDeletion processes VirtualMachine deletion
func (r *VMStatusController) handleVMDeletion(ctx context.Context, namespacedName types.NamespacedName) (ctrl.Result, error) {
    // Try to find VM record by namespace and VM name using existing fields
    namespace := namespacedName.Namespace
    vmName := namespacedName.Name
    
    // Find VM record directly using namespace and vm_name fields
    vmRecord, err := r.VMRepo.GetByNamespaceAndVMName(ctx, namespace, vmName)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // VM not found in database, nothing to update
            return ctrl.Result{}, nil
        }
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    // Update VM status to indicate deletion
    err = r.VMRepo.UpdateStatus(ctx, vmRecord.ID, "DELETED")
    if err != nil {
        return ctrl.Result{RequeueAfter: time.Minute}, err
    }
    
    r.Log.Info("Updated VM status to DELETED", 
        "vm", vmName, 
        "namespace", namespace, 
        "vmID", vmRecord.ID)
    
    return ctrl.Result{}, nil
}
```

### 5. Controller Setup and Registration

```go
// SetupVMStatusController sets up the controller with the Manager
func SetupVMStatusController(mgr ctrl.Manager, vmRepo *repositories.VMRepository) error {
    controller := &VMStatusController{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        VMRepo:   vmRepo,
        Recorder: mgr.GetEventRecorderFor("vm-status-controller"),
        Log:      ctrl.Log.WithName("controllers").WithName("VMStatus"),
    }
    
    return ctrl.NewControllerManagedBy(mgr).
        For(&kubevirtv1.VirtualMachine{}).
        WithOptions(controller.Options{
            MaxConcurrentReconciles: 5,
            RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
                time.Second,    // base delay
                time.Minute*5,  // max delay
            ),
        }).
        Complete(controller)
}

// VM Controller Binary - NEW STANDALONE BINARY
func main() {
    // Parse command line flags
    var configPath string
    flag.StringVar(&configPath, "config", "/etc/ssvirt/config.yaml", "Path to configuration file")
    flag.Parse()
    
    // Load shared configuration
    config, err := loadConfig(configPath)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Setup database connection (shared with API server)
    db, err := setupDatabase(config.Database)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    
    // Create repositories
    vmRepo := repositories.NewVMRepository(db.DB)
    
    // Create controller manager with leader election enabled
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme:                 runtime.NewScheme(),
        MetricsBindAddress:     config.Controller.MetricsAddr,
        Port:                   0, // Disable webhook
        LeaderElection:         true, // ENABLE LEADER ELECTION FOR SINGLETON
        LeaderElectionID:       "ssvirt-vm-controller",
        LeaderElectionNamespace: config.Controller.Namespace,
        Namespace:              "", // Watch all namespaces
    })
    if err != nil {
        log.Fatalf("Failed to create controller manager: %v", err)
    }
    
    // Add required schemes
    if err := kubevirtv1.AddToScheme(mgr.GetScheme()); err != nil {
        log.Fatalf("Failed to add KubeVirt scheme: %v", err)
    }
    
    // Setup VM status controller
    if err = SetupVMStatusController(mgr, vmRepo); err != nil {
        log.Fatalf("Failed to setup VM status controller: %v", err)
    }
    
    log.Info("Starting VM Status Controller as singleton with leader election")
    
    // Start the manager (blocks until context is cancelled)
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        log.Fatalf("Controller manager failed: %v", err)
    }
}
```

## Database Schema Requirements

The controller uses existing database schema without modifications. It leverages the current VM model structure with these existing fields:

- `vm_name`: OpenShift VirtualMachine resource name
- `namespace`: OpenShift namespace where the VirtualMachine exists  
- `status`: Current status of the VM
- `updated_at`: Timestamp of last update
- `vapp_id`: Reference to parent vApp for VM grouping

## Repository Updates

```go
// Add new methods to VMRepository using existing schema fields
func (r *VMRepository) GetByNamespaceAndVMName(ctx context.Context, namespace, vmName string) (*models.VM, error) {
    var vm models.VM
    err := r.db.WithContext(ctx).
        Where("namespace = ? AND vm_name = ?", namespace, vmName).
        First(&vm).Error
    return &vm, err
}

func (r *VMRepository) GetByVAppAndVMName(ctx context.Context, vappID, vmName string) (*models.VM, error) {
    var vm models.VM
    err := r.db.WithContext(ctx).
        Where("vapp_id = ? AND vm_name = ?", vappID, vmName).
        First(&vm).Error
    return &vm, err
}

func (r *VMRepository) UpdateStatus(ctx context.Context, vmID string, status string) error {
    return r.db.WithContext(ctx).
        Model(&models.VM{}).
        Where("id = ?", vmID).
        Updates(map[string]interface{}{
            "status":     status,
            "updated_at": time.Now(),
        }).Error
}
```

## RBAC Requirements

The controller requires additional permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ssvirt-vm-status-controller
rules:
# Watch VirtualMachine resources across all namespaces
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines"]
  verbs: ["get", "list", "watch"]
# Read VM status and specs
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines/status"]
  verbs: ["get", "list"]
# Create events for tracking
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
# Access to all VDC namespaces (managed by SSVirt)
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```

## Configuration

Add controller configuration options:

```go
type ControllerConfig struct {
    Enabled                 bool          `yaml:"enabled" env:"CONTROLLER_ENABLED" envDefault:"true"`
    MaxConcurrentReconciles int           `yaml:"maxConcurrentReconciles" env:"CONTROLLER_MAX_CONCURRENT" envDefault:"5"`
    BaseDelay               time.Duration `yaml:"baseDelay" env:"CONTROLLER_BASE_DELAY" envDefault:"1s"`
    MaxDelay                time.Duration `yaml:"maxDelay" env:"CONTROLLER_MAX_DELAY" envDefault:"5m"`
    ResyncPeriod            time.Duration `yaml:"resyncPeriod" env:"CONTROLLER_RESYNC_PERIOD" envDefault:"10m"`
    EnableLeaderElection    bool          `yaml:"enableLeaderElection" env:"CONTROLLER_LEADER_ELECTION" envDefault:"true"`
    Namespace               string        `yaml:"namespace" env:"CONTROLLER_NAMESPACE" envDefault:"ssvirt-system"`
    MetricsAddr             string        `yaml:"metricsAddr" env:"CONTROLLER_METRICS_ADDR" envDefault:":8080"`
}
```

## Monitoring and Observability

### Metrics

Expose Prometheus metrics for monitoring:

```go
var (
    vmStatusUpdates = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ssvirt_vm_status_updates_total",
            Help: "Total number of VM status updates processed",
        },
        []string{"namespace", "status", "result"},
    )
    
    vmStatusUpdateDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "ssvirt_vm_status_update_duration_seconds",
            Help: "Time taken to update VM status in database",
        },
        []string{"namespace"},
    )
    
    vmReconcileErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ssvirt_vm_reconcile_errors_total",
            Help: "Total number of VM reconciliation errors",
        },
        []string{"namespace", "error_type"},
    )
)
```

### Logging

Structured logging for troubleshooting:

```go
// Use structured logging throughout controller
r.Log.Info("Processing VM status update",
    "vm", vm.Name,
    "namespace", vm.Namespace,
    "oldStatus", vmRecord.Status,
    "newStatus", vmInfo.Status,
    "vmID", vmRecord.ID)

r.Log.Error("Failed to update VM status",
    "vm", vm.Name,
    "namespace", vm.Namespace,
    "vmID", vmRecord.ID,
    "error", err)
```

### Health Checks

Add controller health endpoints:

```go
// Add health check for controller status
func (s *APIServer) setupControllerHealthCheck() {
    s.router.GET("/health/controller", func(c *gin.Context) {
        if s.controllerManager != nil {
            c.JSON(http.StatusOK, gin.H{
                "status": "healthy",
                "controller": "running",
            })
        } else {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "status": "unhealthy",
                "controller": "not_running",
            })
        }
    })
}
```

## Error Handling and Recovery

### Retry Strategy

```go
// Configure exponential backoff for failed reconciliations
func (r *VMStatusController) Options() controller.Options {
    return controller.Options{
        MaxConcurrentReconciles: 5,
        RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
            time.Second,    // base delay
            time.Minute*5,  // max delay
        ),
    }
}
```

### Dead Letter Handling

```go
// Track consistently failing VMs
func (r *VMStatusController) handleReconcileError(ctx context.Context, vm *kubevirtv1.VirtualMachine, err error) {
    // Get failure count from annotations
    failureCount := getFailureCount(vm)
    
    if failureCount > maxRetries {
        r.Log.Error("VM reconciliation failed too many times, adding to dead letter queue",
            "vm", vm.Name,
            "namespace", vm.Namespace,
            "failures", failureCount,
            "error", err)
        
        // Record persistent failure event
        r.Recorder.Event(vm, corev1.EventTypeWarning, "ReconciliationFailed",
            fmt.Sprintf("Failed to update VM status after %d retries: %v", failureCount, err))
        
        // Could implement dead letter queue or alerting here
        return
    }
    
    // Increment failure count
    updateFailureCount(vm, failureCount+1)
}
```

## Testing Strategy

### Unit Tests

```go
func TestVMStatusController_Reconcile(t *testing.T) {
    tests := []struct {
        name           string
        vm             *kubevirtv1.VirtualMachine
        existingVMDB   *models.VM
        expectedStatus string
        expectError    bool
    }{
        {
            name: "VM status changes from POWERED_OFF to POWERED_ON",
            vm: &kubevirtv1.VirtualMachine{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-vm",
                    Namespace: "test-vdc",
                    Labels: map[string]string{
                        "vapp.ssvirt.io/vapp-id": "test-vapp-id",
                    },
                },
                Status: kubevirtv1.VirtualMachineStatus{
                    Phase: kubevirtv1.VirtualMachinePhaseRunning,
                },
            },
            existingVMDB: &models.VM{
                ID:     "test-vm-id",
                Name:   "test-vm",
                Status: "POWERED_OFF",
                VAppID: "test-vapp-id",
            },
            expectedStatus: "POWERED_ON",
            expectError:    false,
        },
        // Add more test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup test environment
            // Run reconcile
            // Assert results
        })
    }
}
```

### Integration Tests

```go
func TestVMStatusController_Integration(t *testing.T) {
    // Create test cluster with VirtualMachine resources
    // Setup database with VM records
    // Start controller
    // Verify status synchronization
    // Test error scenarios
}
```

## Security Considerations

### Access Control

1. **Minimal Permissions**: Controller only has read access to VirtualMachine resources
2. **Namespace Isolation**: Respects VDC namespace boundaries
3. **Database Security**: Uses existing database authentication and connection pooling
4. **Event Recording**: Creates audit trail of status changes

### Resource Protection

1. **Rate Limiting**: Prevents controller from overwhelming database
2. **Resource Quotas**: Respects cluster resource limits
3. **Graceful Degradation**: Continues operating if some namespaces are inaccessible
4. **Read-Only Operations**: Never modifies VirtualMachine resources

## Performance Considerations

### Efficiency Optimizations

1. **Event-Driven**: Only processes actual changes, not periodic polling
2. **Batching**: Groups multiple status updates when possible
3. **Caching**: Uses controller-runtime caching for Kubernetes resources
4. **Connection Pooling**: Reuses database connections
5. **Selective Updates**: Only updates changed fields in database

### Scalability

1. **Concurrent Processing**: Configurable number of reconcile workers
2. **Namespace Partitioning**: Can be horizontally scaled by namespace
3. **Leader Election**: Supports high availability deployment
4. **Resource Limits**: Configurable memory and CPU limits

## Build and Deployment

### Build Process

The VM controller is built as a separate binary within the existing container build process:

```dockerfile
# Add to existing Containerfile
FROM registry.access.redhat.com/ubi8/ubi:latest as builder

# ... existing build steps for API server ...

# Build VM controller binary
RUN go build -o /usr/local/bin/ssvirt-vm-controller ./cmd/vm-controller

# Final image stage
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# Copy both binaries
COPY --from=builder /usr/local/bin/ssvirt-api-server /usr/local/bin/
COPY --from=builder /usr/local/bin/ssvirt-vm-controller /usr/local/bin/

# ... rest of existing container setup ...
```

### Deployment Structure

```yaml
# New VM Controller Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ssvirt-vm-controller
  namespace: ssvirt-system
spec:
  replicas: 1  # SINGLETON DEPLOYMENT
  selector:
    matchLabels:
      app: ssvirt-vm-controller
  template:
    metadata:
      labels:
        app: ssvirt-vm-controller
    spec:
      serviceAccountName: ssvirt-vm-controller
      containers:
      - name: controller
        image: ssvirt:latest  # SAME IMAGE AS API SERVER
        command:
        - /usr/local/bin/ssvirt-vm-controller  # DIFFERENT BINARY
        - --config=/etc/ssvirt/config.yaml
        env:
        - name: CONTROLLER_LEADER_ELECTION
          value: "true"
        - name: CONTROLLER_NAMESPACE
          value: "ssvirt-system"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        volumeMounts:
        - name: config
          mountPath: /etc/ssvirt
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: ssvirt-config
```

### Service Account and RBAC

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ssvirt-vm-controller
  namespace: ssvirt-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ssvirt-vm-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ssvirt-vm-controller
subjects:
- kind: ServiceAccount
  name: ssvirt-vm-controller
  namespace: ssvirt-system
```

## Implementation Plan

### Phase 1: Core Controller Implementation (Week 1-2)
1. **Create new binary**: Add `cmd/vm-controller/main.go` with standalone controller
2. **Implement VMStatusController**: Basic controller struct and reconcile logic
3. **Add VirtualMachine watching**: Event handling for VM resources
4. **Implement status mapping**: Between OpenShift and SSVirt status values
5. **Add database operations**: VM record lookup and status updates

### Phase 2: Singleton and Leader Election (Week 3)
1. **Leader election setup**: Configure controller-runtime leader election
2. **Deployment manifests**: Create singleton deployment configuration  
3. **Error handling and retry**: Comprehensive error handling with exponential backoff
4. **VM discovery strategies**: Implement VM-to-database record mapping
5. **RBAC configuration**: Set up required permissions for controller

### Phase 3: Build Integration and Testing (Week 4)
1. **Container build**: Integrate controller binary into existing image build
2. **Configuration sharing**: Use shared config between API server and controller
3. **Unit and integration tests**: Comprehensive test coverage
4. **Monitoring and metrics**: Prometheus metrics and structured logging
5. **Health checks**: Controller health and readiness endpoints

### Phase 4: Documentation and Production Readiness (Week 5)
1. **Deployment documentation**: Installation and configuration guides
2. **Operational runbooks**: Troubleshooting and maintenance procedures
3. **Performance testing**: Load testing and resource optimization
4. **Production validation**: End-to-end testing in staging environment
5. **Release preparation**: Final documentation and deployment guides

## Dependencies

### Required Packages

```go
// Add to go.mod
require (
    sigs.k8s.io/controller-runtime v0.18.4
    kubevirt.io/api v1.2.0
    k8s.io/apimachinery v0.30.3
    k8s.io/client-go v0.30.3
)
```

### OpenShift Version Requirements

- **Required**: OpenShift 4.19+ (for latest VirtualMachine API and controller-runtime features)
- **KubeVirt**: Compatible with KubeVirt 1.2+ (included in OpenShift Virtualization 4.19+)
- **Kubernetes**: 1.29+ (included with OpenShift 4.19+)

## Conclusion

The VM Status Controller enhancement provides essential real-time synchronization between OpenShift VirtualMachine resources and SSVirt database records. This ensures accurate status reporting through the API while maintaining system reliability and performance.

### Key Design Decisions

1. **Singleton Architecture**: The controller runs as a single pod with leader election to prevent duplicate processing and ensure consistent state management.

2. **Separate Binary, Shared Image**: Built as a standalone binary (`ssvirt-vm-controller`) within the existing container image, enabling independent deployment and scaling while maintaining operational simplicity.

3. **No Database Schema Changes**: Uses existing VM model fields (`vm_name`, `namespace`, `status`) to minimize implementation complexity and deployment risk.

4. **OpenShift 4.19+ Requirement**: Leverages latest VirtualMachine API stability and controller-runtime features for robust operation.

5. **Leader Election**: Ensures high availability and prevents split-brain scenarios when multiple controller replicas are deployed.

The controller-runtime framework provides a robust foundation for watching Kubernetes resources and handling reconciliation, while the singleton design with leader election ensures minimal performance impact and maximum reliability through proper error handling and retry mechanisms.

This enhancement is crucial for providing users with accurate, up-to-date information about their virtual machines and maintaining data consistency across the SSVirt system without requiring complex infrastructure changes.