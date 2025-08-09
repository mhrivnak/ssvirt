package unit

import (
	"encoding/base64"
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
	err = gormDB.AutoMigrate(&models.User{}, &models.Organization{}, &models.Role{}, &models.VDC{}, &models.Catalog{}, &models.VAppTemplate{}, &models.VApp{}, &models.VM{})
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
		Session: struct {
			IdleTimeoutMinutes int `mapstructure:"idle_timeout_minutes"`
			Site               struct {
				Name string `mapstructure:"name"`
				ID   string `mapstructure:"id"`
			} `mapstructure:"site"`
			Location string `mapstructure:"location"`
		}{
			IdleTimeoutMinutes: 30,
			Site: struct {
				Name string `mapstructure:"name"`
				ID   string `mapstructure:"id"`
			}{
				Name: "SSVirt Provider",
				ID:   "urn:vcloud:site:00000000-0000-0000-0000-000000000001",
			},
			Location: "us-west-1",
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

func TestVCDSessionEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test user with roles and organization
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

	t.Run("POST /cloudapi/1.0.0/sessions with valid Basic Auth creates VCD session", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/sessions", nil)
		// Set Basic Authentication header
		auth := base64.StdEncoding.EncodeToString([]byte("authuser:password123"))
		req.Header.Set("Authorization", "Basic "+auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var session map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &session)
		require.NoError(t, err)

		// Verify VCD session structure
		assert.Contains(t, session, "id")
		assert.Contains(t, session, "site")
		assert.Contains(t, session, "user")
		assert.Contains(t, session, "org")
		assert.Contains(t, session, "operatingOrg")
		assert.Contains(t, session, "location")
		assert.Contains(t, session, "roles")
		assert.Contains(t, session, "roleRefs")
		assert.Contains(t, session, "sessionIdleTimeoutMinutes")

		// Verify session ID format
		sessionID := session["id"].(string)
		assert.True(t, strings.HasPrefix(sessionID, "urn:vcloud:session:"))

		// Verify user reference
		userRef := session["user"].(map[string]interface{})
		assert.Equal(t, "authuser", userRef["name"])
		assert.Equal(t, user.ID, userRef["id"])

		// Verify site reference
		siteRef := session["site"].(map[string]interface{})
		assert.Equal(t, "SSVirt Provider", siteRef["name"])
		assert.True(t, strings.HasPrefix(siteRef["id"].(string), "urn:vcloud:site:"))

		// Verify location
		assert.Equal(t, "us-west-1", session["location"])

		// Verify timeout
		assert.Equal(t, float64(30), session["sessionIdleTimeoutMinutes"])

		// Check Authorization header for JWT token
		assert.True(t, strings.HasPrefix(w.Header().Get("Authorization"), "Bearer "))
	})

	t.Run("POST /cloudapi/1.0.0/sessions with invalid credentials returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/sessions", nil)
		auth := base64.StdEncoding.EncodeToString([]byte("authuser:wrongpassword"))
		req.Header.Set("Authorization", "Basic "+auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(401), response["code"])
		assert.Equal(t, "Unauthorized", response["error"])
		assert.Equal(t, "Invalid username or password", response["message"])
	})

	t.Run("POST /cloudapi/1.0.0/sessions with inactive user returns 403", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/sessions", nil)
		auth := base64.StdEncoding.EncodeToString([]byte("inactiveuser:password123"))
		req.Header.Set("Authorization", "Basic "+auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(403), response["code"])
		assert.Equal(t, "Forbidden", response["error"])
		assert.Equal(t, "User account is inactive", response["message"])
	})

	t.Run("POST /cloudapi/1.0.0/sessions without Authorization header returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Create session for protected endpoint tests
	sessionID := "urn:vcloud:session:12345678-1234-1234-1234-123456789abc"
	token, err := jwtManager.GenerateWithSessionID(user.ID, user.Username, sessionID)
	require.NoError(t, err)

	t.Run("GET /cloudapi/1.0.0/sessions/{sessionId} with valid token returns session", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/sessions/"+sessionID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var session map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &session)
		require.NoError(t, err)

		// Verify session ID matches request
		assert.Equal(t, sessionID, session["id"])

		// Verify VCD session structure
		assert.Contains(t, session, "site")
		assert.Contains(t, session, "user")
		assert.Contains(t, session, "location")
	})

	t.Run("GET /cloudapi/1.0.0/sessions/{sessionId} with wrong session ID returns 403", func(t *testing.T) {
		wrongSessionID := "urn:vcloud:session:87654321-4321-4321-4321-cba987654321"
		req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/sessions/"+wrongSessionID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(403), response["code"])
		assert.Equal(t, "Forbidden", response["error"])
		assert.Equal(t, "Cannot access another user's session", response["message"])
	})

	t.Run("DELETE /cloudapi/1.0.0/sessions/{sessionId} with valid token logs out", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/sessions/"+sessionID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("DELETE /cloudapi/1.0.0/sessions/{sessionId} with wrong session ID returns 403", func(t *testing.T) {
		wrongSessionID := "urn:vcloud:session:87654321-4321-4321-4321-cba987654321"
		req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/sessions/"+wrongSessionID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// Catalog endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

// VApp template instantiation tests removed as these legacy endpoints are not part of CloudAPI specification

// VM endpoints tests removed as these legacy endpoints are not part of CloudAPI specification

// VM power operations tests removed as these legacy endpoints are not part of CloudAPI specification
