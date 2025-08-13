package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)


func TestVDCNamespaceUniqueness(t *testing.T) {
	server, db, _ := setupTestAPIServer(t)

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for namespace testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Test creating VDCs with the same name after deletion
	t.Run("Can create VDC with same name after previous one is deleted", func(t *testing.T) {
		// Create first VDC
		vdc1 := &models.VDC{
			Name:            "Test VDC",
			Description:     "First VDC",
			OrganizationID:  org.ID,
			AllocationModel: models.PayAsYouGo,
			IsEnabled:       true,
		}
		require.NoError(t, db.DB.Create(vdc1).Error)
		assert.Equal(t, "vdc-testorganization-testvdc", vdc1.Namespace)

		// Delete the first VDC (soft delete)
		require.NoError(t, db.DB.Delete(vdc1).Error)

		// Create second VDC with the same name
		vdc2 := &models.VDC{
			Name:            "Test VDC", // Same name
			Description:     "Second VDC",
			OrganizationID:  org.ID,
			AllocationModel: models.Flex,
			IsEnabled:       true,
		}
		require.NoError(t, db.DB.Create(vdc2).Error)

		// Should get the same namespace since the first one is soft-deleted
		assert.Equal(t, "vdc-testorganization-testvdc", vdc2.Namespace)
		assert.NotEqual(t, vdc1.ID, vdc2.ID) // Different VDCs
	})

	t.Run("Generates unique namespace when both VDCs exist", func(t *testing.T) {
		// Create first VDC
		vdc1 := &models.VDC{
			Name:            "Unique Test VDC",
			Description:     "First VDC",
			OrganizationID:  org.ID,
			AllocationModel: models.PayAsYouGo,
			IsEnabled:       true,
		}
		require.NoError(t, db.DB.Create(vdc1).Error)
		assert.Equal(t, "vdc-testorganization-uniquetestvdc", vdc1.Namespace)

		// Create second VDC with the same name (without deleting the first)
		vdc2 := &models.VDC{
			Name:            "Unique Test VDC", // Same name
			Description:     "Second VDC",
			OrganizationID:  org.ID,
			AllocationModel: models.Flex,
			IsEnabled:       true,
		}
		require.NoError(t, db.DB.Create(vdc2).Error)

		// Should get a unique namespace with a suffix
		assert.Equal(t, "vdc-testorganization-uniquetestvdc-1", vdc2.Namespace)
		assert.NotEqual(t, vdc1.ID, vdc2.ID) // Different VDCs
	})

	_ = server // Avoid unused variable warning
}
