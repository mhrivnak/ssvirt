package unit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)


func TestVDCAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for VDC API testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test user with System Administrator role
	user := &models.User{
		Username: "sysadmin",
		Email:    "sysadmin@example.com",
		FullName: "System Administrator",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create and assign System Administrator role
	adminRole := &models.Role{
		Name:        models.RoleSystemAdmin,
		Description: "System Administrator role",
	}
	require.NoError(t, db.DB.Create(adminRole).Error)

	// Assign System Administrator role to admin user
	require.NoError(t, db.DB.Model(user).Association("Roles").Append(adminRole))

	// Create test user without admin role (for authorization tests)
	regularUser := &models.User{
		Username: "regularuser",
		Email:    "regular@example.com",
		FullName: "Regular User",
		Enabled:  true,
	}
	require.NoError(t, regularUser.SetPassword("password123"))
	require.NoError(t, db.DB.Create(regularUser).Error)

	userRole := &models.Role{
		Name:        models.RoleVAppUser,
		Description: "vApp User role",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	// Assign vApp User role to regular user
	require.NoError(t, db.DB.Model(regularUser).Association("Roles").Append(userRole))

	// Generate tokens (using session-style tokens like the real session handler)
	adminToken, err := jwtManager.GenerateWithSessionID(user.ID, user.Username, "test-session-admin")
	require.NoError(t, err)

	regularToken, err := jwtManager.GenerateWithSessionID(regularUser.ID, regularUser.Username, "test-session-regular")
	require.NoError(t, err)

	t.Run("Authorization Tests", func(t *testing.T) {
		t.Run("List VDCs without authorization returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("List VDCs with non-admin user returns 403", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+regularToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "Forbidden", response["error"])
			assert.Equal(t, "System Administrator role required", response["message"])
		})

		t.Run("List VDCs with admin user returns 200", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("CRUD Operations", func(t *testing.T) {
		var createdVDCID string

		t.Run("Create VDC with valid data returns 201", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"name":            "Test VDC",
				"description":     "Test VDC for API testing",
				"allocationModel": "Flex",
				"computeCapacity": map[string]interface{}{
					"cpu": map[string]interface{}{
						"allocated": 10000,
						"limit":     20000,
						"units":     "MHz",
					},
					"memory": map[string]interface{}{
						"allocated": 16384,
						"limit":     32768,
						"units":     "MB",
					},
				},
				"providerVdc": map[string]interface{}{
					"id": "urn:vcloud:providervdc:12345",
				},
				"nicQuota":        100,
				"networkQuota":    50,
				"isThinProvision": false,
				"isEnabled":       true,
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Test VDC", response["name"])
			assert.Equal(t, "Test VDC for API testing", response["description"])
			assert.Equal(t, "Flex", response["allocationModel"])
			assert.Equal(t, true, response["isEnabled"])
			assert.Contains(t, response["id"], "urn:vcloud:vdc:")

			createdVDCID = response["id"].(string)

			// Verify compute capacity structure
			computeCapacity := response["computeCapacity"].(map[string]interface{})
			cpu := computeCapacity["cpu"].(map[string]interface{})
			memory := computeCapacity["memory"].(map[string]interface{})

			assert.Equal(t, float64(10000), cpu["allocated"])
			assert.Equal(t, float64(20000), cpu["limit"])
			assert.Equal(t, "MHz", cpu["units"])
			assert.Equal(t, float64(16384), memory["allocated"])
			assert.Equal(t, float64(32768), memory["limit"])
			assert.Equal(t, "MB", memory["units"])
		})

		t.Run("Create VDC with invalid organization returns 404", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"name":            "Test VDC",
				"allocationModel": "Flex",
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", "/api/admin/org/urn:vcloud:org:nonexistent/vdcs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Create VDC with invalid URN format returns 400", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"name":            "Test VDC",
				"allocationModel": "Flex",
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", "/api/admin/org/invalid-urn/vdcs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Create VDC with invalid allocation model returns 400", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"name":            "Test VDC",
				"allocationModel": "InvalidModel",
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Get VDC returns correct data", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, createdVDCID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, createdVDCID, response["id"])
			assert.Equal(t, "Test VDC", response["name"])
			assert.Equal(t, "Test VDC for API testing", response["description"])
			assert.Equal(t, "Flex", response["allocationModel"])
		})

		t.Run("Get non-existent VDC returns 404", func(t *testing.T) {
			nonExistentID := "urn:vcloud:vdc:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, nonExistentID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Update VDC returns correct updated data", func(t *testing.T) {
			updateData := map[string]interface{}{
				"name":        "Updated Test VDC",
				"description": "Updated description",
				"isEnabled":   false,
			}

			jsonData, _ := json.Marshal(updateData)
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, createdVDCID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Updated Test VDC", response["name"])
			assert.Equal(t, "Updated description", response["description"])
			assert.Equal(t, false, response["isEnabled"])
		})

		t.Run("Delete VDC returns 204", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, createdVDCID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Empty(t, w.Body.String())
		})

		t.Run("Get deleted VDC returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, createdVDCID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	})

	t.Run("Pagination Tests", func(t *testing.T) {
		// Create multiple VDCs for pagination testing
		vdcIDs := make([]string, 0)
		for i := 0; i < 5; i++ {
			vdcData := map[string]interface{}{
				"name":            fmt.Sprintf("VDC-%d", i),
				"description":     fmt.Sprintf("Test VDC %d for pagination", i),
				"allocationModel": "PayAsYouGo",
				"isEnabled":       true,
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			vdcIDs = append(vdcIDs, response["id"].(string))
		}

		t.Run("List VDCs with default pagination", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[interface{}]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(5), response.ResultTotal)
			assert.Equal(t, 1, response.PageCount)
			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 25, response.PageSize)
			assert.Len(t, response.Values, 5)
		})

		t.Run("List VDCs with custom page size", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs?page_size=2", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[interface{}]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(5), response.ResultTotal)
			assert.Equal(t, 3, response.PageCount) // ceil(5/2) = 3
			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 2, response.PageSize)
			assert.Len(t, response.Values, 2)
		})

		t.Run("List VDCs with page 2", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs?page=2&page_size=2", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[interface{}]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(5), response.ResultTotal)
			assert.Equal(t, 3, response.PageCount)
			assert.Equal(t, 2, response.Page)
			assert.Equal(t, 2, response.PageSize)
			assert.Len(t, response.Values, 2)
		})

		// Clean up VDCs
		for _, vdcID := range vdcIDs {
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, vdcID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusNoContent, w.Code)
		}
	})

	t.Run("Validation Tests", func(t *testing.T) {
		t.Run("Create VDC with missing name returns 400", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"allocationModel": "Flex",
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Create VDC with missing allocation model returns 400", func(t *testing.T) {
			vdcData := map[string]interface{}{
				"name": "Test VDC",
			}

			jsonData, _ := json.Marshal(vdcData)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/admin/org/%s/vdcs", org.ID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

	})

	t.Run("Error Scenarios", func(t *testing.T) {
		t.Run("Get VDC with invalid URN format returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/admin/org/%s/vdcs/invalid-urn", org.ID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Update non-existent VDC returns 404", func(t *testing.T) {
			nonExistentID := "urn:vcloud:vdc:00000000-0000-0000-0000-000000000000"
			updateData := map[string]interface{}{
				"name": "Updated Name",
			}

			jsonData, _ := json.Marshal(updateData)
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, nonExistentID), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Delete non-existent VDC returns 404", func(t *testing.T) {
			nonExistentID := "urn:vcloud:vdc:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, nonExistentID), nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	})
}

func TestVDCDependencyValidation(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for dependency testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test user with System Administrator role
	user := &models.User{
		Username: "sysadmin",
		Email:    "sysadmin@example.com",
		FullName: "System Administrator",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create and assign System Administrator role
	adminRole := &models.Role{
		Name:        models.RoleSystemAdmin,
		Description: "System Administrator role",
	}
	require.NoError(t, db.DB.Create(adminRole).Error)

	// Assign System Administrator role to user
	require.NoError(t, db.DB.Model(user).Association("Roles").Append(adminRole))

	adminToken, err := jwtManager.GenerateWithSessionID(user.ID, user.Username, "test-session-admin")
	require.NoError(t, err)

	// Create a VDC
	vdc := &models.VDC{
		Name:            "Test VDC",
		Description:     "Test VDC for dependency testing",
		OrganizationID:  org.ID,
		AllocationModel: models.PayAsYouGo,
		IsEnabled:       true,
	}
	require.NoError(t, db.DB.Create(vdc).Error)

	// Create a vApp in the VDC
	vapp := &models.VApp{
		Name:        "Test vApp",
		Description: "Test vApp in VDC",
		VDCID:       vdc.ID,
		Status:      "POWERED_OFF",
	}
	require.NoError(t, db.DB.Create(vapp).Error)

	t.Run("Delete VDC with dependent vApps returns 409", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, vdc.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Conflict", response["error"])
		assert.Contains(t, response["message"], "dependent resources")
	})

	t.Run("Delete VDC after removing vApps succeeds", func(t *testing.T) {
		// First delete the vApp
		require.NoError(t, db.DB.Delete(vapp).Error)

		// Now delete the VDC should succeed
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/admin/org/%s/vdcs/%s", org.ID, vdc.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
