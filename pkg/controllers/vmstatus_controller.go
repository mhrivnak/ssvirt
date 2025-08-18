package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"gorm.io/gorm"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VMRepositoryInterface defines the interface for VM repository operations
type VMRepositoryInterface interface {
	GetByNamespaceAndVMName(ctx context.Context, namespace, vmName string) (*models.VM, error)
	GetByVAppAndVMName(ctx context.Context, vappID, vmName string) (*models.VM, error)
	UpdateStatus(ctx context.Context, vmID string, status string) error
	UpdateVMData(ctx context.Context, vmID string, cpuCount *int, memoryMB *int, guestOS string) error
	CreateVM(ctx context.Context, vm *models.VM) error
}

// VAppRepositoryInterface defines the interface for VApp repository operations
type VAppRepositoryInterface interface {
	GetByNameInVDC(ctx context.Context, vdcID, name string) (*models.VApp, error)
	CreateVApp(ctx context.Context, vapp *models.VApp) error
}

// VDCRepositoryInterface defines the interface for VDC repository operations
type VDCRepositoryInterface interface {
	GetByNamespace(ctx context.Context, namespaceName string) (*models.VDC, error)
}

// VMStatusController reconciles VirtualMachine resources with database VM records
type VMStatusController struct {
	client.Client
	Scheme   *runtime.Scheme
	VMRepo   VMRepositoryInterface
	VAppRepo VAppRepositoryInterface
	VDCRepo  VDCRepositoryInterface
	Recorder record.EventRecorder
}

// VMInfo contains extracted information from VirtualMachine resource
type VMInfo struct {
	Name      string
	Namespace string
	Status    string
	VAppID    string
	VDCID     string
	UpdatedAt time.Time
}

// VMIData represents the data we extract from VirtualMachineInstance
type VMIData struct {
	CPUCount *int   // From status.currentCPUTopology.cores
	MemoryMB *int   // From status.memory.guestCurrent (converted to MB)
	GuestOS  string // From status.guestOSInfo (formatted string)
}

// SetupVMStatusController sets up the controller with the Manager
func SetupVMStatusController(mgr ctrl.Manager, vmRepo VMRepositoryInterface, vappRepo VAppRepositoryInterface, vdcRepo VDCRepositoryInterface) error {
	controller := &VMStatusController{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		VMRepo:   vmRepo,
		VAppRepo: vappRepo,
		VDCRepo:  vdcRepo,
		Recorder: mgr.GetEventRecorderFor("vm-status-controller"),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kubevirtv1.VirtualMachine{}).
		Watches(&kubevirtv1.VirtualMachineInstance{},
			handler.EnqueueRequestsFromMapFunc(controller.mapVMIToVM)).
		Complete(controller)
}

// Reconcile handles VirtualMachine resource changes
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/status,verbs=get
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances/status,verbs=get
//+kubebuilder:rbac:groups=template.openshift.io,resources=templateinstances,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *VMStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("virtualmachine", req.NamespacedName)

	// Fetch the VirtualMachine resource
	vm := &kubevirtv1.VirtualMachine{}
	err := r.Get(ctx, req.NamespacedName, vm)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Handle VM deletion
			logger.Info("VirtualMachine not found, handling deletion")
			return r.handleVMDeletion(ctx, req.NamespacedName)
		}
		logger.Error(err, "Failed to get VirtualMachine")
		return ctrl.Result{}, err
	}

	// Handle vapp.ssvirt label management first
	updated, err := r.ensureVAppLabel(ctx, vm)
	if err != nil {
		logger.Error(err, "Failed to ensure vapp.ssvirt label")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// If VM was updated, use the updated version for status processing
	if updated != nil {
		vm = updated
	}

	// Handle VM status update
	statusResult, err := r.handleVMStatusUpdate(ctx, vm)
	if err != nil {
		return statusResult, err
	}

	// Handle VMI data update
	vmiResult, err := r.handleVMIDataUpdate(ctx, vm)
	if err != nil {
		return vmiResult, err
	}

	// Return the more restrictive result
	if statusResult.RequeueAfter > 0 || vmiResult.RequeueAfter > 0 {
		requeue := statusResult.RequeueAfter
		if vmiResult.RequeueAfter > 0 && (requeue == 0 || vmiResult.RequeueAfter < requeue) {
			requeue = vmiResult.RequeueAfter
		}
		return ctrl.Result{RequeueAfter: requeue}, nil
	}

	return ctrl.Result{}, nil
}

