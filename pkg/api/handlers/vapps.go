// Package handlers provides vApp management API handlers for VMware Cloud Director compatibility.
//
// This package implements comprehensive vApp management endpoints that enable authenticated
// users to list, retrieve, and delete vApps (virtual applications) within their accessible
// VDCs. The implementation follows VMware Cloud Director API specifications.
//
// Key Features:
//   - List vApps with pagination and filtering at /cloudapi/1.0.0/vdcs/{vdc_id}/vapps
//   - Retrieve detailed vApp information at /cloudapi/1.0.0/vapps/{vapp_id}
//   - Delete vApps with dependency validation at /cloudapi/1.0.0/vapps/{vapp_id}
//   - Organization-based access control through VDC membership
//   - VM reference management within vApps
//   - Force deletion support for powered-on VMs
//
// Access Control:
// All vApp operations validate user access through organization membership. Users can only
// access vApps within VDCs that belong to their organization. This ensures proper isolation
// and security boundaries in multi-tenant environments.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// VAppHandlers handles vApp API endpoints
type VAppHandlers struct {
	vappRepo *repositories.VAppRepository
	vdcRepo  *repositories.VDCRepository
	vmRepo   *repositories.VMRepository
}

// NewVAppHandlers creates a new VAppHandlers instance
func NewVAppHandlers(vappRepo *repositories.VAppRepository, vdcRepo *repositories.VDCRepository, vmRepo *repositories.VMRepository) *VAppHandlers {
	return &VAppHandlers{
		vappRepo: vappRepo,
		vdcRepo:  vdcRepo,
		vmRepo:   vmRepo,
	}
}

