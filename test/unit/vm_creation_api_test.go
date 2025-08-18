package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/api/handlers"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

func TestVMCreationAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for VM creation API testing",
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
		Description:     "Test VDC for VM creation",
		OrganizationID:  org.ID,
		IsEnabled:       true,
		AllocationModel: models.AllocationPool,
		ProviderVdcName: "test-provider-vdc",
	}
	require.NoError(t, db.DB.Create(vdc).Error)

	// Create test catalog for template validation
	catalog := &models.Catalog{
		Name:           "test-catalog",
		Description:    "Test catalog for VM creation",
		OrganizationID: org.ID,
		IsPublished:    false,
	}
	require.NoError(t, db.DB.Create(catalog).Error)

	// Generate token
	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	t.Run("VM Creation", func(t *testing.T) {
		t.Run("Instantiate template with valid data returns 201", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name:        "test-vapp",
				Description: "Test vApp from template",
				CatalogItem: handlers.CatalogItem{
					ID:   "urn:vcloud:catalogitem:template-123",
					Name: "Ubuntu Template",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response handlers.VAppResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "test-vapp", response.Name)
			assert.Equal(t, "Test vApp from template", response.Description)
			assert.Equal(t, models.VAppStatusInstantiating, response.Status)
			assert.Equal(t, vdc.ID, response.VDCID)
			assert.Equal(t, "", response.TemplateID) // No longer auto-generated from description
			assert.Contains(t, response.ID, "urn:vcloud:vapp:")
			assert.Contains(t, response.Href, "/cloudapi/1.0.0/vapps/")
		})

		t.Run("Instantiate template with invalid VDC URN returns 400", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:template-123",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/invalid-vdc-id/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid VDC URN format")
		})

		t.Run("Instantiate template with nonexistent VDC returns 404", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:template-123",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:99999999-9999-9999-9999-999999999999/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "VDC not found")
		})

		t.Run("Instantiate template with duplicate name returns 409", func(t *testing.T) {
			// Create a vApp first
			vapp := &models.VApp{
				Name:        "duplicate-vapp",
				Description: "First vApp",
				VDCID:       vdc.ID,
				Status:      models.VAppStatusInstantiating,
			}
			require.NoError(t, db.DB.Create(vapp).Error)

			// Try to create another with the same name
			requestData := handlers.InstantiateTemplateRequest{
				Name: "duplicate-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:template-123",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusConflict, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Name already in use")
		})

		t.Run("Instantiate template without authentication returns 401", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:template-123",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("Instantiate template with invalid JSON returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer([]byte("invalid json")))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid request format")
		})

		t.Run("Instantiate template with missing required fields returns 400", func(t *testing.T) {
			requestData := map[string]interface{}{
				"description": "Missing name and catalog item",
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid request format")
		})

		t.Run("Instantiate template with invalid catalog item URN returns 400", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:", // Missing item identifier
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "missing item identifier")
		})

		t.Run("Instantiate template with wrong URN type returns 400", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:user:template-123", // Wrong URN type
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "must start with urn:vcloud:catalogitem:")
		})

		t.Run("Instantiate template with catalog item URN containing invalid characters returns 400", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:invalid@#$%characters", // Invalid characters
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid catalog item URN format")
		})

		t.Run("Instantiate template with invalid catalog item format returns 400", func(t *testing.T) {
			requestData := handlers.InstantiateTemplateRequest{
				Name: "test-vapp-invalid-format",
				CatalogItem: handlers.CatalogItem{
					ID: "urn:vcloud:catalogitem:invalid@format#with$special%chars", // Invalid characters in catalog item
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["message"], "Invalid catalog item URN format")
		})

		t.Run("System Administrator can instantiate template with valid data returns 201", func(t *testing.T) {
			// Create test user with System Administrator role
			adminUser := &models.User{
				Username:       "sysadmin",
				Email:          "sysadmin@example.com",
				FullName:       "System Administrator",
				Enabled:        true,
				OrganizationID: stringPtr(org.ID),
			}
			require.NoError(t, adminUser.SetPassword("password123"))
			require.NoError(t, db.DB.Create(adminUser).Error)

			// Create and assign System Administrator role
			systemAdminRole := &models.Role{
				Name:        models.RoleSystemAdmin,
				Description: "System Administrator role",
			}
			require.NoError(t, db.DB.Create(systemAdminRole).Error)

			// Assign System Administrator role to admin user
			err := db.DB.Exec("INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)", adminUser.ID, systemAdminRole.ID).Error
			require.NoError(t, err)

			// Generate token for system admin
			adminToken, err := jwtManager.GenerateWithRole(adminUser.ID, adminUser.Username, org.ID, models.RoleSystemAdmin)
			require.NoError(t, err)

			requestData := handlers.InstantiateTemplateRequest{
				Name:        "sysadmin-test-vapp",
				Description: "Test vApp from template by System Admin",
				CatalogItem: handlers.CatalogItem{
					ID:   "urn:vcloud:catalogitem:admin-template-123",
					Name: "Admin Ubuntu Template",
				},
			}

			jsonData, _ := json.Marshal(requestData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vdcs/"+vdc.ID+"/actions/instantiateTemplate", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response handlers.VAppResponse
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "sysadmin-test-vapp", response.Name)
			assert.Equal(t, "Test vApp from template by System Admin", response.Description)
			assert.Equal(t, models.VAppStatusInstantiating, response.Status)
			assert.Equal(t, vdc.ID, response.VDCID)
			assert.Equal(t, "", response.TemplateID) // No longer auto-generated from description
			assert.Contains(t, response.ID, "urn:vcloud:vapp:")
			assert.Contains(t, response.Href, "/cloudapi/1.0.0/vapps/")
		})
	})
}
