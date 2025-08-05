package unit

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/k8s"
)

func TestVMTranslator_ToKubeVirtVM(t *testing.T) {
	translator := k8s.NewVMTranslator()

	t.Run("Convert basic VM to KubeVirt format", func(t *testing.T) {
		vmID := uuid.New()
		vappID := uuid.New()
		cpuCount := 2
		memoryMB := 4096

		vm := &models.VM{
			ID:        vmID,
			Name:      "test-vm",
			VAppID:    vappID,
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			Status:    "POWERED_OFF",
			CPUCount:  &cpuCount,
			MemoryMB:  &memoryMB,
		}

		kvVM, err := translator.ToKubeVirtVM(vm)
		require.NoError(t, err)
		require.NotNil(t, kvVM)

		// Check metadata
		assert.Equal(t, "test-vm-k8s", kvVM.Name)
		assert.Equal(t, "test-namespace", kvVM.Namespace)
		assert.Equal(t, vmID.String(), kvVM.Labels["ssvirt.io/vm-id"])
		assert.Equal(t, vappID.String(), kvVM.Labels["ssvirt.io/vapp-id"])
		assert.Equal(t, "test-vm", kvVM.Annotations["ssvirt.io/vm-name"])

		// Check spec
		assert.NotNil(t, kvVM.Spec.Running)
		assert.False(t, *kvVM.Spec.Running) // POWERED_OFF should map to false

		// Check CPU
		assert.NotNil(t, kvVM.Spec.Template.Spec.Domain.CPU)
		assert.Equal(t, uint32(2), kvVM.Spec.Template.Spec.Domain.CPU.Cores)

		// Check memory
		memoryQuantity := kvVM.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory]
		expectedMemory := resource.NewQuantity(int64(4096)*1024*1024, resource.BinarySI)
		assert.True(t, memoryQuantity.Equal(*expectedMemory))

		// Check default disk and network
		assert.Len(t, kvVM.Spec.Template.Spec.Domain.Devices.Disks, 1)
		assert.Equal(t, "rootdisk", kvVM.Spec.Template.Spec.Domain.Devices.Disks[0].Name)
		assert.Len(t, kvVM.Spec.Template.Spec.Networks, 1)
		assert.Equal(t, "default", kvVM.Spec.Template.Spec.Networks[0].Name)
	})

	t.Run("Convert VM with defaults when values are nil", func(t *testing.T) {
		vm := &models.VM{
			ID:        uuid.New(),
			Name:      "test-vm",
			VAppID:    uuid.New(),
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			Status:    "POWERED_ON",
			CPUCount:  nil,
			MemoryMB:  nil,
		}

		kvVM, err := translator.ToKubeVirtVM(vm)
		require.NoError(t, err)

		// Check defaults
		assert.True(t, *kvVM.Spec.Running)                                   // POWERED_ON should map to true
		assert.Equal(t, uint32(1), kvVM.Spec.Template.Spec.Domain.CPU.Cores) // Default CPU

		// Check default memory (1Gi)
		memoryQuantity := kvVM.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory]
		expectedMemory := resource.NewQuantity(1024*1024*1024, resource.BinarySI)
		assert.True(t, memoryQuantity.Equal(*expectedMemory))
	})

	t.Run("Error on nil VM", func(t *testing.T) {
		kvVM, err := translator.ToKubeVirtVM(nil)
		assert.Error(t, err)
		assert.Nil(t, kvVM)
		assert.Contains(t, err.Error(), "vm cannot be nil")
	})
}

