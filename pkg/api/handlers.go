package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

// SessionResponse represents a successful session creation response
type SessionResponse struct {
	Token     string          `json:"token"`
	ExpiresAt string          `json:"expires_at"`
	User      SessionUserInfo `json:"user"`
}

// SessionUserInfo represents user information in session responses
type SessionUserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// CurrentSessionResponse represents current session information
type CurrentSessionResponse struct {
	Authenticated bool            `json:"authenticated"`
	User          SessionUserInfo `json:"user"`
	ExpiresAt     string          `json:"expires_at"`
}

// createSessionHandler handles POST /api/sessions - user login
func (s *Server) createSessionHandler(c *gin.Context) {
	var loginReq auth.LoginRequest
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid request body"))
		return
	}

	// Attempt to authenticate the user
	loginResp, err := s.authSvc.Login(&loginReq)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid username or password"))
		} else if err == auth.ErrUserInactive {
			SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "User account is inactive"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Authentication failed"))
		}
		return
	}

	response := SessionResponse{
		Token:     loginResp.Token,
		ExpiresAt: loginResp.ExpiresAt.Format(time.RFC3339),
		User: SessionUserInfo{
			ID:       loginResp.User.ID,
			Username: loginResp.User.Username,
			Email:    loginResp.User.Email,
			FullName: loginResp.User.FullName,
		},
	}

	SendSuccess(c, http.StatusCreated, response)
}

// deleteSessionHandler handles DELETE /api/sessions - user logout
func (s *Server) deleteSessionHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// NOTE: In a production system, proper JWT invalidation would require maintaining
	// a blacklist of tokens or using short-lived tokens with refresh tokens. This
	// should be implemented using Redis or similar cache to store invalidated tokens
	// until their expiration time. For now, we acknowledge the logout and expect
	// the client to discard the token.

	response := map[string]interface{}{
		"message":    "Session terminated successfully",
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"logged_out": true,
	}

	SendSuccess(c, http.StatusOK, response)
}

// getSessionHandler handles GET /api/session - get current session information
func (s *Server) getSessionHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Get user details from database
	user, err := s.authSvc.GetUser(claims.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "User not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		}
		return
	}

	// Get token expiration from claims
	expiresAt := claims.ExpiresAt.Time

	response := CurrentSessionResponse{
		Authenticated: true,
		User: SessionUserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
		},
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}

	SendSuccess(c, http.StatusOK, response)
}