package handlers

import (
	"errors"
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

// VDCPublicHandlers handles public (non-admin) VDC API endpoints
type VDCPublicHandlers struct {
	vdcRepo *repositories.VDCRepository
}

// NewVDCPublicHandlers creates a new VDCPublicHandlers instance
func NewVDCPublicHandlers(vdcRepo *repositories.VDCRepository) *VDCPublicHandlers {
	return &VDCPublicHandlers{
		vdcRepo: vdcRepo,
	}
}

// isValidVDCURN validates that a VDC URN matches the expected format
func isValidVDCURN(urn string) bool {
	urnType, err := models.GetURNType(urn)
	return err == nil && urnType == "vdc"
}

// ListVDCs handles GET /cloudapi/1.0.0/vdcs
func (h *VDCPublicHandlers) ListVDCs(c *gin.Context) {
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

	// Parse pagination parameters
	page, pageSize := parseVDCPaginationParams(c)

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get VDCs accessible to the user
	vdcs, err := h.vdcRepo.ListAccessibleVDCs(c.Request.Context(), userClaims.UserID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve VDCs",
		))
		return
	}

	// Get total count of accessible VDCs
	totalCount, err := h.vdcRepo.CountAccessibleVDCs(c.Request.Context(), userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to count VDCs",
		))
		return
	}

	// Convert to response format
	vdcResponses := make([]VDCResponse, len(vdcs))
	for i, vdc := range vdcs {
		vdcResponses[i] = toVDCResponse(vdc)
	}

	// Calculate pagination info
	pageCount := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	// Create paginated response
	response := types.Page[VDCResponse]{
		ResultTotal: totalCount,
		PageCount:   pageCount,
		Page:        page,
		PageSize:    pageSize,
		Values:      vdcResponses,
	}

	c.JSON(http.StatusOK, response)
}

// GetVDC handles GET /cloudapi/1.0.0/vdcs/{vdc_id}
func (h *VDCPublicHandlers) GetVDC(c *gin.Context) {
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

	// Validate VDC URN format
	if !isValidVDCURN(vdcID) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Get VDC if user has access
	vdc, err := h.vdcRepo.GetAccessibleVDC(c.Request.Context(), userClaims.UserID, vdcID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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
		))
		return
	}

	c.JSON(http.StatusOK, toVDCResponse(*vdc))
}

// toVDCResponse converts a VDC model to VCD-compliant response format
// This function is shared with the admin handlers for consistency
func toVDCResponse(vdc models.VDC) VDCResponse {
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

// parseVDCPaginationParams extracts and validates pagination parameters from the request
// Specific to VDC endpoints to avoid conflicts with other handlers
func parseVDCPaginationParams(c *gin.Context) (page, pageSize int) {
	// Default values
	page = 1
	pageSize = 25

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

	return page, pageSize
}
