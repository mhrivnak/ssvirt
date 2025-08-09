package handlers

import (
	"fmt"
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

type VDCHandlers struct {
	vdcRepo *repositories.VDCRepository
	orgRepo *repositories.OrganizationRepository
}

func NewVDCHandlers(vdcRepo *repositories.VDCRepository, orgRepo *repositories.OrganizationRepository) *VDCHandlers {
	return &VDCHandlers{
		vdcRepo: vdcRepo,
		orgRepo: orgRepo,
	}
}

// VDCCreateRequest represents the request body for creating a VDC
type VDCCreateRequest struct {
	Name            string                 `json:"name" binding:"required"`
	Description     string                 `json:"description"`
	AllocationModel models.AllocationModel `json:"allocationModel" binding:"required"`
	ComputeCapacity models.ComputeCapacity `json:"computeCapacity"`
	ProviderVdc     models.ProviderVdc     `json:"providerVdc"`
	NicQuota        int                    `json:"nicQuota"`
	NetworkQuota    int                    `json:"networkQuota"`
	IsThinProvision bool                   `json:"isThinProvision"`
	IsEnabled       bool                   `json:"isEnabled"`
}

// VDCUpdateRequest represents the request body for updating a VDC
type VDCUpdateRequest struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	AllocationModel models.AllocationModel  `json:"allocationModel"`
	ComputeCapacity *models.ComputeCapacity `json:"computeCapacity,omitempty"`
	ProviderVdc     *models.ProviderVdc     `json:"providerVdc,omitempty"`
	NicQuota        *int                    `json:"nicQuota,omitempty"`
	NetworkQuota    *int                    `json:"networkQuota,omitempty"`
	IsThinProvision *bool                   `json:"isThinProvision,omitempty"`
	IsEnabled       *bool                   `json:"isEnabled,omitempty"`
}

// VDCResponse represents the VCD-compliant VDC response
type VDCResponse struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	AllocationModel    models.AllocationModel    `json:"allocationModel"`
	ComputeCapacity    models.ComputeCapacity    `json:"computeCapacity"`
	ProviderVdc        models.ProviderVdc        `json:"providerVdc"`
	NicQuota           int                       `json:"nicQuota"`
	NetworkQuota       int                       `json:"networkQuota"`
	VdcStorageProfiles models.VdcStorageProfiles `json:"vdcStorageProfiles"`
	IsThinProvision    bool                      `json:"isThinProvision"`
	IsEnabled          bool                      `json:"isEnabled"`
}

// ListVDCs handles GET /api/admin/org/{orgId}/vdcs
func (h *VDCHandlers) ListVDCs(c *gin.Context) {
	orgURN := c.Param("orgId")

	// Validate organization URN format
	if !strings.HasPrefix(orgURN, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
			"Organization ID must be a valid URN with prefix 'urn:vcloud:org:'",
		))
		return
	}

	// Verify organization exists
	_, err := h.orgRepo.GetByID(orgURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Organization not found",
				fmt.Sprintf("Organization with ID '%s' does not exist", orgURN),
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to query organization",
			err.Error(),
		))
		return
	}

	// Parse pagination parameters
	page := 1
	pageSize := 25

	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	if sizeParam := c.Query("page_size"); sizeParam != "" {
		if s, err := strconv.Atoi(sizeParam); err == nil && s > 0 && s <= 100 {
			pageSize = s
		}
	}

	offset := (page - 1) * pageSize

	// Get VDCs with pagination
	vdcs, err := h.vdcRepo.ListByOrgWithPagination(orgURN, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve VDCs",
			err.Error(),
		))
		return
	}

	// Get total count
	totalCount, err := h.vdcRepo.CountByOrganization(orgURN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to count VDCs",
			err.Error(),
		))
		return
	}

	// Convert to response format
	vdcResponses := make([]VDCResponse, len(vdcs))
	for i, vdc := range vdcs {
		vdcResponses[i] = h.toVDCResponse(vdc)
	}

	// Build paginated response
	response := types.NewPage(vdcResponses, page, pageSize, totalCount)

	c.JSON(http.StatusOK, response)
}

// GetVDC handles GET /api/admin/org/{orgId}/vdcs/{vdcId}
func (h *VDCHandlers) GetVDC(c *gin.Context) {
	orgURN := c.Param("orgId")
	vdcURN := c.Param("vdcId")

	// Validate URN formats
	if !strings.HasPrefix(orgURN, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
		))
		return
	}

	if !strings.HasPrefix(vdcURN, models.URNPrefixVDC) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Get VDC
	vdc, err := h.vdcRepo.GetByOrgAndVDCURN(orgURN, vdcURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"VDC not found",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve VDC",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusOK, h.toVDCResponse(*vdc))
}