// handleVMStatusUpdate processes VirtualMachine status changes
func (r *VMStatusController) handleVMStatusUpdate(ctx context.Context, vm *kubevirtv1.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace)
	startTime := time.Now()

	// Find or create corresponding database record
	vmRecord, err := r.findOrCreateVMRecord(ctx, vm)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// VM not managed by SSVirt, skip
			logger.V(1).Info("VirtualMachine not managed by SSVirt, skipping")
			recordVMSkipped(vm.Namespace, vm.Name, "not_managed")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to find or create VM record")
		recordVMReconcileError(vm.Namespace, vm.Name, "database_lookup_error")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Extract current status and info
	vmInfo := r.extractVMInfo(vm)
	oldStatus := vmRecord.Status

	// Check if update is needed
	if vmRecord.Status == vmInfo.Status &&
		vmRecord.UpdatedAt.After(vmInfo.UpdatedAt.Add(-time.Minute)) {
		// Status unchanged and recently updated, skip
		logger.V(1).Info("VM status unchanged, skipping update",
			"currentStatus", vmRecord.Status,
			"newStatus", vmInfo.Status)
		recordVMSkipped(vm.Namespace, vm.Name, "status_unchanged")
		return ctrl.Result{}, nil
	}

	// Update database record - only status and timestamp
	logger.Info("Updating VM status",
		"oldStatus", vmRecord.Status,
		"newStatus", vmInfo.Status,
		"vmID", vmRecord.ID)

	err = r.VMRepo.UpdateStatus(ctx, vmRecord.ID, vmInfo.Status)
	duration := time.Since(startTime).Seconds()

	if err != nil {
		logger.Error(err, "Failed to update VM status in database")
		recordVMStatusUpdate(vm.Namespace, vm.Name, oldStatus, vmInfo.Status, "error", duration)
		recordVMReconcileError(vm.Namespace, vm.Name, "database_update_error")
		r.Recorder.Event(vm, "Warning", "DatabaseUpdateFailed",
			fmt.Sprintf("Failed to update VM status in database: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Record successful update
	recordVMStatusUpdate(vm.Namespace, vm.Name, oldStatus, vmInfo.Status, "success", duration)

	logger.Info("Successfully updated VM status",
		"vmID", vmRecord.ID,
		"status", vmInfo.Status,
		"duration", fmt.Sprintf("%.3fs", duration))

	r.Recorder.Event(vm, "Normal", "StatusUpdated",
		fmt.Sprintf("VM status updated to %s", vmInfo.Status))

	return ctrl.Result{}, nil
}

// handleVMDeletion processes VirtualMachine deletion
func (r *VMStatusController) handleVMDeletion(ctx context.Context, namespacedName types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vm", namespacedName.Name, "namespace", namespacedName.Namespace)

	// Try to find VM record by namespace and VM name using existing fields
	namespace := namespacedName.Namespace
	vmName := namespacedName.Name

	// Find VM record directly using namespace and vm_name fields
	vmRecord, err := r.VMRepo.GetByNamespaceAndVMName(ctx, namespace, vmName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// VM not found in database, nothing to update
			logger.V(1).Info("VM not found in database, nothing to update")
			recordVMDeletion(namespace, vmName, "not_found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to find VM record for deletion")
		recordVMReconcileError(namespace, vmName, "deletion_lookup_error")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Update VM status to indicate deletion
	logger.Info("Updating VM status to DELETED", "vmID", vmRecord.ID)
	err = r.VMRepo.UpdateStatus(ctx, vmRecord.ID, "DELETED")
	if err != nil {
		logger.Error(err, "Failed to update VM status to DELETED")
		recordVMDeletion(namespace, vmName, "error")
		recordVMReconcileError(namespace, vmName, "deletion_update_error")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	recordVMDeletion(namespace, vmName, "success")
	logger.Info("Successfully updated VM status to DELETED", "vmID", vmRecord.ID)
	return ctrl.Result{}, nil
}

// handleVMIDataUpdate processes VirtualMachineInstance data for existing fields
func (r *VMStatusController) handleVMIDataUpdate(ctx context.Context, vm *kubevirtv1.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace)

	// Find corresponding database record
	vmRecord, err := r.findOrCreateVMRecord(ctx, vm)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// VM not managed by SSVirt, skip
			logger.V(1).Info("VirtualMachine not managed by SSVirt, skipping VMI data update")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to find VM record for VMI data update")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Try to find corresponding VMI
	vmi := &kubevirtv1.VirtualMachineInstance{}
	err = r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, vmi)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			// VMI doesn't exist - VM is not running, use VM spec defaults
			return r.handleVMSpecData(ctx, vm, vmRecord)
		}
		logger.Error(err, "Failed to get VirtualMachineInstance")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Extract data from VMI
	vmiData := extractVMIData(vmi)

	// Check if update is needed
	if !r.needsVMDataUpdate(vmRecord, vmiData) {
		logger.V(1).Info("VMI data unchanged, skipping update")
		return ctrl.Result{}, nil
	}

	// Update database record with VMI data
	err = r.VMRepo.UpdateVMData(ctx, vmRecord.ID, vmiData.CPUCount, vmiData.MemoryMB, vmiData.GuestOS)
	if err != nil {
		r.Recorder.Event(vm, "Warning", "VMDataUpdateFailed",
			fmt.Sprintf("Failed to update VM data: %v", err))
		logger.Error(err, "Failed to update VM data in database")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	logger.Info("Updated VM data from VMI",
		"vmID", vmRecord.ID,
		"cpuCount", vmiData.CPUCount,
		"memoryMB", vmiData.MemoryMB,
		"guestOS", vmiData.GuestOS)

	r.Recorder.Event(vm, "Normal", "VMDataUpdated",
		"VM data updated from VirtualMachineInstance")

	return ctrl.Result{}, nil
}

// handleVMSpecData extracts data from VirtualMachine spec when VMI doesn't exist
func (r *VMStatusController) handleVMSpecData(ctx context.Context, vm *kubevirtv1.VirtualMachine, vmRecord *models.VM) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace)

	// Extract data from VM spec
	specData := extractVMSpecData(vm)

	// Check if update is needed
	if !r.needsVMDataUpdate(vmRecord, specData) {
		logger.V(1).Info("VM spec data unchanged, skipping update")
		return ctrl.Result{}, nil
	}

	// Update database record with VM spec data
	err := r.VMRepo.UpdateVMData(ctx, vmRecord.ID, specData.CPUCount, specData.MemoryMB, specData.GuestOS)
	if err != nil {
		logger.Error(err, "Failed to update VM data from spec")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	logger.Info("Updated VM data from VM spec",
		"vmID", vmRecord.ID,
		"cpuCount", specData.CPUCount,
		"memoryMB", specData.MemoryMB,
		"guestOS", specData.GuestOS)

	return ctrl.Result{}, nil
}

// findOrCreateVMRecord locates or creates the database VM record for a VirtualMachine resource
func (r *VMStatusController) findOrCreateVMRecord(ctx context.Context, vm *kubevirtv1.VirtualMachine) (*models.VM, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace)

	// Strategy 1: Use labels to find vApp and VM
	vappID, hasVAppLabel := vm.Labels["vapp.ssvirt.io/vapp-id"]

	if hasVAppLabel {
		// Find VM by vApp ID and VM name (use VMName field for OpenShift name)
		vmRecord, err := r.VMRepo.GetByVAppAndVMName(ctx, vappID, vm.Name)
		if err == nil {
			return vmRecord, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// VM not found, but we have vApp ID - this shouldn't happen if vApp exists
		logger.V(1).Info("VM not found with vApp ID, falling back to namespace lookup", "vappID", vappID)
	}

	// Strategy 2: Search by namespace and VM name using existing fields
	vmRecord, err := r.VMRepo.GetByNamespaceAndVMName(ctx, vm.Namespace, vm.Name)
	if err == nil {
		return vmRecord, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Strategy 3: VM doesn't exist, check if we should create it
	// Only create if the VM has a vapp.ssvirt label (meaning it was created from a TemplateInstance)
	vappName, hasVAppName := vm.Labels["vapp.ssvirt"]
	if !hasVAppName || vappName == "" {
		// VM doesn't have vapp.ssvirt label, not managed by SSVirt
		return nil, gorm.ErrRecordNotFound
	}

	// Create the VM record
	logger.Info("Creating new VM record", "vappName", vappName)
	return r.createVMRecord(ctx, vm, vappName)
}

// createVMRecord creates a new VM record in the database
func (r *VMStatusController) createVMRecord(ctx context.Context, vm *kubevirtv1.VirtualMachine, vappName string) (*models.VM, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace, "vapp", vappName)

	// Find VDC by namespace
	vdc, err := r.VDCRepo.GetByNamespace(ctx, vm.Namespace)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.V(1).Info("VDC not found for namespace, VM not managed by SSVirt")
			return nil, gorm.ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to find VDC by namespace: %w", err)
	}

	// Find or create VApp
	vapp, err := r.findOrCreateVApp(ctx, vdc.ID, vappName)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create VApp: %w", err)
	}

	// Extract VM info
	vmInfo := r.extractVMInfo(vm)

	// Create VM record
	vmRecord := &models.VM{
		Name:      fmt.Sprintf("VM-%s", vm.Name), // VM display name
		VMName:    vm.Name,                       // OpenShift VM resource name
		Namespace: vm.Namespace,                  // OpenShift namespace
		VAppID:    vapp.ID,
		Status:    vmInfo.Status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = r.VMRepo.CreateVM(ctx, vmRecord)
	if err != nil {
		recordVMCreationOperation(vm.Namespace, vm.Name, vappName, "error")
		return nil, fmt.Errorf("failed to create VM record: %w", err)
	}

	recordVMCreationOperation(vm.Namespace, vm.Name, vappName, "success")
	logger.Info("Successfully created VM record", "vmID", vmRecord.ID, "vappID", vapp.ID)
	r.Recorder.Event(vm, "Normal", "VMRecordCreated",
		fmt.Sprintf("Created VM record %s in vApp %s", vmRecord.ID, vapp.Name))

	return vmRecord, nil
}

