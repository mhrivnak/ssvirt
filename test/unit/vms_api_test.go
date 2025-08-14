package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/api/handlers"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

func TestVMAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for VM API testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test user with vApp User role
	user := &models.User{
		Username:       "testuser",
		Email:          "testuser@example.com",
		FullName:       "Test User",
		Enabled:        true,
		OrganizationID: stringPtr(org.ID),
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create vApp User role
	userRole := &models.Role{
		Name:        models.RoleVAppUser,
		Description: "vApp User role",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	// Create test VDC
	vdc := &models.VDC{
		Name:            "test-vdc",
		Description:     "Test VDC for VM testing",
		OrganizationID:  org.ID,
		IsEnabled:       true,
		AllocationModel: models.AllocationPool,
		ProviderVdcName: "test-provider-vdc",
	}
	require.NoError(t, db.DB.Create(vdc).Error)

	// Create test vApp
	vapp := &models.VApp{
		Name:        "test-vapp",
		Description: "Test vApp for VM testing",
		VDCID:       vdc.ID,
		Status:      "RESOLVED",
	}
	require.NoError(t, db.DB.Create(vapp).Error)

	// Create test VMs
	vm1 := &models.VM{
		Name:        "test-vm-1",
		Description: "First test VM",
		VAppID:      vapp.ID,
		Status:      "POWERED_ON",
		VMName:      "test-vm-1",
		Namespace:   "test-ns",
		CPUCount:    intPtr(2),
		MemoryMB:    intPtr(4096),
		GuestOS:     "Ubuntu Linux (64-bit)",
	}
	require.NoError(t, db.DB.Create(vm1).Error)

	vm2 := &models.VM{
		Name:        "test-vm-2",
		Description: "Second test VM",
		VAppID:      vapp.ID,
		Status:      "POWERED_OFF",
		VMName:      "test-vm-2",
		Namespace:   "test-ns",
		CPUCount:    intPtr(4),
		MemoryMB:    intPtr(8192),
		GuestOS:     "Windows Server 2019",
	}
	require.NoError(t, db.DB.Create(vm2).Error)

	vm3 := &models.VM{
		Name:      "minimal-vm",
		VAppID:    vapp.ID,
		Status:    "SUSPENDED",
		VMName:    "minimal-vm",
		Namespace: "test-ns",
		// No CPU, memory, or guest OS specified (test defaults)
	}
	require.NoError(t, db.DB.Create(vm3).Error)

	// Generate token
	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	t.Run("Get VM", func(t *testing.T) {
		t.Run("Get VM with full details returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm1.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response handlers.VMResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, vm1.ID, response.ID)
			assert.Equal(t, "test-vm-1", response.Name)
			assert.Equal(t, "First test VM", response.Description)
			assert.Equal(t, "POWERED_ON", response.Status)
			assert.Equal(t, vapp.ID, response.VAppID)
			assert.Equal(t, "Ubuntu Linux (64-bit)", response.GuestOS)
			assert.Equal(t, 2, response.Hardware.NumCPUs)
			assert.Equal(t, 1, response.Hardware.NumCoresPerSocket)
			assert.Equal(t, 4096, response.Hardware.MemoryMB)
			assert.Equal(t, "RUNNING", response.VMTools.Status)
			assert.Equal(t, "12.1.5", response.VMTools.Version)
			assert.Equal(t, "default-storage-policy", response.StorageProfile.Name)
			assert.Equal(t, "/cloudapi/1.0.0/storageProfiles/default-storage-policy", response.StorageProfile.Href)
			assert.Len(t, response.NetworkConnections, 1)
			assert.Equal(t, "default-network", response.NetworkConnections[0].NetworkName)
			assert.True(t, response.NetworkConnections[0].Connected)
			assert.Equal(t, "/cloudapi/1.0.0/vms/"+vm1.ID, response.Href)
		})

		t.Run("Get VM with different configuration returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm2.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response handlers.VMResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "test-vm-2", response.Name)
			assert.Equal(t, "Second test VM", response.Description)
			assert.Equal(t, "POWERED_OFF", response.Status)
			assert.Equal(t, "Windows Server 2019", response.GuestOS)
			assert.Equal(t, 4, response.Hardware.NumCPUs)
			assert.Equal(t, 8192, response.Hardware.MemoryMB)
		})

		t.Run("Get VM with default values returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm3.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response handlers.VMResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "minimal-vm", response.Name)
			assert.Equal(t, "Virtual machine minimal-vm", response.Description) // Default description
			assert.Equal(t, "SUSPENDED", response.Status)
			assert.Equal(t, "Ubuntu Linux (64-bit)", response.GuestOS) // Default guest OS
			assert.Equal(t, 2, response.Hardware.NumCPUs)              // Default CPU count
			assert.Equal(t, 4096, response.Hardware.MemoryMB)          // Default memory
		})

		t.Run("Get VM with invalid URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/invalid-vm-id", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid VM URN format")
		})

		t.Run("Get nonexistent VM returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/urn:vcloud:vm:99999999-9999-9999-9999-999999999999", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "VM not found")
		})

		t.Run("Get VM without authentication returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm1.ID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			if response["message"] != nil {
				assert.Contains(t, response["message"], "Authentication required")
			}
		})

		t.Run("Get VM with invalid token returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm1.ID, nil)
			req.Header.Set("Authorization", "Bearer invalid-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})

	t.Run("Access Control", func(t *testing.T) {
		// Create another organization and user to test access control
		otherOrg := &models.Organization{
			Name:        "Other Organization",
			DisplayName: "Other Organization",
			Description: "Other organization for access control testing",
			IsEnabled:   true,
		}
		require.NoError(t, db.DB.Create(otherOrg).Error)

		otherUser := &models.User{
			Username:       "otheruser",
			Email:          "otheruser@example.com",
			FullName:       "Other User",
			Enabled:        true,
			OrganizationID: stringPtr(otherOrg.ID),
		}
		require.NoError(t, otherUser.SetPassword("password123"))
		require.NoError(t, db.DB.Create(otherUser).Error)

		// Generate token for other user
		otherUserToken, err := jwtManager.GenerateWithRole(otherUser.ID, otherUser.Username, otherOrg.ID, models.RoleVAppUser)
		require.NoError(t, err)

		t.Run("Access VM from different organization returns 403", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vms/"+vm1.ID, nil)
			req.Header.Set("Authorization", "Bearer "+otherUserToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "VM access denied")
		})
	})
}

// Helper function to create int pointers for the test data
func intPtr(i int) *int {
	return &i
}
