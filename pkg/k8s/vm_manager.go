package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// VMManager manages VirtualMachine resources in OpenShift Virtualization
type VMManager struct {
	client     *Client
	translator *VMTranslator
	vmRepo     *repositories.VMRepository
}

// NewVMManager creates a new VMManager
func NewVMManager(client *Client, vmRepo *repositories.VMRepository) *VMManager {
	return &VMManager{
		client:     client,
		translator: NewVMTranslator(),
		vmRepo:     vmRepo,
	}
}

// CreateVM creates a new VirtualMachine in OpenShift and updates the database
func (vm *VMManager) CreateVM(ctx context.Context, dbVM *models.VM, diskSizeGB int) error {
	// Validate VM specification
	if err := vm.translator.ValidateVMSpec(dbVM); err != nil {
		return fmt.Errorf("invalid VM specification: %w", err)
	}

	// Convert to KubeVirt VirtualMachine
	kvVM, err := vm.translator.ToKubeVirtVM(dbVM)
	if err != nil {
		return fmt.Errorf("failed to convert VM to KubeVirt format: %w", err)
	}

	// Create VirtualMachine in OpenShift
	if err := vm.client.VMs(dbVM.Namespace).Create(ctx, kvVM); err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("VM %s already exists in namespace %s", dbVM.VMName, dbVM.Namespace)
		}
		return fmt.Errorf("failed to create VM in OpenShift: %w", err)
	}

	// Update VM status in database based on KubeVirt VM state
	dbVM.Status = vm.translator.VMStatusFromKubeVirt(kvVM)
	if err := vm.vmRepo.Update(dbVM); err != nil {
		// Try to clean up the created VM
		_ = vm.client.VMs(dbVM.Namespace).Delete(ctx, dbVM.VMName)
		return fmt.Errorf("failed to update VM in database: %w", err)
	}

	return nil
}

// GetVM retrieves a VM from OpenShift and updates database status
func (vm *VMManager) GetVM(ctx context.Context, vmName, namespace string) (*kubevirtv1.VirtualMachine, error) {
	kvVM, err := vm.client.VMs(namespace).Get(ctx, vmName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("VM %s not found in namespace %s", vmName, namespace)
		}
		return nil, fmt.Errorf("failed to get VM from OpenShift: %w", err)
	}

	// Update database status if we have the VM ID
	if vmID, exists := kvVM.Labels["ssvirt.io/vm-id"]; exists {
		if dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace); err == nil && dbVM.ID.String() == vmID {
			vm.translator.UpdateVMFromKubeVirt(dbVM, kvVM)
			if err := vm.vmRepo.Update(dbVM); err != nil {
				log.Printf("Warning: failed to update VM status in database for VM %s/%s: %v", namespace, vmName, err)
			}
		}
	}

	return kvVM, nil
}

// UpdateVM updates a VM in OpenShift
func (vm *VMManager) UpdateVM(ctx context.Context, dbVM *models.VM) error {
	// Validate VM specification
	if err := vm.translator.ValidateVMSpec(dbVM); err != nil {
		return fmt.Errorf("invalid VM specification: %w", err)
	}

	// Get current VM from OpenShift
	currentKvVM, err := vm.client.VMs(dbVM.Namespace).Get(ctx, dbVM.VMName)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("VM %s not found in namespace %s", dbVM.VMName, dbVM.Namespace)
		}
		return fmt.Errorf("failed to get current VM from OpenShift: %w", err)
	}

	// Convert updated VM to KubeVirt format
	newKvVM, err := vm.translator.ToKubeVirtVM(dbVM)
	if err != nil {
		return fmt.Errorf("failed to convert VM to KubeVirt format: %w", err)
	}

	// Preserve resource version and other metadata
	newKvVM.ResourceVersion = currentKvVM.ResourceVersion
	newKvVM.UID = currentKvVM.UID
	newKvVM.CreationTimestamp = currentKvVM.CreationTimestamp

	// Update VM in OpenShift
	if err := vm.client.VMs(dbVM.Namespace).Update(ctx, newKvVM); err != nil {
		return fmt.Errorf("failed to update VM in OpenShift: %w", err)
	}

	// Update database
	if err := vm.vmRepo.Update(dbVM); err != nil {
		return fmt.Errorf("failed to update VM in database: %w", err)
	}

	return nil
}

// DeleteVM deletes a VM from OpenShift and database
func (vm *VMManager) DeleteVM(ctx context.Context, vmName, namespace string) error {
	// Delete from OpenShift
	if err := vm.client.VMs(namespace).Delete(ctx, vmName); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete VM from OpenShift: %w", err)
		}
		// VM doesn't exist in OpenShift, continue with database cleanup
	}

	// Delete from database
	if dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace); err == nil {
		if err := vm.vmRepo.Delete(dbVM.ID); err != nil {
			return fmt.Errorf("failed to delete VM from database: %w", err)
		}
	}

	return nil
}

