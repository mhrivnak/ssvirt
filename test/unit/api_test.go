package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

func setupTestAPIServer(t *testing.T) (*api.Server, *database.DB, *auth.JWTManager) {
	// Create in-memory SQLite database
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = gormDB.AutoMigrate(&models.User{}, &models.Organization{}, &models.UserRole{}, &models.VDC{})
	require.NoError(t, err)

	db := &database.DB{DB: gormDB}

	// Create test configuration
	cfg := &config.Config{
		API: struct {
			Port    int    `mapstructure:"port"`
			TLSCert string `mapstructure:"tls_cert"`
			TLSKey  string `mapstructure:"tls_key"`
		}{
			Port: 8080,
		},
		Auth: struct {
			JWTSecret   string        `mapstructure:"jwt_secret"`
			TokenExpiry time.Duration `mapstructure:"token_expiry"`
		}{
			JWTSecret:   "test-secret",
			TokenExpiry: time.Hour,
		},
		Log: struct {
			Level  string `mapstructure:"level"`
			Format string `mapstructure:"format"`
		}{
			Level: "debug",
		},
	}

	// Initialize repositories and services
	userRepo := repositories.NewUserRepository(gormDB)
	orgRepo := repositories.NewOrganizationRepository(gormDB)
	vdcRepo := repositories.NewVDCRepository(gormDB)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	authSvc := auth.NewService(userRepo, jwtManager)

	// Create API server
	server := api.NewServer(cfg, db, authSvc, jwtManager, orgRepo, vdcRepo)

	return server, db, jwtManager
}

func TestHealthEndpoint(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("Health check returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "ok", response["status"])
		assert.Equal(t, "1.0.0", response["version"])
		assert.Equal(t, "ok", response["database"])
		assert.Contains(t, response, "timestamp")
	})

	t.Run("API v1 health check returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestReadinessEndpoint(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("Readiness check returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/ready", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["ready"])
		assert.Contains(t, response, "timestamp")
		assert.Contains(t, response, "services")

		services, ok := response["services"].(map[string]interface{})
		require.True(t, ok, "services field should be a map[string]interface{}")
		assert.Equal(t, "ready", services["database"])
		assert.Equal(t, "ready", services["auth"])
	})
}

func TestVersionEndpoint(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("Version endpoint returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/version", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "1.0.0", response["version"])
		assert.Equal(t, "dev", response["build_time"])
		assert.Equal(t, runtime.Version(), response["go_version"])
		assert.Equal(t, "dev", response["git_commit"])
	})
}

