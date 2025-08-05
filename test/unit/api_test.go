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
	err = gormDB.AutoMigrate(&models.User{}, &models.Organization{}, &models.UserRole{})
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
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	authSvc := auth.NewService(userRepo, jwtManager)

	// Create API server
	server := api.NewServer(cfg, db, authSvc, jwtManager)

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