// VAppDetailedResponse represents the detailed response for vApp with VMs
type VAppDetailedResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      string        `json:"status"`
	VDCID       string        `json:"vdcId"`
	TemplateID  string        `json:"templateId,omitempty"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
	NumberOfVMs int           `json:"numberOfVMs"`
	VMs         []VMReference `json:"vms"`
	Href        string        `json:"href"`
}

// VMReference represents a VM reference in vApp response
type VMReference struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Href   string `json:"href"`
}

// ListVApps handles GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps
func (h *VAppHandlers) ListVApps(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Authentication required",
		))
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Invalid authentication token",
		))
		return
	}

	vdcID := c.Param("vdc_id")

	// Validate VDC URN format using centralized validation
	if urnType, err := models.GetURNType(vdcID); err != nil || urnType != "vdc" {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Validate VDC access
	err := h.validateVDCAccess(c.Request.Context(), userClaims.UserID, vdcID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"VDC not found",
			))
		} else {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"VDC access denied",
			))
		}
		return
	}

	// Parse pagination and sorting parameters
	page, pageSize, offset, sortOrder := h.parseVAppPaginationParams(c)
	filter := c.Query("filter")

	// Get vApps in VDC
	vapps, err := h.vappRepo.ListByVDCWithPagination(c.Request.Context(), vdcID, pageSize, offset, filter, sortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve vApps",
		))
		return
	}

	// Get total count
	totalCount, err := h.vappRepo.CountByVDC(c.Request.Context(), vdcID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to count vApps",
		))
		return
	}

	// Convert to response format
	vappResponses := make([]VAppResponse, len(vapps))
	for i, vapp := range vapps {
		vappResponses[i] = h.toVAppResponse(vapp)
	}

	// Calculate pagination info
	pageCount := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	// Create paginated response
	response := types.Page[VAppResponse]{
		ResultTotal: totalCount,
		PageCount:   pageCount,
		Page:        page,
		PageSize:    pageSize,
		Values:      vappResponses,
	}

	c.JSON(http.StatusOK, response)
}

// GetVApp handles GET /cloudapi/1.0.0/vapps/{vapp_id}
func (h *VAppHandlers) GetVApp(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Authentication required",
		))
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Invalid authentication token",
		))
		return
	}

	vappID := c.Param("vapp_id")

	// Validate vApp URN format using centralized validation
	if urnType, err := models.GetURNType(vappID); err != nil || urnType != "vapp" {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid vApp URN format",
		))
		return
	}

	// Validate vApp access
	_, err := h.validateVAppAccess(c.Request.Context(), userClaims.UserID, vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"vApp not found",
			))
		} else {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"vApp access denied",
			))
		}
		return
	}

	// Get vApp with VMs
	vappWithVMs, err := h.vappRepo.GetWithVMsString(c.Request.Context(), vappID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve vApp details",
		))
		return
	}

	// Convert to detailed response format
	response := h.toVAppDetailedResponse(*vappWithVMs)
	c.JSON(http.StatusOK, response)
}

// DeleteVApp handles DELETE /cloudapi/1.0.0/vapps/{vapp_id}
func (h *VAppHandlers) DeleteVApp(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Authentication required",
		))
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Invalid authentication token",
		))
		return
	}

	vappID := c.Param("vapp_id")

	// Validate vApp URN format using centralized validation
	if urnType, err := models.GetURNType(vappID); err != nil || urnType != "vapp" {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid vApp URN format",
		))
		return
	}

	// Parse force parameter
	force := c.Query("force") == "true"

	// Validate vApp access
	_, err := h.validateVAppAccess(c.Request.Context(), userClaims.UserID, vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"vApp not found",
			))
		} else {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"vApp access denied",
			))
		}
		return
	}

	// Delete vApp with validation
	err = h.vappRepo.DeleteWithValidation(c.Request.Context(), vappID, force)
	if err != nil {
		if errors.Is(err, repositories.ErrVAppHasRunningVMs) {
			c.JSON(http.StatusBadRequest, NewAPIError(
				http.StatusBadRequest,
				"Bad Request",
				"vApp contains running VMs",
			))
		} else {
			c.JSON(http.StatusInternalServerError, NewAPIError(
				http.StatusInternalServerError,
				"Internal Server Error",
				"Failed to delete vApp",
			))
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// validateVDCAccess validates that a user has access to a VDC
func (h *VAppHandlers) validateVDCAccess(ctx context.Context, userID, vdcID string) error {
	_, err := h.vdcRepo.GetAccessibleVDC(ctx, userID, vdcID)
	return err
}

// validateVAppAccess validates that a user has access to a vApp through VDC organization membership
func (h *VAppHandlers) validateVAppAccess(ctx context.Context, userID, vappID string) (*models.VApp, error) {
	vapp, err := h.vappRepo.GetWithVDC(ctx, vappID)
	if err != nil {
		return nil, err
	}

	// Check if user has access to the VDC containing this vApp
	err = h.validateVDCAccess(ctx, userID, vapp.VDCID)
	if err != nil {
		return nil, fmt.Errorf("VDC access denied: %w", err)
	}

	return vapp, nil
}

// toVAppResponse converts a VApp model to VCD-compliant response format
func (h *VAppHandlers) toVAppResponse(vapp models.VApp) VAppResponse {
	templateID := ""
	if vapp.TemplateID != nil {
		templateID = *vapp.TemplateID
	}

	return VAppResponse{
		ID:          vapp.ID,
		Name:        vapp.Name,
		Description: vapp.Description,
		Status:      vapp.Status,
		VDCID:       vapp.VDCID,
		TemplateID:  templateID,
		CreatedAt:   vapp.CreatedAt.Format("2006-01-02T15:04:05Z"),
		NumberOfVMs: len(vapp.VMs), // Count actual VMs
		Href:        fmt.Sprintf("/cloudapi/1.0.0/vapps/%s", vapp.ID),
	}
}

// toVAppDetailedResponse converts a VApp model to detailed VCD-compliant response format
func (h *VAppHandlers) toVAppDetailedResponse(vapp models.VApp) VAppDetailedResponse {
	templateID := ""
	if vapp.TemplateID != nil {
		templateID = *vapp.TemplateID
	}

	// Convert VMs to references
	vmRefs := make([]VMReference, len(vapp.VMs))
	for i, vm := range vapp.VMs {
		vmRefs[i] = VMReference{
			ID:     vm.ID,
			Name:   vm.Name,
			Status: vm.Status,
			Href:   fmt.Sprintf("/cloudapi/1.0.0/vms/%s", vm.ID),
		}
	}

	return VAppDetailedResponse{
		ID:          vapp.ID,
		Name:        vapp.Name,
		Description: vapp.Description,
		Status:      vapp.Status,
		VDCID:       vapp.VDCID,
		TemplateID:  templateID,
		CreatedAt:   vapp.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   vapp.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		NumberOfVMs: len(vapp.VMs),
		VMs:         vmRefs,
		Href:        fmt.Sprintf("/cloudapi/1.0.0/vapps/%s", vapp.ID),
	}
}

// parseVAppPaginationParams extracts and validates pagination and sorting parameters from the request
func (h *VAppHandlers) parseVAppPaginationParams(c *gin.Context) (page, pageSize, offset int, sortOrder string) {
	// Default values
	page = 1
	pageSize = 25
	offset = 0
	sortOrder = "created_at DESC, id DESC" // Default sort order

	// Parse page parameter
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Parse pageSize parameter
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
			// Limit maximum page size
			if pageSize > 100 {
				pageSize = 100
			}
		}
	}

	// Parse offset parameter (takes precedence over page-based pagination)
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	} else {
		// Calculate offset from page if offset is not provided
		offset = (page - 1) * pageSize
	}

	// Parse sorting parameters
	sortAsc := c.Query("sortAsc")
	sortDesc := c.Query("sortDesc")

	if sortAsc != "" {
		// Validate sort field to prevent SQL injection
		if h.isValidSortField(sortAsc) {
			sortOrder = sortAsc + " ASC"
		}
	} else if sortDesc != "" {
		// Validate sort field to prevent SQL injection
		if h.isValidSortField(sortDesc) {
			sortOrder = sortDesc + " DESC"
		}
	}

	return page, pageSize, offset, sortOrder
}

// isValidSortField validates that the sort field is allowed
func (h *VAppHandlers) isValidSortField(field string) bool {
	allowedFields := map[string]bool{
		"name":       true,
		"created_at": true,
		"updated_at": true,
		"status":     true,
	}
	return allowedFields[field]
}