// CreateVDC handles POST /api/admin/org/{orgId}/vdcs
func (h *VDCHandlers) CreateVDC(c *gin.Context) {
	orgURN := c.Param("orgId")

	// Validate organization URN format
	if !strings.HasPrefix(orgURN, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
		))
		return
	}

	// Verify organization exists
	_, err := h.orgRepo.GetByID(orgURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Organization not found",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to query organization",
			err.Error(),
		))
		return
	}

	var req VDCCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid request body",
			err.Error(),
		))
		return
	}

	// Validate allocation model
	if !req.AllocationModel.Valid() {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid allocation model",
			"Allocation model must be one of: PayAsYouGo, AllocationPool, ReservationPool, Flex",
		))
		return
	}

	// Set defaults for optional fields
	if req.NicQuota == 0 {
		req.NicQuota = 100
	}
	if req.NetworkQuota == 0 {
		req.NetworkQuota = 50
	}

	// Create VDC model
	vdc := &models.VDC{
		Name:            req.Name,
		Description:     req.Description,
		OrganizationID:  orgURN,
		AllocationModel: req.AllocationModel,
		NicQuota:        req.NicQuota,
		NetworkQuota:    req.NetworkQuota,
		IsThinProvision: req.IsThinProvision,
		IsEnabled:       req.IsEnabled,
	}

	// Set compute capacity
	vdc.SetComputeCapacity(req.ComputeCapacity)

	// Set provider VDC reference
	vdc.SetProviderVdc(req.ProviderVdc)

	// Create VDC
	if err := h.vdcRepo.Create(vdc); err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to create VDC",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusCreated, h.toVDCResponse(*vdc))
}

// UpdateVDC handles PUT /api/admin/org/{orgId}/vdcs/{vdcId}
func (h *VDCHandlers) UpdateVDC(c *gin.Context) {
	orgURN := c.Param("orgId")
	vdcURN := c.Param("vdcId")

	// Validate URN formats
	if !strings.HasPrefix(orgURN, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
		))
		return
	}

	if !strings.HasPrefix(vdcURN, models.URNPrefixVDC) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Get existing VDC
	vdc, err := h.vdcRepo.GetByOrgAndVDCURN(orgURN, vdcURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"VDC not found",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve VDC",
			err.Error(),
		))
		return
	}

	var req VDCUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid request body",
			err.Error(),
		))
		return
	}

	// Update fields if provided
	if req.Name != "" {
		vdc.Name = req.Name
	}
	if req.Description != "" {
		vdc.Description = req.Description
	}
	if req.AllocationModel != "" {
		if !req.AllocationModel.Valid() {
			c.JSON(http.StatusBadRequest, NewAPIError(
				http.StatusBadRequest,
				"Bad Request",
				"Invalid allocation model",
			))
			return
		}
		vdc.AllocationModel = req.AllocationModel
	}
	if req.ComputeCapacity != nil {
		vdc.SetComputeCapacity(*req.ComputeCapacity)
	}
	if req.ProviderVdc != nil {
		vdc.SetProviderVdc(*req.ProviderVdc)
	}
	if req.NicQuota != nil {
		vdc.NicQuota = *req.NicQuota
	}
	if req.NetworkQuota != nil {
		vdc.NetworkQuota = *req.NetworkQuota
	}
	if req.IsThinProvision != nil {
		vdc.IsThinProvision = *req.IsThinProvision
	}
	if req.IsEnabled != nil {
		vdc.IsEnabled = *req.IsEnabled
	}

	// Update VDC
	if err := h.vdcRepo.Update(vdc); err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to update VDC",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusOK, h.toVDCResponse(*vdc))
}

// DeleteVDC handles DELETE /api/admin/org/{orgId}/vdcs/{vdcId}
func (h *VDCHandlers) DeleteVDC(c *gin.Context) {
	orgURN := c.Param("orgId")
	vdcURN := c.Param("vdcId")

	// Validate URN formats
	if !strings.HasPrefix(orgURN, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
		))
		return
	}

	if !strings.HasPrefix(vdcURN, models.URNPrefixVDC) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Verify VDC exists and belongs to organization
	vdc, err := h.vdcRepo.GetByOrgAndVDCURN(orgURN, vdcURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"VDC not found",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve VDC",
			err.Error(),
		))
		return
	}

	// Delete VDC with validation (checks for dependent vApps)
	if err := h.vdcRepo.DeleteWithValidation(vdc.ID); err != nil {
		if strings.Contains(err.Error(), "dependent vApps") {
			c.JSON(http.StatusConflict, NewAPIError(
				http.StatusConflict,
				"Conflict",
				"Cannot delete VDC with dependent resources",
				"VDC contains vApps that must be deleted first",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to delete VDC",
			err.Error(),
		))
		return
	}

	c.Status(http.StatusNoContent)
}

// toVDCResponse converts a VDC model to VCD-compliant response format
func (h *VDCHandlers) toVDCResponse(vdc models.VDC) VDCResponse {
	return VDCResponse{
		ID:                 vdc.ID,
		Name:               vdc.Name,
		Description:        vdc.Description,
		AllocationModel:    vdc.AllocationModel,
		ComputeCapacity:    vdc.ComputeCapacity(),
		ProviderVdc:        vdc.ProviderVdc(),
		NicQuota:           vdc.NicQuota,
		NetworkQuota:       vdc.NetworkQuota,
		VdcStorageProfiles: vdc.VdcStorageProfiles(),
		IsThinProvision:    vdc.IsThinProvision,
		IsEnabled:          vdc.IsEnabled,
	}
}

// RequireSystemAdmin middleware ensures only System Administrators can access VDC endpoints
func RequireSystemAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(auth.ClaimsContextKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, NewAPIError(
				http.StatusUnauthorized,
				"Unauthorized",
				"Authentication required",
			))
			c.Abort()
			return
		}

		userClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, NewAPIError(
				http.StatusUnauthorized,
				"Unauthorized",
				"Invalid authentication token",
			))
			c.Abort()
			return
		}

		// Check if user has System Administrator role
		if userClaims.Role == nil || *userClaims.Role != models.RoleSystemAdmin {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"System Administrator role required",
				"VDC management requires System Administrator privileges",
			))
			c.Abort()
			return
		}

		c.Next()
	}
}
