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

	// NOTE: This is a simplified logout implementation. In production, token blacklisting
	// should be implemented using Redis or similar cache to store invalidated tokens
	// until their expiration time. For now, we acknowledge the logout and expect
	// the client to discard the token.
	
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

// CatalogResponse represents a catalog response
type CatalogResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Description  string `json:"description"`
	IsShared     bool   `json:"is_shared"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CatalogQueryResponse represents a catalog query response
type CatalogQueryResponse struct {
	Catalogs []CatalogResponse `json:"catalogs"`
	Total    int               `json:"total"`
}

// catalogsQueryHandler handles GET /api/catalogs/query - list catalogs accessible to user
func (s *Server) catalogsQueryHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Get user with their organization roles
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Extract organization IDs from user roles
	orgIDs := make([]uuid.UUID, 0, len(user.UserRoles))
	for _, role := range user.UserRoles {
		orgIDs = append(orgIDs, role.OrganizationID)
	}

	// Get catalogs accessible to user (based on organization membership and shared catalogs)
	catalogs, err := s.catalogRepo.GetByOrganizationIDs(orgIDs)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve catalogs"))
		return
	}

	// Convert to response format
	catalogResponses := make([]CatalogResponse, len(catalogs))
	for i, catalog := range catalogs {
		catalogResponses[i] = CatalogResponse{
			ID:           catalog.ID.String(),
			Name:         catalog.Name,
			Organization: catalog.OrganizationID.String(),
			Description:  catalog.Description,
			IsShared:     catalog.IsShared,
			CreatedAt:    catalog.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    catalog.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := CatalogQueryResponse{
		Catalogs: catalogResponses,
		Total:    len(catalogResponses),
	}

	SendSuccess(c, http.StatusOK, response)
}

// CatalogDetailResponse represents a detailed catalog response with templates
type CatalogDetailResponse struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Description  string                    `json:"description"`
	IsShared     bool                      `json:"is_shared"`
	CreatedAt    string                    `json:"created_at"`
	UpdatedAt    string                    `json:"updated_at"`
	Templates    []CatalogItemResponse     `json:"catalog_items"`
	TemplateCount int                      `json:"template_count"`
}

// CatalogItemResponse represents a catalog item (vApp template) response
type CatalogItemResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	OSType         string `json:"os_type"`
	VMInstanceType string `json:"vm_instance_type"`
	CPUCount       *int   `json:"cpu_count"`
	MemoryMB       *int   `json:"memory_mb"`
	DiskSizeGB     *int   `json:"disk_size_gb"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// catalogHandler handles GET /api/catalog/{catalog-id} - get specific catalog with details
func (s *Server) catalogHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse catalog ID
	catalogIDStr := c.Param("catalog-id")
	catalogID, err := uuid.Parse(catalogIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid catalog ID format"))
		return
	}

	// Get catalog with templates from database
	catalog, err := s.catalogRepo.GetWithTemplates(catalogID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "Catalog not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve catalog"))
		}
		return
	}

	// Check if user has access to this catalog
	// Get user with their organization roles to verify access
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Check if catalog is shared (accessible to all users) or if user belongs to catalog's organization
	hasAccess := catalog.IsShared
	if !hasAccess {
		for _, role := range user.UserRoles {
			if role.OrganizationID == catalog.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to access this catalog"))
		return
	}

	// Convert templates to response format
	templateResponses := make([]CatalogItemResponse, len(catalog.VAppTemplates))
	for i, template := range catalog.VAppTemplates {
		templateResponses[i] = CatalogItemResponse{
			ID:             template.ID.String(),
			Name:           template.Name,
			Description:    template.Description,
			OSType:         template.OSType,
			VMInstanceType: template.VMInstanceType,
			CPUCount:       template.CPUCount,
			MemoryMB:       template.MemoryMB,
			DiskSizeGB:     template.DiskSizeGB,
			CreatedAt:      template.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      template.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := CatalogDetailResponse{
		ID:            catalog.ID.String(),
		Name:          catalog.Name,
		Description:   catalog.Description,
		IsShared:      catalog.IsShared,
		CreatedAt:     catalog.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     catalog.UpdatedAt.Format(time.RFC3339),
		Templates:     templateResponses,
		TemplateCount: len(templateResponses),
	}

	SendSuccess(c, http.StatusOK, response)
}

// CatalogItemsQueryResponse represents a catalog items query response
type CatalogItemsQueryResponse struct {
	CatalogItems []CatalogItemResponse `json:"catalog_items"`
	Total        int                   `json:"total"`
}

// catalogItemsQueryHandler handles GET /api/catalog/{catalog-id}/catalogItems/query - list catalog items
func (s *Server) catalogItemsQueryHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse catalog ID
	catalogIDStr := c.Param("catalog-id")
	catalogID, err := uuid.Parse(catalogIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid catalog ID format"))
		return
	}

	// Verify catalog exists
	catalog, err := s.catalogRepo.GetByID(catalogID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "Catalog not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve catalog"))
		}
		return
	}

	// Check if user has access to this catalog
	// Get user with their organization roles to verify access
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Check if catalog is shared (accessible to all users) or if user belongs to catalog's organization
	hasAccess := catalog.IsShared
	if !hasAccess {
		for _, role := range user.UserRoles {
			if role.OrganizationID == catalog.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to access this catalog"))
		return
	}

	// Get templates for this catalog
	templates, err := s.templateRepo.GetByCatalogID(catalogID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve catalog items"))
		return
	}

	// Convert to response format
	itemResponses := make([]CatalogItemResponse, len(templates))
	for i, template := range templates {
		itemResponses[i] = CatalogItemResponse{
			ID:             template.ID.String(),
			Name:           template.Name,
			Description:    template.Description,
			OSType:         template.OSType,
			VMInstanceType: template.VMInstanceType,
			CPUCount:       template.CPUCount,
			MemoryMB:       template.MemoryMB,
			DiskSizeGB:     template.DiskSizeGB,
			CreatedAt:      template.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      template.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := CatalogItemsQueryResponse{
		CatalogItems: itemResponses,
		Total:        len(itemResponses),
	}

	SendSuccess(c, http.StatusOK, response)
}