// findOrCreateVApp finds or creates a VApp record
func (r *VMStatusController) findOrCreateVApp(ctx context.Context, vdcID, vappName string) (*models.VApp, error) {
	logger := log.FromContext(ctx).WithValues("vdc", vdcID, "vapp", vappName)

	// Try to find existing VApp
	vapp, err := r.VAppRepo.GetByNameInVDC(ctx, vdcID, vappName)
	if err == nil {
		return vapp, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// VApp doesn't exist, create it
	logger.Info("Creating new VApp record")
	vapp = &models.VApp{
		Name:        vappName,
		VDCID:       vdcID,
		Status:      models.VAppStatusInstantiating, // Initial status for new vApps
		Description: fmt.Sprintf("VApp created from OpenShift TemplateInstance: %s", vappName),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = r.VAppRepo.CreateVApp(ctx, vapp)
	if err != nil {
		recordVAppCreationOperation("", vdcID, vappName, "error") // namespace not available in this context
		return nil, fmt.Errorf("failed to create VApp record: %w", err)
	}

	recordVAppCreationOperation("", vdcID, vappName, "success") // namespace not available in this context
	logger.Info("Successfully created VApp record", "vappID", vapp.ID)
	return vapp, nil
}

// extractVMInfo extracts status information from VirtualMachine resource
func (r *VMStatusController) extractVMInfo(vm *kubevirtv1.VirtualMachine) VMInfo {
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

// mapVMStatus converts KubeVirt VM status to SSVirt status
func mapVMStatus(vm *kubevirtv1.VirtualMachine) string {
	// Check if VM is deleted (has deletion timestamp)
	if !vm.DeletionTimestamp.IsZero() {
		return "DELETING"
	}

	// Map based on VM PrintableStatus
	switch vm.Status.PrintableStatus {
	case kubevirtv1.VirtualMachineStatusRunning:
		return "POWERED_ON"
	case kubevirtv1.VirtualMachineStatusStopped:
		return "POWERED_OFF"
	case kubevirtv1.VirtualMachineStatusStarting:
		return "POWERING_ON"
	case kubevirtv1.VirtualMachineStatusStopping:
		return "POWERING_OFF"
	case kubevirtv1.VirtualMachineStatusTerminating:
		return "POWERING_OFF"
	case kubevirtv1.VirtualMachineStatusProvisioning:
		return "STARTING"
	case kubevirtv1.VirtualMachineStatusPaused:
		return "SUSPENDED"
	case kubevirtv1.VirtualMachineStatusMigrating:
		return "POWERED_ON" // Still considered running during migration
	case kubevirtv1.VirtualMachineStatusUnknown:
		return "UNKNOWN"
	case kubevirtv1.VirtualMachineStatusCrashLoopBackOff:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusUnschedulable:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusErrImagePull:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusImagePullBackOff:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusPvcNotFound:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusDataVolumeError:
		return "ERROR"
	case kubevirtv1.VirtualMachineStatusWaitingForVolumeBinding:
		return "STARTING"
	case kubevirtv1.VirtualMachineStatusWaitingForReceiver:
		return "STARTING"
	default:
		// If PrintableStatus is empty, check spec to determine intended state
		if vm.Status.PrintableStatus == "" {
			if vm.Spec.Running != nil && *vm.Spec.Running {
				return "STARTING"
			}
			return "STOPPED"
		}
		return "UNKNOWN"
	}
}

// ensureVAppLabel ensures the vapp.ssvirt label is set correctly on the VirtualMachine
func (r *VMStatusController) ensureVAppLabel(ctx context.Context, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "namespace", vm.Namespace)

	// Check if vapp.ssvirt label already exists
	if vm.Labels != nil {
		if _, exists := vm.Labels["vapp.ssvirt"]; exists {
			// Label already exists, no need to update
			recordVMLabelOperation(vm.Namespace, vm.Name, "check", "exists")
			return nil, nil
		}
	}

	// Look for template instance owner label
	templateInstanceUID, hasTemplateLabel := vm.Labels["template.openshift.io/template-instance-owner"]
	if !hasTemplateLabel || templateInstanceUID == "" {
		// No template instance, skip label management
		logger.V(1).Info("No template instance owner label found, skipping vapp.ssvirt label")
		recordVMLabelOperation(vm.Namespace, vm.Name, "check", "no_template")
		return nil, nil
	}

	// Find the TemplateInstance by UID
	templateInstance, err := r.findTemplateInstanceByUID(ctx, templateInstanceUID)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.V(1).Info("TemplateInstance not found", "uid", templateInstanceUID)
			recordVMLabelOperation(vm.Namespace, vm.Name, "lookup", "not_found")
			return nil, nil
		}
		logger.Error(err, "Failed to find TemplateInstance", "uid", templateInstanceUID)
		recordVMLabelOperation(vm.Namespace, vm.Name, "lookup", "error")
		return nil, err
	}

	// Set the vapp.ssvirt label to the TemplateInstance name
	logger.Info("Setting vapp.ssvirt label", "templateInstance", templateInstance.Name)

	// Create a copy to modify
	vmCopy := vm.DeepCopy()
	if vmCopy.Labels == nil {
		vmCopy.Labels = make(map[string]string)
	}
	vmCopy.Labels["vapp.ssvirt"] = templateInstance.Name

	// Update the VirtualMachine
	err = r.Update(ctx, vmCopy)
	if err != nil {
		logger.Error(err, "Failed to update VirtualMachine with vapp.ssvirt label")
		recordVMReconcileError(vm.Namespace, vm.Name, "label_update_error")
		recordVMLabelOperation(vm.Namespace, vm.Name, "update", "error")
		return nil, err
	}

	logger.Info("Successfully set vapp.ssvirt label", "templateInstance", templateInstance.Name)
	recordVMLabelOperation(vm.Namespace, vm.Name, "update", "success")
	r.Recorder.Event(vmCopy, "Normal", "LabelUpdated",
		fmt.Sprintf("Set vapp.ssvirt label to %s", templateInstance.Name))

	return vmCopy, nil
}

