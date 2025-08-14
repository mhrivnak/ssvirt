package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/api/handlers"
	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

func TestVAppAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for vApp API testing",
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
		Description:     "Test VDC for vApp testing",
		OrganizationID:  org.ID,
		IsEnabled:       true,
		AllocationModel: models.AllocationPool,
		ProviderVdcName: "test-provider-vdc",
	}
	require.NoError(t, db.DB.Create(vdc).Error)

	// Create test vApps
	vapp1 := &models.VApp{
		Name:        "test-vapp-1",
		Description: "First test vApp",
		VDCID:       vdc.ID,
		Status:      "RESOLVED",
	}
	require.NoError(t, db.DB.Create(vapp1).Error)

	vapp2 := &models.VApp{
		Name:        "test-vapp-2",
		Description: "Second test vApp",
		VDCID:       vdc.ID,
		Status:      "SUSPENDED",
	}
	require.NoError(t, db.DB.Create(vapp2).Error)

	// Create test VM in vapp1
	vm1 := &models.VM{
		Name:      "test-vm-1",
		VAppID:    vapp1.ID,
		Status:    "POWERED_ON",
		VMName:    "test-vm-1",
		Namespace: "test-ns",
	}
	require.NoError(t, db.DB.Create(vm1).Error)

	// Generate token
	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	t.Run("List vApps", func(t *testing.T) {
		t.Run("List vApps in VDC returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/vapps", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[handlers.VAppResponse]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(2), response.ResultTotal)
			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 25, response.PageSize)
			assert.Len(t, response.Values, 2)

			// Verify vApp details
			for _, vappResp := range response.Values {
				if vappResp.Name == "test-vapp-1" {
					assert.Equal(t, "First test vApp", vappResp.Description)
					assert.Equal(t, "RESOLVED", vappResp.Status)
					assert.Equal(t, 1, vappResp.NumberOfVMs)
				} else if vappResp.Name == "test-vapp-2" {
					assert.Equal(t, "Second test vApp", vappResp.Description)
					assert.Equal(t, "SUSPENDED", vappResp.Status)
					assert.Equal(t, 0, vappResp.NumberOfVMs)
				}
			}
		})

		t.Run("List vApps with pagination returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/vapps?page=1&pageSize=1", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[handlers.VAppResponse]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(2), response.ResultTotal)
			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 1, response.PageSize)
			assert.Len(t, response.Values, 1)
		})

		t.Run("List vApps with filtering returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/vapps?filter="+vapp1.Name, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[handlers.VAppResponse]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(1), response.ResultTotal)
			assert.Len(t, response.Values, 1)
			assert.Equal(t, vapp1.Name, response.Values[0].Name)
		})

		t.Run("List vApps with invalid VDC URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/invalid-vdc-id/vapps", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid VDC URN format")
		})

		t.Run("List vApps with nonexistent VDC returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:99999999-9999-9999-9999-999999999999/vapps", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "VDC not found")
		})

		t.Run("List vApps without authentication returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/vapps", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})

	t.Run("Get vApp", func(t *testing.T) {
		t.Run("Get vApp returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vapps/"+vapp1.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response handlers.VAppDetailedResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "test-vapp-1", response.Name)
			assert.Equal(t, "First test vApp", response.Description)
			assert.Equal(t, "RESOLVED", response.Status)
			assert.Equal(t, vdc.ID, response.VDCID)
			assert.Equal(t, 1, response.NumberOfVMs)
			assert.Len(t, response.VMs, 1)
			assert.Equal(t, "test-vm-1", response.VMs[0].Name)
			assert.Equal(t, "POWERED_ON", response.VMs[0].Status)
		})

		t.Run("Get vApp with invalid URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vapps/invalid-vapp-id", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid vApp URN format")
		})

		t.Run("Get nonexistent vApp returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:99999999-9999-9999-9999-999999999999", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "vApp not found")
		})

		t.Run("Get vApp without authentication returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/vapps/"+vapp1.ID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})

	t.Run("Delete vApp", func(t *testing.T) {
		// Create a vApp specifically for deletion testing
		deleteVApp := &models.VApp{
			Name:        "delete-test-vapp",
			Description: "vApp for deletion testing",
			VDCID:       vdc.ID,
			Status:      "RESOLVED",
		}
		require.NoError(t, db.DB.Create(deleteVApp).Error)

		t.Run("Delete vApp returns 204", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+deleteVApp.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Empty(t, w.Body.String())

			// Verify vApp is deleted
			var count int64
			db.DB.Model(&models.VApp{}).Where("id = ?", deleteVApp.ID).Count(&count)
			assert.Equal(t, int64(0), count)
		})

		t.Run("Delete vApp with force parameter returns 204", func(t *testing.T) {
			// Create another vApp for force deletion testing
			forceDeleteVApp := &models.VApp{
				Name:        "force-delete-vapp",
				Description: "vApp for force deletion testing",
				VDCID:       vdc.ID,
				Status:      "RESOLVED",
			}
			require.NoError(t, db.DB.Create(forceDeleteVApp).Error)

			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+forceDeleteVApp.ID+"?force=true", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)
		})

		t.Run("Delete vApp with running VMs returns 400", func(t *testing.T) {
			// Create a vApp with a running VM
			runningVApp := &models.VApp{
				Name:        "running-vm-vapp",
				Description: "vApp with running VMs",
				VDCID:       vdc.ID,
				Status:      "RESOLVED",
			}
			require.NoError(t, db.DB.Create(runningVApp).Error)

			// Create a VM in POWERED_ON status
			runningVM := &models.VM{
				Name:        "running-vm",
				VAppID:      runningVApp.ID,
				Status:      "POWERED_ON",
				Description: "Running VM",
				GuestOS:     "Ubuntu",
			}
			require.NoError(t, db.DB.Create(runningVM).Error)

			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+runningVApp.ID, nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "running VMs")
		})

		t.Run("Delete vApp with invalid URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/invalid-vapp-id", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid vApp URN format")
		})

		t.Run("Delete nonexistent vApp returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:99999999-9999-9999-9999-999999999999", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "vApp not found")
		})

		t.Run("Delete vApp without authentication returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+vapp1.ID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}