// PowerOnVM powers on a VM
func (vm *VMManager) PowerOnVM(ctx context.Context, vmName, namespace string) error {
	return vm.setPowerState(ctx, vmName, namespace, true)
}

// PowerOffVM powers off a VM
func (vm *VMManager) PowerOffVM(ctx context.Context, vmName, namespace string) error {
	return vm.setPowerState(ctx, vmName, namespace, false)
}

// SuspendVM suspends a VM (simulated by powering off and setting annotation)
func (vm *VMManager) SuspendVM(ctx context.Context, vmName, namespace string) error {
	kvVM, err := vm.client.VMs(namespace).Get(ctx, vmName)
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Set suspend annotation
	if kvVM.Annotations == nil {
		kvVM.Annotations = make(map[string]string)
	}
	kvVM.Annotations["ssvirt.io/suspend-status"] = "suspended"
	kvVM.Annotations["ssvirt.io/suspend-time"] = time.Now().Format(time.RFC3339)

	// Power off the VM
	running := false
	kvVM.Spec.Running = &running

	if err := vm.client.VMs(namespace).Update(ctx, kvVM); err != nil {
		return fmt.Errorf("failed to suspend VM: %w", err)
	}

	// Update database status
	if dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace); err == nil {
		dbVM.Status = "SUSPENDED"
		if err := vm.vmRepo.Update(dbVM); err != nil {
			log.Printf("Warning: failed to update VM status to SUSPENDED in database for VM %s/%s: %v", namespace, vmName, err)
		}
	}

	return nil
}

// ResetVM resets a VM (restart if running)
func (vm *VMManager) ResetVM(ctx context.Context, vmName, namespace string) error {
	kvVM, err := vm.client.VMs(namespace).Get(ctx, vmName)
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Only reset if VM is running
	if kvVM.Spec.Running == nil || !*kvVM.Spec.Running {
		return fmt.Errorf("VM must be powered on to reset")
	}

	// Add restart annotation to trigger reset
	if kvVM.Annotations == nil {
		kvVM.Annotations = make(map[string]string)
	}
	kvVM.Annotations["ssvirt.io/restart-request"] = time.Now().Format(time.RFC3339)

	if err := vm.client.VMs(namespace).Update(ctx, kvVM); err != nil {
		return fmt.Errorf("failed to reset VM: %w", err)
	}

	return nil
}

// setPowerState sets the power state of a VM
func (vm *VMManager) setPowerState(ctx context.Context, vmName, namespace string, running bool) error {
	kvVM, err := vm.client.VMs(namespace).Get(ctx, vmName)
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Clear suspend annotation if powering on
	if running && kvVM.Annotations != nil {
		delete(kvVM.Annotations, "ssvirt.io/suspend-status")
		delete(kvVM.Annotations, "ssvirt.io/suspend-time")
	}

	kvVM.Spec.Running = &running

	if err := vm.client.VMs(namespace).Update(ctx, kvVM); err != nil {
		return fmt.Errorf("failed to set power state: %w", err)
	}

	// Update database status
	if dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace); err == nil {
		if running {
			dbVM.Status = "POWERED_ON"
		} else {
			dbVM.Status = "POWERED_OFF"
		}
		if err := vm.vmRepo.Update(dbVM); err != nil {
			log.Printf("Warning: failed to update VM power status in database for VM %s/%s: %v", namespace, vmName, err)
		}
	}

	return nil
}

// ListVMs lists VMs in a namespace
func (vm *VMManager) ListVMs(ctx context.Context, namespace string) (*kubevirtv1.VirtualMachineList, error) {
	return vm.client.VMs(namespace).List(ctx, client.MatchingLabels{"app": "ssvirt"})
}

// SyncVMStatus synchronizes VM status between OpenShift and database
func (vm *VMManager) SyncVMStatus(ctx context.Context, vmName, namespace string) error {
	// Get VM from OpenShift
	kvVM, err := vm.client.VMs(namespace).Get(ctx, vmName)
	if err != nil {
		if errors.IsNotFound(err) {
			// VM doesn't exist in OpenShift, mark as deleted in database
			if dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace); err == nil {
				dbVM.Status = "UNRESOLVED"
				return vm.vmRepo.Update(dbVM)
			}
			return nil
		}
		return fmt.Errorf("failed to get VM from OpenShift: %w", err)
	}

	// Get VM from database
	dbVM, err := vm.vmRepo.GetByVMName(vmName, namespace)
	if err != nil {
		return fmt.Errorf("failed to get VM from database: %w", err)
	}

	// Update database VM with OpenShift status
	vm.translator.UpdateVMFromKubeVirt(dbVM, kvVM)
	return vm.vmRepo.Update(dbVM)
}

// HealthCheck verifies that the VMManager can communicate with OpenShift
func (vm *VMManager) HealthCheck(ctx context.Context) error {
	return vm.client.Health(ctx)
}