func TestUserProfileEndpoint(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create a test user
	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Generate token using the JWT manager from server setup
	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("User profile with valid token returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/user/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")
		assert.Equal(t, user.ID.String(), data["id"])
		assert.Equal(t, "testuser", data["username"])
		assert.Equal(t, "test@example.com", data["email"])
		assert.Equal(t, "Test", data["first_name"])
		assert.Equal(t, "User", data["last_name"])
	})

	t.Run("User profile without token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/user/profile", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Authorization header required", response["error"])
	})

	t.Run("User profile with invalid token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/user/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestCORSMiddleware(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("CORS headers are set", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})

	t.Run("OPTIONS request returns 204", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestErrorHandling(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("404 for unknown endpoint", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/unknown", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestOrganizationEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organizations
	org1 := &models.Organization{
		Name:        "test-org-1",
		DisplayName: "Test Organization 1",
		Description: "Test description 1",
		Enabled:     true,
		Namespace:   "test-org-1-ns",
	}
	org2 := &models.Organization{
		Name:        "test-org-2",
		DisplayName: "Test Organization 2",
		Description: "Test description 2",
		Enabled:     false,
		Namespace:   "test-org-2-ns",
	}
	require.NoError(t, db.DB.Create(org1).Error)
	require.NoError(t, db.DB.Create(org2).Error)

	// Create test user and token
	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("GET /api/org returns organization list", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/org", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")

		organizations, ok := data["organizations"].([]interface{})
		require.True(t, ok, "organizations field should be an array")
		assert.Equal(t, 2, len(organizations))
		assert.Equal(t, float64(2), data["total"])

		// Check first organization
		firstOrg := organizations[0].(map[string]interface{})
		assert.Equal(t, org1.ID.String(), firstOrg["id"])
		assert.Equal(t, "test-org-1", firstOrg["name"])
		assert.Equal(t, "Test Organization 1", firstOrg["display_name"])
	})

	t.Run("GET /api/org/{org-id} returns specific organization", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/org/"+org1.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")

		assert.Equal(t, org1.ID.String(), data["id"])
		assert.Equal(t, "test-org-1", data["name"])
		assert.Equal(t, "Test Organization 1", data["display_name"])
		assert.Equal(t, "Test description 1", data["description"])
		assert.Equal(t, true, data["enabled"])
		assert.Equal(t, "test-org-1-ns", data["namespace"])
	})

	t.Run("GET /api/org/{org-id} with invalid ID returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/org/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response["error"])
		assert.Equal(t, "Invalid organization ID format", response["message"])
	})

	t.Run("GET /api/org/{org-id} with non-existent ID returns 404", func(t *testing.T) {
		nonExistentID := "550e8400-e29b-41d4-a716-446655440000"
		req, _ := http.NewRequest("GET", "/api/org/"+nonExistentID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Not Found", response["error"])
		assert.Equal(t, "Organization not found", response["message"])
	})

	t.Run("Organization endpoints without token return 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/org", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestVDCEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "test-org",
		DisplayName: "Test Organization",
		Description: "Test description",
		Enabled:     true,
		Namespace:   "test-org-ns",
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test VDC
	cpuLimit := 100
	memoryLimit := 8192
	storageLimit := 102400
	vdc := &models.VDC{
		Name:            "test-vdc",
		OrganizationID:  org.ID,
		AllocationModel: "PayAsYouGo",
		CPULimit:        &cpuLimit,
		MemoryLimitMB:   &memoryLimit,
		StorageLimitMB:  &storageLimit,
		Enabled:         true,
	}
	require.NoError(t, db.DB.Create(vdc).Error)

	// Create test user and token
	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("GET /api/vdc/{vdc-id} returns specific VDC", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/vdc/"+vdc.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")

		assert.Equal(t, vdc.ID.String(), data["id"])
		assert.Equal(t, "test-vdc", data["name"])
		assert.Equal(t, org.ID.String(), data["organization_id"])
		assert.Equal(t, "PayAsYouGo", data["allocation_model"])
		assert.Equal(t, float64(100), data["cpu_limit"])
		assert.Equal(t, float64(8192), data["memory_limit_mb"])
		assert.Equal(t, float64(102400), data["storage_limit_mb"])
		assert.Equal(t, true, data["enabled"])
	})

	t.Run("GET /api/vdc/{vdc-id} with invalid ID returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/vdc/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response["error"])
		assert.Equal(t, "Invalid VDC ID format", response["message"])
	})

	t.Run("GET /api/vdc/{vdc-id} with non-existent ID returns 404", func(t *testing.T) {
		nonExistentID := "550e8400-e29b-41d4-a716-446655440000"
		req, _ := http.NewRequest("GET", "/api/vdc/"+nonExistentID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Not Found", response["error"])
		assert.Equal(t, "VDC not found", response["message"])
	})

	t.Run("VDC endpoints without token return 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/vdc/"+vdc.ID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAPIErrorHelpers(t *testing.T) {
	t.Run("NewAPIError creates correct structure", func(t *testing.T) {
		apiErr := api.NewAPIError(400, "Bad Request", "Invalid input", "Field 'name' is required")

		assert.Equal(t, 400, apiErr.Code)
		assert.Equal(t, "Bad Request", apiErr.Error)
		assert.Equal(t, "Invalid input", apiErr.Message)
		assert.Equal(t, "Field 'name' is required", apiErr.Details)
	})

	t.Run("NewAPIError without details", func(t *testing.T) {
		apiErr := api.NewAPIError(500, "Internal Error", "Something went wrong")

		assert.Equal(t, 500, apiErr.Code)
		assert.Equal(t, "Internal Error", apiErr.Error)
		assert.Equal(t, "Something went wrong", apiErr.Message)
		assert.Empty(t, apiErr.Details)
	})
}