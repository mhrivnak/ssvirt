package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// OrgHandlers contains handlers for organization-related CloudAPI endpoints
type OrgHandlers struct {
	orgRepo *repositories.OrganizationRepository
}

// CreateOrgRequest represents the request body for creating an organization
type CreateOrgRequest struct {
	Name                    string `json:"name" binding:"required"`
	DisplayName             string `json:"displayName"`
	Description             string `json:"description"`
	IsEnabled               *bool  `json:"isEnabled"`
	CanManageOrgs           *bool  `json:"canManageOrgs"`
	CanPublish              *bool  `json:"canPublish"`
	MaskedEventTaskUsername string `json:"maskedEventTaskUsername"`
}

// UpdateOrgRequest represents the request body for updating an organization
type UpdateOrgRequest struct {
	Name                    string `json:"name"`
	DisplayName             string `json:"displayName"`
	Description             string `json:"description"`
	IsEnabled               *bool  `json:"isEnabled"`
	CanManageOrgs           *bool  `json:"canManageOrgs"`
	CanPublish              *bool  `json:"canPublish"`
	MaskedEventTaskUsername string `json:"maskedEventTaskUsername"`
}

// NewOrgHandlers creates a new OrgHandlers instance
func NewOrgHandlers(orgRepo *repositories.OrganizationRepository) *OrgHandlers {
	return &OrgHandlers{
		orgRepo: orgRepo,
	}
}

// ListOrgs handles GET /cloudapi/1.0.0/orgs
func (h *OrgHandlers) ListOrgs(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("page_size", "25")
	offsetStr := c.DefaultQuery("page", "1")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100 // Maximum page size
	}

	page, err := strconv.Atoi(offsetStr)
	if err != nil || page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	// Get total count of accessible organizations
	totalCount, err := h.orgRepo.CountAccessibleOrgs(c.Request.Context(), userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count organizations"})
		return
	}

	// Get organizations accessible to the user
	orgs, err := h.orgRepo.ListAccessibleOrgs(c.Request.Context(), userClaims.UserID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organizations"})
		return
	}

	// Create paginated response
	response := types.NewPage(orgs, page, limit, totalCount)

	c.JSON(http.StatusOK, response)
}

// GetOrg handles GET /cloudapi/1.0.0/orgs/{id}
func (h *OrgHandlers) GetOrg(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return
	}

	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}

	// Validate URN type is "org"
	urnType, err := models.GetURNType(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}
	if urnType != "org" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID: expected org URN"})
		return
	}

	// Get organization if user has access
	org, err := h.orgRepo.GetAccessibleOrg(c.Request.Context(), userClaims.UserID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organization"})
		return
	}

	c.JSON(http.StatusOK, org)
}

// CreateOrg handles POST /cloudapi/1.0.0/orgs
func (h *OrgHandlers) CreateOrg(c *gin.Context) {
	var req CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Check if organization name already exists
	existingOrg, err := h.orgRepo.GetByName(req.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing organization name"})
		return
	}
	if existingOrg != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Organization name already exists"})
		return
	}

	// Create organization model
	org := &models.Organization{
		Name:                    req.Name,
		DisplayName:             req.DisplayName,
		Description:             req.Description,
		MaskedEventTaskUsername: req.MaskedEventTaskUsername,
	}

	// Set default display name if not provided
	if org.DisplayName == "" {
		org.DisplayName = org.Name
	}

	// Set enabled flag (default true if not provided)
	if req.IsEnabled != nil {
		org.IsEnabled = *req.IsEnabled
	} else {
		org.IsEnabled = true
	}

	// Set canManageOrgs flag (default false if not provided)
	if req.CanManageOrgs != nil {
		org.CanManageOrgs = *req.CanManageOrgs
	} else {
		org.CanManageOrgs = false
	}

	// Set canPublish flag (default false if not provided)
	if req.CanPublish != nil {
		org.CanPublish = *req.CanPublish
	} else {
		org.CanPublish = false
	}

	// Create organization in database
	if err := h.orgRepo.Create(org); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "Organization with name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	// Get created organization with entity references
	createdOrg, err := h.orgRepo.GetWithEntityRefs(org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve created organization"})
		return
	}

	c.JSON(http.StatusCreated, createdOrg)
}

// UpdateOrg handles PUT /cloudapi/1.0.0/orgs/{id}
func (h *OrgHandlers) UpdateOrg(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}

	// Validate URN type is "org"
	urnType, err := models.GetURNType(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}
	if urnType != "org" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID: expected org URN"})
		return
	}

	// Get existing organization
	org, err := h.orgRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organization"})
		return
	}

	var req UpdateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Update fields if provided
	if req.Name != "" {
		// Check if organization name already exists (excluding current org)
		existingOrg, err := h.orgRepo.GetByName(req.Name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing organization name"})
			return
		}
		if existingOrg != nil && existingOrg.ID != org.ID {
			c.JSON(http.StatusConflict, gin.H{"error": "Organization name already exists"})
			return
		}
		org.Name = req.Name
	}

	if req.DisplayName != "" {
		org.DisplayName = req.DisplayName
	}

	if req.Description != "" {
		org.Description = req.Description
	}

	if req.MaskedEventTaskUsername != "" {
		org.MaskedEventTaskUsername = req.MaskedEventTaskUsername
	}

	if req.IsEnabled != nil {
		org.IsEnabled = *req.IsEnabled
	}

	if req.CanManageOrgs != nil {
		org.CanManageOrgs = *req.CanManageOrgs
	}

	if req.CanPublish != nil {
		org.CanPublish = *req.CanPublish
	}

	// Update organization in database
	if err := h.orgRepo.Update(org); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "Organization with name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization"})
		return
	}

	// Get updated organization with entity references
	updatedOrg, err := h.orgRepo.GetWithEntityRefs(org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated organization"})
		return
	}

	c.JSON(http.StatusOK, updatedOrg)
}

// DeleteOrg handles DELETE /cloudapi/1.0.0/orgs/{id}
func (h *OrgHandlers) DeleteOrg(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}

	// Validate URN type is "org"
	urnType, err := models.GetURNType(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}
	if urnType != "org" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID: expected org URN"})
		return
	}

	// Get existing organization to check if it exists and prevent deletion of Provider org
	org, err := h.orgRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organization"})
		return
	}

	// Prevent deletion of Provider organization
	if org.IsProvider() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the Provider organization"})
		return
	}

	// Delete organization
	if err := h.orgRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
