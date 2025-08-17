package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

func TestOrgAPIAccessControl(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Initialize repositories for test data setup
	orgRepo := repositories.NewOrganizationRepository(db.DB)
	userRepo := repositories.NewUserRepository(db.DB)
	roleRepo := repositories.NewRoleRepository(db.DB)

	// Create test roles
	systemAdminRole := &models.Role{
		Name:        models.RoleSystemAdmin,
		Description: "System Administrator role",
		ReadOnly:    true,
	}
	require.NoError(t, roleRepo.Create(systemAdminRole))

	orgAdminRole := &models.Role{
		Name:        models.RoleOrgAdmin,
		Description: "Organization Administrator role",
		ReadOnly:    true,
	}
	require.NoError(t, roleRepo.Create(orgAdminRole))

	vappUserRole := &models.Role{
		Name:        models.RoleVAppUser,
		Description: "vApp User role",
		ReadOnly:    true,
	}
	require.NoError(t, roleRepo.Create(vappUserRole))

	// Create test organizations
	org1 := &models.Organization{
		Name:        "TestOrg1",
		DisplayName: "Test Organization 1",
		Description: "First test organization",
		IsEnabled:   true,
	}
	require.NoError(t, orgRepo.Create(org1))

	org2 := &models.Organization{
		Name:        "TestOrg2",
		DisplayName: "Test Organization 2",
		Description: "Second test organization",
		IsEnabled:   true,
	}
	require.NoError(t, orgRepo.Create(org2))

	org3 := &models.Organization{
		Name:        "TestOrg3",
		DisplayName: "Test Organization 3",
		Description: "Third test organization",
		IsEnabled:   true,
	}
	require.NoError(t, orgRepo.Create(org3))

	t.Run("List Organizations - System Administrator", func(t *testing.T) {
		// Create system admin user
		sysAdminUser := &models.User{
			Username: "sysadmin",
			Email:    "sysadmin@example.com",
			FullName: "System Administrator",
			Enabled:  true,
		}
		require.NoError(t, sysAdminUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(sysAdminUser))

		// Assign system admin role
		require.NoError(t, db.DB.Model(sysAdminUser).Association("Roles").Append(systemAdminRole))

		// Generate token
		token, err := jwtManager.Generate(sysAdminUser.ID, sysAdminUser.Username)
		require.NoError(t, err)

		// Test request
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)

		var response types.Page[models.Organization]
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// System admin should see all organizations
		assert.GreaterOrEqual(t, response.ResultTotal, int64(3)) // Our 3 test orgs
		assert.GreaterOrEqual(t, len(response.Values), 3)
	})

	t.Run("List Organizations - Organization Administrator", func(t *testing.T) {
		// Create org admin user
		orgAdminUser := &models.User{
			Username:       "orgadmin",
			Email:          "orgadmin@example.com",
			FullName:       "Organization Administrator",
			Enabled:        true,
			OrganizationID: &org1.ID, // Primary organization
		}
		require.NoError(t, orgAdminUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(orgAdminUser))

		// Assign org admin role for org1
		require.NoError(t, db.DB.Model(orgAdminUser).Association("Roles").Append(orgAdminRole))

		// Generate token
		token, err := jwtManager.Generate(orgAdminUser.ID, orgAdminUser.Username)
		require.NoError(t, err)

		// Test request
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)

		var response types.Page[models.Organization]
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Org admin should see only their organization
		assert.Equal(t, int64(1), response.ResultTotal)
		assert.Len(t, response.Values, 1)
		assert.Equal(t, "TestOrg1", response.Values[0].Name)
	})

	t.Run("List Organizations - vApp User", func(t *testing.T) {
		// Create vApp user
		vappUser := &models.User{
			Username:       "vappuser",
			Email:          "vappuser@example.com",
			FullName:       "vApp User",
			Enabled:        true,
			OrganizationID: &org2.ID, // Primary organization
		}
		require.NoError(t, vappUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(vappUser))

		// Assign vApp user role for org2
		require.NoError(t, db.DB.Model(vappUser).Association("Roles").Append(vappUserRole))

		// Generate token
		token, err := jwtManager.Generate(vappUser.ID, vappUser.Username)
		require.NoError(t, err)

		// Test request
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)

		var response types.Page[models.Organization]
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// vApp user should see only their primary organization
		assert.Equal(t, int64(1), response.ResultTotal)
		assert.Len(t, response.Values, 1)
		assert.Equal(t, "TestOrg2", response.Values[0].Name)
	})

	t.Run("List Organizations - Unauthorized", func(t *testing.T) {
		// Test request without authorization
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Authorization header required", response["error"])
	})

	t.Run("Get Organization - System Administrator", func(t *testing.T) {
		// Create system admin user
		sysAdminUser := &models.User{
			Username: "sysadmin2",
			Email:    "sysadmin2@example.com",
			FullName: "System Administrator 2",
			Enabled:  true,
		}
		require.NoError(t, sysAdminUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(sysAdminUser))

		// Assign system admin role
		require.NoError(t, db.DB.Model(sysAdminUser).Association("Roles").Append(systemAdminRole))

		// Generate token
		token, err := jwtManager.Generate(sysAdminUser.ID, sysAdminUser.Username)
		require.NoError(t, err)

		// Test request
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs/"+org3.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)

		var response models.Organization
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "TestOrg3", response.Name)
		assert.Equal(t, "Test Organization 3", response.DisplayName)
	})

	t.Run("Get Organization - Access Denied", func(t *testing.T) {
		// Create vApp user
		vappUser := &models.User{
			Username:       "vappuser2",
			Email:          "vappuser2@example.com",
			FullName:       "vApp User 2",
			Enabled:        true,
			OrganizationID: &org1.ID, // Primary organization is org1
		}
		require.NoError(t, vappUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(vappUser))

		// Assign vApp user role for org1
		require.NoError(t, db.DB.Model(vappUser).Association("Roles").Append(vappUserRole))

		// Generate token
		token, err := jwtManager.Generate(vappUser.ID, vappUser.Username)
		require.NoError(t, err)

		// Test request for org3 (which the user doesn't have access to)
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs/"+org3.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Organization not found", response["error"])
	})

	t.Run("Get Organization - Invalid URN", func(t *testing.T) {
		// Create system admin user
		sysAdminUser := &models.User{
			Username: "sysadmin3",
			Email:    "sysadmin3@example.com",
			FullName: "System Administrator 3",
			Enabled:  true,
		}
		require.NoError(t, sysAdminUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(sysAdminUser))

		// Assign system admin role
		require.NoError(t, db.DB.Model(sysAdminUser).Association("Roles").Append(systemAdminRole))

		// Generate token
		token, err := jwtManager.Generate(sysAdminUser.ID, sysAdminUser.Username)
		require.NoError(t, err)

		// Test request with invalid URN
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs/invalid-urn", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Invalid organization ID format", response["error"])
	})

	t.Run("List Organizations - Pagination", func(t *testing.T) {
		// Create system admin user
		sysAdminUser := &models.User{
			Username: "sysadmin4",
			Email:    "sysadmin4@example.com",
			FullName: "System Administrator 4",
			Enabled:  true,
		}
		require.NoError(t, sysAdminUser.SetPassword("password123"))
		require.NoError(t, userRepo.Create(sysAdminUser))

		// Assign system admin role
		require.NoError(t, db.DB.Model(sysAdminUser).Association("Roles").Append(systemAdminRole))

		// Generate token
		token, err := jwtManager.Generate(sysAdminUser.ID, sysAdminUser.Username)
		require.NoError(t, err)

		// Test request with pagination
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/orgs?page=1&page_size=2", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Assertions
		assert.Equal(t, http.StatusOK, w.Code)

		var response types.Page[models.Organization]
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, 1, response.Page)
		assert.Equal(t, 2, response.PageSize)
		assert.LessOrEqual(t, len(response.Values), 2)
		assert.Greater(t, response.PageCount, 1) // Should have multiple pages
	})
}
