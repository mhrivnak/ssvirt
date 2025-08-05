package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
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
	err = gormDB.AutoMigrate(&models.User{}, &models.Organization{}, &models.UserRole{}, &models.VDC{}, &models.Catalog{}, &models.VAppTemplate{}, &models.VApp{}, &models.VM{})
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
	catalogRepo := repositories.NewCatalogRepository(gormDB)
	templateRepo := repositories.NewVAppTemplateRepository(gormDB)
	vappRepo := repositories.NewVAppRepository(gormDB)
	vmRepo := repositories.NewVMRepository(gormDB)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	authSvc := auth.NewService(userRepo, jwtManager)

	// Create API server
	server := api.NewServer(cfg, db, authSvc, jwtManager, userRepo, orgRepo, vdcRepo, catalogRepo, templateRepo, vappRepo, vmRepo)

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

func TestAuthenticationEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test user
	user := &models.User{
		Username:  "authuser",
		Email:     "authuser@example.com",
		FirstName: "Auth",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create inactive user for testing
	inactiveUser := &models.User{
		Username:  "inactiveuser",
		Email:     "inactive@example.com",
		FirstName: "Inactive",
		LastName:  "User",
		IsActive:  false,
	}
	require.NoError(t, inactiveUser.SetPassword("password123"))
	require.NoError(t, db.DB.Create(inactiveUser).Error)

	// Explicitly set to inactive to override any GORM defaults
	require.NoError(t, db.DB.Model(inactiveUser).Update("is_active", false).Error)

	t.Run("POST /api/sessions with valid credentials creates session", func(t *testing.T) {
		loginData := map[string]string{
			"username": "authuser",
			"password": "password123",
		}
		jsonData, _ := json.Marshal(loginData)

		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")

		assert.Contains(t, data, "token")
		assert.Contains(t, data, "expires_at")
		assert.Contains(t, data, "user")

		userData := data["user"].(map[string]interface{})
		assert.Equal(t, user.ID.String(), userData["id"])
		assert.Equal(t, "authuser", userData["username"])
		assert.Equal(t, "authuser@example.com", userData["email"])
		assert.Equal(t, "Auth", userData["first_name"])
		assert.Equal(t, "User", userData["last_name"])

		// Verify token is valid
		token := data["token"].(string)
		_, err = jwtManager.Verify(token)
		assert.NoError(t, err)
	})

	t.Run("POST /api/sessions with invalid credentials returns 401", func(t *testing.T) {
		loginData := map[string]string{
			"username": "authuser",
			"password": "wrongpassword",
		}
		jsonData, _ := json.Marshal(loginData)

		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Unauthorized", response["error"])
		assert.Equal(t, "Invalid username or password", response["message"])
	})

	t.Run("POST /api/sessions with non-existent user returns 401", func(t *testing.T) {
		loginData := map[string]string{
			"username": "nonexistent",
			"password": "password123",
		}
		jsonData, _ := json.Marshal(loginData)

		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Unauthorized", response["error"])
		assert.Equal(t, "Invalid username or password", response["message"])
	})

	t.Run("POST /api/sessions with inactive user returns 403", func(t *testing.T) {
		loginData := map[string]string{
			"username": "inactiveuser",
			"password": "password123",
		}
		jsonData, _ := json.Marshal(loginData)

		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Forbidden", response["error"])
		assert.Equal(t, "User account is inactive", response["message"])
	})

	t.Run("POST /api/sessions with invalid JSON returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response["error"])
		assert.Equal(t, "Invalid request body", response["message"])
	})

	t.Run("POST /api/sessions with missing fields returns 400", func(t *testing.T) {
		loginData := map[string]string{
			"username": "authuser",
			// password missing
		}
		jsonData, _ := json.Marshal(loginData)

		req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// Create a valid token for protected endpoint tests
	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("DELETE /api/sessions with valid token logs out", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/sessions", nil)
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

		assert.Equal(t, "Session terminated successfully", data["message"])
		assert.Equal(t, user.ID.String(), data["user_id"])
		assert.Equal(t, "authuser", data["username"])
		assert.Equal(t, true, data["logged_out"])
	})

	t.Run("DELETE /api/sessions without token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("DELETE /api/sessions with invalid token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/sessions", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("GET /api/session with valid token returns session info", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/session", nil)
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

		assert.Equal(t, true, data["authenticated"])
		assert.Contains(t, data, "expires_at")

		userData := data["user"].(map[string]interface{})
		assert.Equal(t, user.ID.String(), userData["id"])
		assert.Equal(t, "authuser", userData["username"])
		assert.Equal(t, "authuser@example.com", userData["email"])
		assert.Equal(t, "Auth", userData["first_name"])
		assert.Equal(t, "User", userData["last_name"])
	})

	t.Run("GET /api/session without token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/session", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("GET /api/session with invalid token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/session", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestCatalogEndpoints(t *testing.T) {
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

	// Create test catalogs
	catalog1 := &models.Catalog{
		Name:           "test-catalog-1",
		OrganizationID: org.ID,
		Description:    "Test catalog 1",
		IsShared:       false,
	}
	catalog2 := &models.Catalog{
		Name:           "shared-catalog",
		OrganizationID: org.ID,
		Description:    "Shared catalog",
		IsShared:       true,
	}
	require.NoError(t, db.DB.Create(catalog1).Error)
	require.NoError(t, db.DB.Create(catalog2).Error)

	// Create test vApp templates
	cpuCount := 2
	memoryMB := 4096
	diskSizeGB := 20
	template1 := &models.VAppTemplate{
		Name:           "ubuntu-template",
		CatalogID:      catalog1.ID,
		Description:    "Ubuntu 20.04 template",
		OSType:         "linux",
		VMInstanceType: "standard.small",
		CPUCount:       &cpuCount,
		MemoryMB:       &memoryMB,
		DiskSizeGB:     &diskSizeGB,
		TemplateData:   `{"vm_spec": {"cpu": 2, "memory": "4Gi"}}`,
	}
	template2 := &models.VAppTemplate{
		Name:        "centos-template",
		CatalogID:   catalog1.ID,
		Description: "CentOS 8 template",
		OSType:      "linux",
	}
	require.NoError(t, db.DB.Create(template1).Error)
	require.NoError(t, db.DB.Create(template2).Error)

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

	// Create user role to give access to the organization
	userRole := &models.UserRole{
		UserID:         user.ID,
		OrganizationID: org.ID,
		Role:           "VAppUser",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("GET /api/catalogs/query returns catalog list", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/catalogs/query", nil)
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

		catalogs, ok := data["catalogs"].([]interface{})
		require.True(t, ok, "catalogs field should be an array")
		assert.Equal(t, 2, len(catalogs))
		assert.Equal(t, float64(2), data["total"])

		// Check first catalog
		firstCatalog := catalogs[0].(map[string]interface{})
		assert.Equal(t, catalog1.ID.String(), firstCatalog["id"])
		assert.Equal(t, "test-catalog-1", firstCatalog["name"])
		assert.Equal(t, "Test catalog 1", firstCatalog["description"])
		assert.Equal(t, false, firstCatalog["is_shared"])
	})

	t.Run("GET /api/catalog/{catalog-id} returns specific catalog with templates", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/catalog/"+catalog1.ID.String(), nil)
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

		assert.Equal(t, catalog1.ID.String(), data["id"])
		assert.Equal(t, "test-catalog-1", data["name"])
		assert.Equal(t, "Test catalog 1", data["description"])
		assert.Equal(t, false, data["is_shared"])
		assert.Equal(t, float64(2), data["template_count"])

		templates, ok := data["catalog_items"].([]interface{})
		require.True(t, ok, "catalog_items field should be an array")
		assert.Equal(t, 2, len(templates))

		// Check first template
		firstTemplate := templates[0].(map[string]interface{})
		assert.Equal(t, template1.ID.String(), firstTemplate["id"])
		assert.Equal(t, "ubuntu-template", firstTemplate["name"])
		assert.Equal(t, "Ubuntu 20.04 template", firstTemplate["description"])
		assert.Equal(t, "linux", firstTemplate["os_type"])
		assert.Equal(t, "standard.small", firstTemplate["vm_instance_type"])
		assert.Equal(t, float64(2), firstTemplate["cpu_count"])
		assert.Equal(t, float64(4096), firstTemplate["memory_mb"])
		assert.Equal(t, float64(20), firstTemplate["disk_size_gb"])
	})

	t.Run("GET /api/catalog/{catalog-id}/catalogItems/query returns catalog items", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/catalog/"+catalog1.ID.String()+"/catalogItems/query", nil)
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

		catalogItems, ok := data["catalog_items"].([]interface{})
		require.True(t, ok, "catalog_items field should be an array")
		assert.Equal(t, 2, len(catalogItems))
		assert.Equal(t, float64(2), data["total"])

		// Check first item
		firstItem := catalogItems[0].(map[string]interface{})
		assert.Equal(t, template1.ID.String(), firstItem["id"])
		assert.Equal(t, "ubuntu-template", firstItem["name"])
		assert.Equal(t, "Ubuntu 20.04 template", firstItem["description"])
	})

	t.Run("GET /api/catalog/{catalog-id} with invalid ID returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/catalog/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response["error"])
		assert.Equal(t, "Invalid catalog ID format", response["message"])
	})

	t.Run("GET /api/catalog/{catalog-id} with non-existent ID returns 404", func(t *testing.T) {
		nonExistentID := "550e8400-e29b-41d4-a716-446655440000"
		req, _ := http.NewRequest("GET", "/api/catalog/"+nonExistentID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Not Found", response["error"])
		assert.Equal(t, "Catalog not found", response["message"])
	})

	t.Run("Catalog endpoints without token return 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/catalogs/query", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestVAppTemplateInstantiation(t *testing.T) {
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

	// Create test catalog
	catalog := &models.Catalog{
		Name:           "test-catalog",
		OrganizationID: org.ID,
		Description:    "Test catalog",
		IsShared:       false,
	}
	require.NoError(t, db.DB.Create(catalog).Error)

	// Create test vApp template
	cpuCount := 2
	memoryMB := 4096
	diskSizeGB := 20
	template := &models.VAppTemplate{
		Name:           "ubuntu-template",
		CatalogID:      catalog.ID,
		Description:    "Ubuntu 20.04 template",
		OSType:         "linux",
		VMInstanceType: "standard.small",
		CPUCount:       &cpuCount,
		MemoryMB:       &memoryMB,
		DiskSizeGB:     &diskSizeGB,
		TemplateData:   `{"vm_spec": {"cpu": 2, "memory": "4Gi"}}`,
	}
	require.NoError(t, db.DB.Create(template).Error)

	// Create test user
	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create user role to give access to the organization
	userRole := &models.UserRole{
		UserID:         user.ID,
		OrganizationID: org.ID,
		Role:           "VAppUser",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	t.Run("POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate creates vApp", func(t *testing.T) {
		requestData := map[string]interface{}{
			"name":        "test-vapp",
			"description": "Test vApp from template",
			"source":      template.ID.String(),
			"deploy":      true,
			"power_on":    false,
		}
		jsonData, _ := json.Marshal(requestData)

		req, _ := http.NewRequest("POST", "/api/vdc/"+vdc.ID.String()+"/action/instantiateVAppTemplate", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map[string]interface{}")

		assert.Equal(t, "test-vapp", data["name"])
		assert.Equal(t, "Test vApp from template", data["description"])
		assert.Equal(t, "POWERED_OFF", data["status"])
		assert.Equal(t, vdc.ID.String(), data["vdc_id"])
		assert.Equal(t, template.ID.String(), data["template_id"])

		// Verify task info
		task, ok := data["task"].(map[string]interface{})
		require.True(t, ok, "task field should be a map[string]interface{}")
		assert.Equal(t, "running", task["status"])
		assert.Equal(t, "vappInstantiateFromTemplate", task["type"])
		assert.NotEmpty(t, task["id"])

		// Verify vApp was created in database
		var createdVApp models.VApp
		err = db.DB.Where("name = ?", "test-vapp").First(&createdVApp).Error
		require.NoError(t, err)
		assert.Equal(t, "test-vapp", createdVApp.Name)
		assert.Equal(t, vdc.ID, createdVApp.VDCID)
		assert.Equal(t, template.ID, *createdVApp.TemplateID)

		// Verify VM was created
		var createdVM models.VM
		err = db.DB.Where("v_app_id = ?", createdVApp.ID).First(&createdVM).Error
		require.NoError(t, err)
		assert.Equal(t, "test-vapp-vm-1", createdVM.Name)
		assert.Equal(t, org.Namespace, createdVM.Namespace)
		assert.Equal(t, cpuCount, *createdVM.CPUCount)
		assert.Equal(t, memoryMB, *createdVM.MemoryMB)
	})

	t.Run("POST with invalid VDC ID returns 400", func(t *testing.T) {
		requestData := map[string]interface{}{
			"name":   "test-vapp",
			"source": template.ID.String(),
		}
		jsonData, _ := json.Marshal(requestData)

		req, _ := http.NewRequest("POST", "/api/vdc/invalid-uuid/action/instantiateVAppTemplate", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST with non-existent VDC returns 404", func(t *testing.T) {
		nonExistentID := "550e8400-e29b-41d4-a716-446655440000"
		requestData := map[string]interface{}{
			"name":   "test-vapp",
			"source": template.ID.String(),
		}
		jsonData, _ := json.Marshal(requestData)

		req, _ := http.NewRequest("POST", "/api/vdc/"+nonExistentID+"/action/instantiateVAppTemplate", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("POST with invalid template ID returns 400", func(t *testing.T) {
		requestData := map[string]interface{}{
			"name":   "test-vapp",
			"source": "invalid-uuid",
		}
		jsonData, _ := json.Marshal(requestData)

		req, _ := http.NewRequest("POST", "/api/vdc/"+vdc.ID.String()+"/action/instantiateVAppTemplate", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST without token returns 401", func(t *testing.T) {
		requestData := map[string]interface{}{
			"name":   "test-vapp",
			"source": template.ID.String(),
		}
		jsonData, _ := json.Marshal(requestData)

		req, _ := http.NewRequest("POST", "/api/vdc/"+vdc.ID.String()+"/action/instantiateVAppTemplate", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
