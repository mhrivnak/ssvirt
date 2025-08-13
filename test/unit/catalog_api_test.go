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


func TestCatalogAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for catalog API testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test user with vApp User role (catalogs are generally accessible)
	user := &models.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		FullName: "Test User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create vApp User role
	userRole := &models.Role{
		Name:        models.RoleVAppUser,
		Description: "vApp User role",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	// Generate token
	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	t.Run("CRUD Operations", func(t *testing.T) {
		var createdCatalogID string

		t.Run("Create catalog with valid data returns 201", func(t *testing.T) {
			catalogData := map[string]interface{}{
				"name":        "Test Catalog",
				"description": "Test catalog for API testing",
				"orgId":       org.ID,
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Test Catalog", response["name"])
			assert.Equal(t, "Test catalog for API testing", response["description"])
			assert.Equal(t, false, response["isPublished"])
			assert.Equal(t, false, response["isSubscribed"])
			assert.Equal(t, true, response["isLocal"])
			assert.Equal(t, float64(1), response["version"])
			assert.Contains(t, response["id"], "urn:vcloud:catalog:")

			createdCatalogID = response["id"].(string)

			// Verify org reference structure
			org := response["org"].(map[string]interface{})
			assert.Contains(t, org["id"], "urn:vcloud:org:")

			// Verify VCD-compliant response structure
			assert.Equal(t, float64(0), response["numberOfVAppTemplates"])
			assert.Equal(t, float64(0), response["numberOfMedia"])
			assert.NotNil(t, response["catalogStorageProfiles"])
			assert.NotNil(t, response["publishConfig"])
			assert.NotNil(t, response["subscriptionConfig"])
			assert.NotNil(t, response["distributedCatalogConfig"])
			assert.NotNil(t, response["owner"])
			assert.NotEmpty(t, response["creationDate"])
		})

		t.Run("Create catalog with invalid organization returns 404", func(t *testing.T) {
			catalogData := map[string]interface{}{
				"name":        "Test Catalog",
				"orgId":       "urn:vcloud:org:nonexistent",
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Create catalog with invalid URN format returns 400", func(t *testing.T) {
			catalogData := map[string]interface{}{
				"name":        "Test Catalog",
				"orgId":       "invalid-urn",
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Create catalog with missing name returns 400", func(t *testing.T) {
			catalogData := map[string]interface{}{
				"orgId":       org.ID,
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Get catalog returns correct data", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", createdCatalogID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, createdCatalogID, response["id"])
			assert.Equal(t, "Test Catalog", response["name"])
			assert.Equal(t, "Test catalog for API testing", response["description"])
			assert.Equal(t, false, response["isPublished"])
		})

		t.Run("Get non-existent catalog returns 404", func(t *testing.T) {
			nonExistentID := "urn:vcloud:catalog:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", nonExistentID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Get catalog with invalid URN format returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs/invalid-urn", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("Delete catalog returns 204", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", createdCatalogID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Empty(t, w.Body.String())
		})

		t.Run("Get deleted catalog returns 404", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", createdCatalogID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Delete non-existent catalog returns 404", func(t *testing.T) {
			nonExistentID := "urn:vcloud:catalog:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", nonExistentID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Delete catalog with invalid URN format returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/catalogs/invalid-urn", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("Pagination Tests", func(t *testing.T) {
		// Create multiple catalogs for pagination testing
		catalogIDs := make([]string, 0)
		for i := 0; i < 5; i++ {
			catalogData := map[string]interface{}{
				"name":        fmt.Sprintf("Catalog-%d", i),
				"description": fmt.Sprintf("Test catalog %d for pagination", i),
				"orgId":       org.ID,
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			catalogIDs = append(catalogIDs, response["id"].(string))
		}

		t.Run("List catalogs with default pagination", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
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

		t.Run("List catalogs with custom page size", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs?pageSize=2", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
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

		t.Run("List catalogs with page 2", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs?page=2&pageSize=2", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
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

		// Clean up catalogs
		for _, catalogID := range catalogIDs {
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", catalogID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusNoContent, w.Code)
		}
	})

	t.Run("Authentication Tests", func(t *testing.T) {
		t.Run("List catalogs without authorization returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("Create catalog without authorization returns 401", func(t *testing.T) {
			catalogData := map[string]interface{}{
				"name":        "Test Catalog",
				"orgId":       org.ID,
				"isPublished": false,
			}

			jsonData, _ := json.Marshal(catalogData)
			req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/catalogs", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}

func TestCatalogDependencyValidation(t *testing.T) {
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

	// Create test user
	user := &models.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		FullName: "Test User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	// Create a catalog
	catalog := &models.Catalog{
		Name:           "Test Catalog",
		Description:    "Test catalog for dependency testing",
		OrganizationID: org.ID,
		IsPublished:    false,
		IsSubscribed:   false,
		IsLocal:        true,
		Version:        1,
	}
	require.NoError(t, db.DB.Create(catalog).Error)

	// Create a vApp template in the catalog
	template := &models.VAppTemplate{
		Name:        "Test Template",
		Description: "Test template in catalog",
		CatalogID:   catalog.ID,
		OSType:      "ubuntu",
	}
	require.NoError(t, db.DB.Create(template).Error)

	t.Run("Delete catalog with dependent templates returns 409", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", catalog.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Conflict", response["error"])
		assert.Contains(t, response["message"], "dependent resources")
	})

	t.Run("Delete catalog after removing templates succeeds", func(t *testing.T) {
		// First delete the template
		require.NoError(t, db.DB.Delete(template).Error)

		// Now delete the catalog should succeed
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s", catalog.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
