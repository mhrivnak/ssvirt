package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

func TestVMRepositorySimple(t *testing.T) {
	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Auto-migrate only VM schema for simple testing
	err = db.AutoMigrate(&models.VM{})
	assert.NoError(t, err)

	repo := NewVMRepository(db)

	// Test GetByNamespaceAndVMName with non-existent VM
	t.Run("GetByNamespaceAndVMName_NotFound", func(t *testing.T) {
		vm, err := repo.GetByNamespaceAndVMName(context.Background(), "test-namespace", "test-vm")
		assert.Error(t, err)
		assert.True(t, gorm.ErrRecordNotFound == err)
		assert.Nil(t, vm)
	})

	// Create a test VM directly
	testVM := &models.VM{
		ID:        "vm-123",
		Name:      "Test VM",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_OFF",
		VAppID:    "vapp-123", // This doesn't need to exist for this test
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = db.Create(testVM).Error
	assert.NoError(t, err)

	// Test GetByNamespaceAndVMName with existing VM
	t.Run("GetByNamespaceAndVMName_Found", func(t *testing.T) {
		vm, err := repo.GetByNamespaceAndVMName(context.Background(), "test-namespace", "test-vm")
		assert.NoError(t, err)
		assert.NotNil(t, vm)
		assert.Equal(t, "vm-123", vm.ID)
		assert.Equal(t, "test-vm", vm.VMName)
		assert.Equal(t, "test-namespace", vm.Namespace)
	})

	// Test GetByVAppAndVMName with existing VM
	t.Run("GetByVAppAndVMName_Found", func(t *testing.T) {
		vm, err := repo.GetByVAppAndVMName(context.Background(), "vapp-123", "test-vm")
		assert.NoError(t, err)
		assert.NotNil(t, vm)
		assert.Equal(t, "vm-123", vm.ID)
		assert.Equal(t, "test-vm", vm.VMName)
		assert.Equal(t, "vapp-123", vm.VAppID)
	})

	// Test UpdateStatus
	t.Run("UpdateStatus_Success", func(t *testing.T) {
		err := repo.UpdateStatus(context.Background(), "vm-123", "POWERED_ON")
		assert.NoError(t, err)

		// Verify the update
		var updatedVM models.VM
		err = db.First(&updatedVM, "id = ?", "vm-123").Error
		assert.NoError(t, err)
		assert.Equal(t, "POWERED_ON", updatedVM.Status)
		assert.True(t, updatedVM.UpdatedAt.After(testVM.UpdatedAt))
	})

	// Test UpdateStatus with non-existent VM
	t.Run("UpdateStatus_NotFound", func(t *testing.T) {
		err := repo.UpdateStatus(context.Background(), "nonexistent-vm", "POWERED_ON")
		assert.Error(t, err) // Should return ErrRecordNotFound for 0 rows affected
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})

	// Test with multiple VMs having same VMName but different namespaces
	t.Run("MultipleVMs_DifferentNamespaces", func(t *testing.T) {
		vm2 := &models.VM{
			ID:        "vm-456",
			Name:      "Test VM 2",
			VMName:    "test-vm",         // Same VM name
			Namespace: "other-namespace", // Different namespace
			Status:    "POWERED_OFF",
			VAppID:    "vapp-456",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = db.Create(vm2).Error
		assert.NoError(t, err)

		// Should find different VMs based on namespace
		vm1, err := repo.GetByNamespaceAndVMName(context.Background(), "test-namespace", "test-vm")
		assert.NoError(t, err)
		assert.Equal(t, "vm-123", vm1.ID)

		vm2Result, err := repo.GetByNamespaceAndVMName(context.Background(), "other-namespace", "test-vm")
		assert.NoError(t, err)
		assert.Equal(t, "vm-456", vm2Result.ID)
	})
}
