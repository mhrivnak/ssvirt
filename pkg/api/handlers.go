package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	Ready     bool      `json:"ready"`
	Timestamp time.Time `json:"timestamp"`
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
		GitCommit: "dev",   // TODO: Get from build info
	}

	c.JSON(http.StatusOK, response)
}

// UserProfileResponse represents the user profile response
type UserProfileResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
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
		ID:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	SendSuccess(c, http.StatusOK, response)
}

// OrganizationResponse represents an organization response
type OrganizationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Namespace   string `json:"namespace"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// OrganizationListResponse represents a list of organizations
type OrganizationListResponse struct {
	Organizations []OrganizationResponse `json:"organizations"`
	Total         int                    `json:"total"`
}

// organizationsHandler handles GET /api/org - list all organizations
func (s *Server) organizationsHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	_, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Get organizations from database
	orgs, err := s.orgRepo.List()
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve organizations"))
		return
	}

	// Convert to response format
	orgResponses := make([]OrganizationResponse, len(orgs))
	for i, org := range orgs {
		orgResponses[i] = OrganizationResponse{
			ID:          org.ID.String(),
			Name:        org.Name,
			DisplayName: org.DisplayName,
			Description: org.Description,
			Enabled:     org.Enabled,
			Namespace:   org.Namespace,
			CreatedAt:   org.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   org.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := OrganizationListResponse{
		Organizations: orgResponses,
		Total:         len(orgResponses),
	}

	SendSuccess(c, http.StatusOK, response)
}

// organizationHandler handles GET /api/org/{org-id} - get specific organization
func (s *Server) organizationHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	_, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse organization ID
	orgIDStr := c.Param("org-id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid organization ID format"))
		return
	}

	// Get organization from database
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "Organization not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve organization"))
		}
		return
	}

	response := OrganizationResponse{
		ID:          org.ID.String(),
		Name:        org.Name,
		DisplayName: org.DisplayName,
		Description: org.Description,
		Enabled:     org.Enabled,
		Namespace:   org.Namespace,
		CreatedAt:   org.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   org.UpdatedAt.Format(time.RFC3339),
	}

	SendSuccess(c, http.StatusOK, response)
}

// VDCResponse represents a VDC response
type VDCResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	OrganizationID  string `json:"organization_id"`
	AllocationModel string `json:"allocation_model"`
	CPULimit        *int   `json:"cpu_limit"`
	MemoryLimitMB   *int   `json:"memory_limit_mb"`
	StorageLimitMB  *int   `json:"storage_limit_mb"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// vdcHandler handles GET /api/vdc/{vdc-id} - get specific VDC
func (s *Server) vdcHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	_, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse VDC ID
	vdcIDStr := c.Param("vdc-id")
	vdcID, err := uuid.Parse(vdcIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid VDC ID format"))
		return
	}

	// Get VDC from database
	vdc, err := s.vdcRepo.GetByID(vdcID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "VDC not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve VDC"))
		}
		return
	}

	response := VDCResponse{
		ID:              vdc.ID.String(),
		Name:            vdc.Name,
		OrganizationID:  vdc.OrganizationID.String(),
		AllocationModel: string(vdc.AllocationModel),
		CPULimit:        vdc.CPULimit,
		MemoryLimitMB:   vdc.MemoryLimitMB,
		StorageLimitMB:  vdc.StorageLimitMB,
		Enabled:         vdc.Enabled,
		CreatedAt:       vdc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       vdc.UpdatedAt.Format(time.RFC3339),
	}

	SendSuccess(c, http.StatusOK, response)
}

// SessionRequest represents a session creation request (login)
type SessionRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SessionResponse represents a successful session creation response
type SessionResponse struct {
	Token     string           `json:"token"`
	ExpiresAt string           `json:"expires_at"`
	User      SessionUserInfo  `json:"user"`
}

// SessionUserInfo represents user information in session responses
type SessionUserInfo struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// CurrentSessionResponse represents current session information
type CurrentSessionResponse struct {
	Authenticated bool            `json:"authenticated"`
	User          SessionUserInfo `json:"user"`
	ExpiresAt     string          `json:"expires_at"`
}

// createSessionHandler handles POST /api/sessions - user login
func (s *Server) createSessionHandler(c *gin.Context) {
	var req SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid request body", err.Error()))
		return
	}

	// Convert to auth service request
	loginReq := &auth.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}

	// Authenticate user
	loginResp, err := s.authSvc.Login(loginReq)
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
			ID:        loginResp.User.ID.String(),
			Username:  loginResp.User.Username,
			Email:     loginResp.User.Email,
			FirstName: loginResp.User.FirstName,
			LastName:  loginResp.User.LastName,
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

	// In a more sophisticated implementation, we might maintain a blacklist of tokens
	// or store sessions in a database/cache. For now, we simply acknowledge the logout.
	// The client should discard the token.
	
	response := map[string]interface{}{
		"message":    "Session terminated successfully",
		"user_id":    claims.UserID.String(),
		"username":   claims.Username,
		"logged_out": true,
	}

	SendSuccess(c, http.StatusOK, response)
}

// getSessionHandler handles GET /api/session - get current session info
func (s *Server) getSessionHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Get full user details from database
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
			ID:        user.ID.String(),
			Username:  user.Username,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}

	SendSuccess(c, http.StatusOK, response)
}