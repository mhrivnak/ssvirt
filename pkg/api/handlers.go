package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhrivnak/ssvirt/pkg/auth"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	// Test database connection
	dbStatus := "ok"
	if sqlDB, err := s.db.DB.DB(); err != nil {
		dbStatus = "error"
	} else if err := sqlDB.Ping(); err != nil {
		dbStatus = "error"
	}

	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   "1.0.0", // TODO: Get from build info
		Database:  dbStatus,
	}

	// Return 503 if database is not healthy
	if dbStatus != "ok" {
		response.Status = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ReadinessResponse represents the readiness check response
type ReadinessResponse struct {
	Ready     bool              `json:"ready"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// readinessHandler handles readiness check requests
func (s *Server) readinessHandler(c *gin.Context) {
	services := make(map[string]string)
	allReady := true

	// Check database readiness
	if sqlDB, err := s.db.DB.DB(); err != nil {
		services["database"] = "not ready"
		allReady = false
	} else if err := sqlDB.Ping(); err != nil {
		services["database"] = "not ready"
		allReady = false
	} else {
		services["database"] = "ready"
	}

	// Check auth service readiness
	if s.authSvc != nil {
		services["auth"] = "ready"
	} else {
		services["auth"] = "not ready"
		allReady = false
	}

	response := ReadinessResponse{
		Ready:     allReady,
		Timestamp: time.Now(),
		Services:  services,
	}

	statusCode := http.StatusOK
	if !allReady {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, response)
}

// VersionResponse represents the version information response
type VersionResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	GitCommit string `json:"git_commit"`
}

// versionHandler handles version information requests
func (s *Server) versionHandler(c *gin.Context) {
	response := VersionResponse{
		Version:   "1.0.0", // TODO: Get from build info
		BuildTime: "dev",   // TODO: Get from build info
		GoVersion: runtime.Version(),
		GitCommit: "dev", // TODO: Get from build info
	}

	c.JSON(http.StatusOK, response)
}

// UserProfileResponse represents the user profile response
type UserProfileResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// userProfileHandler handles user profile requests (example authenticated endpoint)
func (s *Server) userProfileHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Get user details from database
	user, err := s.authSvc.GetUser(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "User not found"))
		return
	}

	response := UserProfileResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
	}

	SendSuccess(c, http.StatusOK, response)
}
