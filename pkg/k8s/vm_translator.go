package k8s

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VMTranslator handles translation between database VM models and KubeVirt VirtualMachine resources
type VMTranslator struct{}

// NewVMTranslator creates a new VMTranslator
func NewVMTranslator() *VMTranslator {
	return &VMTranslator{}
}

// ToKubeVirtVM converts a database VM model to a KubeVirt VirtualMachine
func (vt *VMTranslator) ToKubeVirtVM(vm *models.VM) (*kubevirtv1.VirtualMachine, error) {
	if vm == nil {
		return nil, fmt.Errorf("vm cannot be nil")
	}

	// Create VirtualMachine resource
	kvVM := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vm.VMName,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"app":                    "ssvirt",
				"ssvirt.io/vm-id":        vm.ID.String(),
				"ssvirt.io/vapp-id":      vm.VAppID.String(),
				"ssvirt.io/managed-by":   "ssvirt-controller",
			},
			Annotations: map[string]string{
				"ssvirt.io/vm-name":          vm.Name,
				"ssvirt.io/vm-status":        vm.Status,
				"ssvirt.io/created-by":       "ssvirt",
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: vt.shouldVMBeRunning(vm.Status),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    "ssvirt",
						"ssvirt.io/vm-id":        vm.ID.String(),
						"ssvirt.io/vapp-id":      vm.VAppID.String(),
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Resources: kubevirtv1.ResourceRequirements{
							Requests: corev1.ResourceList{},
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: "rootdisk",
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
							Interfaces: []kubevirtv1.Interface{
								{
									Name: "default",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Masquerade: &kubevirtv1.InterfaceMasquerade{},
									},
								},
							},
						},
					},
					Networks: []kubevirtv1.Network{
						{
							Name: "default",
							NetworkSource: kubevirtv1.NetworkSource{
								Pod: &kubevirtv1.PodNetwork{},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "rootdisk",
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: vm.VMName + "-disk",
								},
							},
						},
					},
				},
			},
		},
	}

	// Set CPU count
	if vm.CPUCount != nil && *vm.CPUCount > 0 {
		kvVM.Spec.Template.Spec.Domain.CPU = &kubevirtv1.CPU{
			Cores: uint32(*vm.CPUCount),
		}
	} else {
		// Default to 1 CPU
		kvVM.Spec.Template.Spec.Domain.CPU = &kubevirtv1.CPU{
			Cores: 1,
		}
	}

	// Set memory
	if vm.MemoryMB != nil && *vm.MemoryMB > 0 {
		memoryQuantity := resource.NewQuantity(int64(*vm.MemoryMB)*1024*1024, resource.BinarySI)
		kvVM.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory] = *memoryQuantity
	} else {
		// Default to 1Gi memory
		memoryQuantity := resource.NewQuantity(1024*1024*1024, resource.BinarySI)
		kvVM.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory] = *memoryQuantity
	}

	return kvVM, nil
}

// shouldVMBeRunning determines if the VM should be running based on VCD status
func (vt *VMTranslator) shouldVMBeRunning(status string) *bool {
	running := false
	switch status {
	case "POWERED_ON":
		running = true
	case "POWERED_OFF", "SUSPENDED", "UNRESOLVED":
		running = false
	default:
		// Default to powered off for unknown states
		running = false
	}
	return &running
}

// VMStatusFromKubeVirt determines VCD VM status from KubeVirt VirtualMachine
func (vt *VMTranslator) VMStatusFromKubeVirt(kvVM *kubevirtv1.VirtualMachine) string {
	// Check if VM should be running
	if kvVM.Spec.Running != nil && *kvVM.Spec.Running {
		// Check if VMI exists and is running
		if kvVM.Status.Ready {
			return "POWERED_ON"
		}
		// VM should be running but isn't ready yet
		return "POWERED_ON" // Starting up
	}

	// Check for suspend annotation (KubeVirt doesn't have native suspend)
	if kvVM.Annotations != nil {
		if suspendStatus, exists := kvVM.Annotations["ssvirt.io/suspend-status"]; exists && suspendStatus == "suspended" {
			return "SUSPENDED"
		}
	}

	// Check if VM is being deleted
	if kvVM.DeletionTimestamp != nil {
		return "UNRESOLVED"
	}

	// Default to powered off
	return "POWERED_OFF"
}

// CreateDataVolumeSpec creates a DataVolume spec for VM disk storage
func (vt *VMTranslator) CreateDataVolumeSpec(vm *models.VM, diskSizeGB int) map[string]interface{} {
	if diskSizeGB <= 0 {
		diskSizeGB = 20 // Default 20GB
	}

	dataVolumeSpec := map[string]interface{}{
		"apiVersion": "cdi.kubevirt.io/v1beta1",
		"kind":       "DataVolume",
		"metadata": map[string]interface{}{
			"name":      vm.VMName + "-disk",
			"namespace": vm.Namespace,
			"labels": map[string]string{
				"app":                   "ssvirt",
				"ssvirt.io/vm-id":       vm.ID.String(),
				"ssvirt.io/vapp-id":     vm.VAppID.String(),
				"ssvirt.io/managed-by":  "ssvirt-controller",
			},
		},
		"spec": map[string]interface{}{
			"pvc": map[string]interface{}{
				"accessModes": []string{"ReadWriteOnce"},
				"resources": map[string]interface{}{
					"requests": map[string]string{
						"storage": fmt.Sprintf("%dGi", diskSizeGB),
					},
				},
				"storageClassName": "default", // Use default storage class
			},
			"source": map[string]interface{}{
				"blank": map[string]interface{}{}, // Create blank disk
			},
		},
	}

	return dataVolumeSpec
}

// UpdateVMFromKubeVirt updates a database VM model with status from KubeVirt
func (vt *VMTranslator) UpdateVMFromKubeVirt(vm *models.VM, kvVM *kubevirtv1.VirtualMachine) {
	if vm == nil || kvVM == nil {
		return
	}

	// Update status
	vm.Status = vt.VMStatusFromKubeVirt(kvVM)

	// Update CPU and memory if they changed in KubeVirt
	if kvVM.Spec.Template != nil && kvVM.Spec.Template.Spec.Domain.CPU != nil {
		cpuCount := int(kvVM.Spec.Template.Spec.Domain.CPU.Cores)
		vm.CPUCount = &cpuCount
	}

	if kvVM.Spec.Template != nil {
		if memoryQuantity, exists := kvVM.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory]; exists {
			memoryMB := int(memoryQuantity.Value() / (1024 * 1024))
			vm.MemoryMB = &memoryMB
		}
	}
}

// ValidateVMSpec validates that VM specifications are within acceptable limits
func (vt *VMTranslator) ValidateVMSpec(vm *models.VM) error {
	if vm == nil {
		return fmt.Errorf("vm cannot be nil")
	}

	if vm.VMName == "" {
		return fmt.Errorf("vm name cannot be empty")
	}

	if vm.Namespace == "" {
		return fmt.Errorf("vm namespace cannot be empty")
	}

	// Validate CPU count
	if vm.CPUCount != nil {
		if *vm.CPUCount < 1 {
			return fmt.Errorf("cpu count must be at least 1")
		}
		if *vm.CPUCount > 64 {
			return fmt.Errorf("cpu count cannot exceed 64")
		}
	}

	// Validate memory
	if vm.MemoryMB != nil {
		if *vm.MemoryMB < 128 {
			return fmt.Errorf("memory must be at least 128MB")
		}
		if *vm.MemoryMB > 1024*1024 {
			return fmt.Errorf("memory cannot exceed 1TB")
		}
	}

	return nil
}