func TestVMTranslator_VMStatusFromKubeVirt(t *testing.T) {
	translator := k8s.NewVMTranslator()

	t.Run("Running VM", func(t *testing.T) {
		running := true
		kvVM := &kubevirtv1.VirtualMachine{
			Spec: kubevirtv1.VirtualMachineSpec{
				Running: &running,
			},
			Status: kubevirtv1.VirtualMachineStatus{
				Ready: true,
			},
		}

		status := translator.VMStatusFromKubeVirt(kvVM)
		assert.Equal(t, "POWERED_ON", status)
	})

	t.Run("Stopped VM", func(t *testing.T) {
		running := false
		kvVM := &kubevirtv1.VirtualMachine{
			Spec: kubevirtv1.VirtualMachineSpec{
				Running: &running,
			},
		}

		status := translator.VMStatusFromKubeVirt(kvVM)
		assert.Equal(t, "POWERED_OFF", status)
	})

	t.Run("Suspended VM", func(t *testing.T) {
		running := false
		kvVM := &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"ssvirt.io/suspend-status": "suspended",
				},
			},
			Spec: kubevirtv1.VirtualMachineSpec{
				Running: &running,
			},
		}

		status := translator.VMStatusFromKubeVirt(kvVM)
		assert.Equal(t, "SUSPENDED", status)
	})

	t.Run("Deleting VM", func(t *testing.T) {
		now := metav1.Now()
		kvVM := &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &now,
			},
		}

		status := translator.VMStatusFromKubeVirt(kvVM)
		assert.Equal(t, "UNRESOLVED", status)
	})
}

func TestVMTranslator_ValidateVMSpec(t *testing.T) {
	translator := k8s.NewVMTranslator()

	t.Run("Valid VM spec", func(t *testing.T) {
		cpuCount := 2
		memoryMB := 4096
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			CPUCount:  &cpuCount,
			MemoryMB:  &memoryMB,
		}

		err := translator.ValidateVMSpec(vm)
		assert.NoError(t, err)
	})

	t.Run("Nil VM", func(t *testing.T) {
		err := translator.ValidateVMSpec(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vm cannot be nil")
	})

	t.Run("Empty VM name", func(t *testing.T) {
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "",
			Namespace: "test-namespace",
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vm name cannot be empty")
	})

	t.Run("Empty namespace", func(t *testing.T) {
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "",
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vm namespace cannot be empty")
	})

	t.Run("Invalid CPU count - too low", func(t *testing.T) {
		cpuCount := 0
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			CPUCount:  &cpuCount,
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cpu count must be at least 1")
	})

	t.Run("Invalid CPU count - too high", func(t *testing.T) {
		cpuCount := 128
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			CPUCount:  &cpuCount,
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cpu count cannot exceed 64")
	})

	t.Run("Invalid memory - too low", func(t *testing.T) {
		memoryMB := 64
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			MemoryMB:  &memoryMB,
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory must be at least 128MB")
	})

	t.Run("Invalid memory - too high", func(t *testing.T) {
		memoryMB := 2 * 1024 * 1024 // 2TB
		vm := &models.VM{
			Name:      "test-vm",
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			MemoryMB:  &memoryMB,
		}

		err := translator.ValidateVMSpec(vm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory cannot exceed 1TB")
	})
}

func TestVMTranslator_CreateDataVolumeSpec(t *testing.T) {
	translator := k8s.NewVMTranslator()

	vm := &models.VM{
		ID:        uuid.New(),
		VAppID:    uuid.New(),
		VMName:    "test-vm-k8s",
		Namespace: "test-namespace",
	}

	t.Run("Create DataVolume spec with custom size", func(t *testing.T) {
		spec := translator.CreateDataVolumeSpec(vm, 50)

		assert.Equal(t, "cdi.kubevirt.io/v1beta1", spec["apiVersion"])
		assert.Equal(t, "DataVolume", spec["kind"])

		metadata := spec["metadata"].(map[string]interface{})
		assert.Equal(t, "test-vm-k8s-disk", metadata["name"])
		assert.Equal(t, "test-namespace", metadata["namespace"])

		labels := metadata["labels"].(map[string]string)
		assert.Equal(t, "ssvirt", labels["app"])
		assert.Equal(t, vm.ID.String(), labels["ssvirt.io/vm-id"])

		specMap := spec["spec"].(map[string]interface{})
		pvc := specMap["pvc"].(map[string]interface{})
		resources := pvc["resources"].(map[string]interface{})
		requests := resources["requests"].(map[string]string)
		assert.Equal(t, "50Gi", requests["storage"])
	})

	t.Run("Create DataVolume spec with default size", func(t *testing.T) {
		spec := translator.CreateDataVolumeSpec(vm, 0)

		specMap := spec["spec"].(map[string]interface{})
		pvc := specMap["pvc"].(map[string]interface{})
		resources := pvc["resources"].(map[string]interface{})
		requests := resources["requests"].(map[string]string)
		assert.Equal(t, "20Gi", requests["storage"]) // Default 20GB
	})
}

func TestVMTranslator_UpdateVMFromKubeVirt(t *testing.T) {
	translator := k8s.NewVMTranslator()

	t.Run("Update VM from KubeVirt data", func(t *testing.T) {
		vm := &models.VM{
			Status: "POWERED_OFF",
		}

		running := true
		memoryQuantity := resource.NewQuantity(8192*1024*1024, resource.BinarySI)
		kvVM := &kubevirtv1.VirtualMachine{
			Spec: kubevirtv1.VirtualMachineSpec{
				Running: &running,
				Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
					Spec: kubevirtv1.VirtualMachineInstanceSpec{
						Domain: kubevirtv1.DomainSpec{
							CPU: &kubevirtv1.CPU{
								Cores: 4,
							},
							Resources: kubevirtv1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: *memoryQuantity,
								},
							},
						},
					},
				},
			},
			Status: kubevirtv1.VirtualMachineStatus{
				Ready: true,
			},
		}

		translator.UpdateVMFromKubeVirt(vm, kvVM)

		assert.Equal(t, "POWERED_ON", vm.Status)
		assert.NotNil(t, vm.CPUCount)
		assert.Equal(t, 4, *vm.CPUCount)
		assert.NotNil(t, vm.MemoryMB)
		assert.Equal(t, 8192, *vm.MemoryMB)
	})

	t.Run("Handle nil inputs gracefully", func(t *testing.T) {
		translator.UpdateVMFromKubeVirt(nil, nil)
		// Should not panic
	})
}