// findTemplateInstanceByUID finds a TemplateInstance by its UID across all namespaces
func (r *VMStatusController) findTemplateInstanceByUID(ctx context.Context, uid string) (*templatev1.TemplateInstance, error) {
	// List all TemplateInstances across all namespaces
	templateInstanceList := &templatev1.TemplateInstanceList{}
	err := r.List(ctx, templateInstanceList)
	if err != nil {
		return nil, fmt.Errorf("failed to list TemplateInstances: %w", err)
	}

	// Find the one with matching UID
	for _, ti := range templateInstanceList.Items {
		if string(ti.UID) == uid {
			return &ti, nil
		}
	}

	return nil, k8serrors.NewNotFound(templatev1.Resource("templateinstance"), uid)
}

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

	// No owner reference found - skip processing
	return nil
}

// extractVMIData extracts relevant data from VirtualMachineInstance
func extractVMIData(vmi *kubevirtv1.VirtualMachineInstance) VMIData {
	data := VMIData{}

	// Extract CPU count from current topology
	if vmi.Status.CurrentCPUTopology != nil {
		totalCores := int(vmi.Status.CurrentCPUTopology.Cores *
			vmi.Status.CurrentCPUTopology.Sockets *
			vmi.Status.CurrentCPUTopology.Threads)
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
	if vmi.Status.GuestOSInfo.Name != "" || vmi.Status.GuestOSInfo.PrettyName != "" || vmi.Status.GuestOSInfo.ID != "" {
		guestOS := formatGuestOS(&vmi.Status.GuestOSInfo)
		data.GuestOS = guestOS
	}

	return data
}

// extractVMSpecData extracts data from VirtualMachine specification when VMI doesn't exist
func extractVMSpecData(vm *kubevirtv1.VirtualMachine) VMIData {
	data := VMIData{}

	// Extract CPU from VM spec
	if vm.Spec.Template != nil && vm.Spec.Template.Spec.Domain.CPU != nil {
		cores := vm.Spec.Template.Spec.Domain.CPU.Cores
		sockets := vm.Spec.Template.Spec.Domain.CPU.Sockets
		threads := vm.Spec.Template.Spec.Domain.CPU.Threads

		totalCores := int(cores * sockets * threads)
		data.CPUCount = &totalCores
	}

	// Extract memory from VM spec
	if vm.Spec.Template != nil && vm.Spec.Template.Spec.Domain.Memory != nil && vm.Spec.Template.Spec.Domain.Memory.Guest != nil {
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
