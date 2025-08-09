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
	err = gormDB.AutoMigrate(&models.User{}, &models.Organization{}, &models.Role{}, &models.UserRole{}, &models.VDC{}, &models.Catalog{}, &models.VAppTemplate{}, &models.VApp{}, &models.VM{})
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
	roleRepo := repositories.NewRoleRepository(gormDB)
	orgRepo := repositories.NewOrganizationRepository(gormDB)
	vdcRepo := repositories.NewVDCRepository(gormDB)
	catalogRepo := repositories.NewCatalogRepository(gormDB)
	templateRepo := repositories.NewVAppTemplateRepository(gormDB)
	vappRepo := repositories.NewVAppRepository(gormDB)
	vmRepo := repositories.NewVMRepository(gormDB)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	authSvc := auth.NewService(userRepo, jwtManager)

	// Create API server
	server := api.NewServer(cfg, db, authSvc, jwtManager, userRepo, roleRepo, orgRepo, vdcRepo, catalogRepo, templateRepo, vappRepo, vmRepo)

	return server, db, jwtManager
}

func TestHealthEndpoint(t *testing.T) {
	server, _, _ := setupTestAPIServer(t)
	router := server.GetRouter()

	t.Run("Health check returns 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/healthz", nil)
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
		req, _ := http.NewRequest("GET", "/readyz", nil)
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
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Enabled:  true,
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
		assert.Equal(t, user.ID, data["id"])
		assert.Equal(t, "testuser", data["username"])
		assert.Equal(t, "test@example.com", data["email"])
		assert.Equal(t, "Test User", data["full_name"])
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
		req, _ := http.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})

	t.Run("OPTIONS request returns 204", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/healthz", nil)
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

// Organization endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

// VDC endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

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
		Username: "authuser",
		Email:    "authuser@example.com",
		FullName: "Auth User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create inactive user for testing
	inactiveUser := &models.User{
		Username: "inactiveuser",
		Email:    "inactive@example.com",
		FullName: "Inactive User",
		Enabled:  false,
	}
	require.NoError(t, inactiveUser.SetPassword("password123"))
	require.NoError(t, db.DB.Create(inactiveUser).Error)

	// Explicitly set to inactive to override any GORM defaults
	require.NoError(t, db.DB.Model(inactiveUser).Update("enabled", false).Error)

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
		assert.Equal(t, user.ID, userData["id"])
		assert.Equal(t, "authuser", userData["username"])
		assert.Equal(t, "authuser@example.com", userData["email"])
		assert.Equal(t, "Auth User", userData["full_name"])

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
		assert.Equal(t, user.ID, data["user_id"])
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
		assert.Equal(t, user.ID, userData["id"])
		assert.Equal(t, "authuser", userData["username"])
		assert.Equal(t, "authuser@example.com", userData["email"])
		assert.Equal(t, "Auth User", userData["full_name"])
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

// Catalog endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

// VApp template instantiation tests removed as these legacy endpoints are not part of CloudAPI specification

// VM endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

// VM power operations tests removed as these legacy endpoints are not part of CloudAPI specification