func TestClient_CacheIntegration(t *testing.T) {
	// Note: These tests verify the client structure and methods exist
	// Integration tests with real Kubernetes clusters would require test infrastructure

	t.Run("Client has cache functionality", func(t *testing.T) {
		// We can't create a real client without a cluster, but we can test the structure
		// This ensures the cache-related methods are available
		
		// Test that the client has the expected methods for caching
		// This is more of a compile-time check to ensure the interface is correct
		
		// Test that methods exist by checking they compile (they would fail at runtime without cluster)
		// This is mainly to ensure the API surface is correct
		var client *k8s.Client
		if client != nil {
			// These methods should exist and be callable
			_ = client.GetConfig()
			_ = client.GetCache()
		}
		
		// If we reach here, the methods compiled successfully
		assert.True(t, true, "Cache methods exist and compile")
	})
	
	t.Run("VMTranslator handles integer overflow safely", func(t *testing.T) {
		translator := k8s.NewVMTranslator()
		
		// Test very large CPU count that could cause overflow
		largeCPUCount := 2147483648 // Larger than int32 max
		vm := &models.VM{
			ID:        uuid.New(),
			Name:      "test-vm",
			VAppID:    uuid.New(),
			VMName:    "test-vm-k8s",
			Namespace: "test-namespace",
			Status:    "POWERED_OFF",
			CPUCount:  &largeCPUCount,
			MemoryMB:  nil,
		}

		kvVM, err := translator.ToKubeVirtVM(vm)
		assert.Error(t, err)
		assert.Nil(t, kvVM)
		assert.Contains(t, err.Error(), "CPU count")
		assert.Contains(t, err.Error(), "out of valid range")
	})
}
