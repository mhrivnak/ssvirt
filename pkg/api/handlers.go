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