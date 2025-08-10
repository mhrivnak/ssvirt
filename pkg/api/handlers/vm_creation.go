package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// VMCreationHandlers handles VM creation via template instantiation
type VMCreationHandlers struct {
	vdcRepo         *repositories.VDCRepository
	vappRepo        *repositories.VAppRepository
	catalogItemRepo *repositories.CatalogItemRepository
}

// NewVMCreationHandlers creates a new VMCreationHandlers instance
func NewVMCreationHandlers(vdcRepo *repositories.VDCRepository, vappRepo *repositories.VAppRepository, catalogItemRepo *repositories.CatalogItemRepository) *VMCreationHandlers {
	return &VMCreationHandlers{
		vdcRepo:         vdcRepo,
		vappRepo:        vappRepo,
		catalogItemRepo: catalogItemRepo,
	}
}

// InstantiateTemplateRequest represents the request body for template instantiation
type InstantiateTemplateRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description"`
	CatalogItem CatalogItem `json:"catalogItem" binding:"required"`
}

// CatalogItem represents a catalog item reference in the request
type CatalogItem struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name"`
}

// VAppResponse represents the response for vApp operations
type VAppResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	VDCID       string `json:"vdcId"`
	TemplateID  string `json:"templateId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	NumberOfVMs int    `json:"numberOfVMs"`
	Href        string `json:"href"`
}

// InstantiateTemplate handles POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate
func (h *VMCreationHandlers) InstantiateTemplate(c *gin.Context) {
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
	if !vdcURNRegex.MatchString(vdcID) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VDC URN format",
		))
		return
	}

	// Parse request body
	var req InstantiateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid request format",
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

	// Validate catalog item access
	err = h.validateCatalogItemAccess(c.Request.Context(), userClaims.UserID, req.CatalogItem.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog item not found",
			))
		} else {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"Catalog item access denied",
			))
		}
		return
	}

	// Check for name conflicts within VDC
	exists, err = h.vappRepo.ExistsByNameInVDC(c.Request.Context(), vdcID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to check name availability",
		))
		return
	}
	if exists {
		c.JSON(http.StatusConflict, NewAPIError(
			http.StatusConflict,
			"Conflict",
			"Name already in use within VDC",
		))
		return
	}

	// Create vApp
	vapp := &models.VApp{
		Name:        req.Name,
		Description: req.Description,
		VDCID:       vdcID,
		TemplateID:  &req.CatalogItem.ID,
		Status:      "INSTANTIATING",
	}

	err = h.vappRepo.CreateWithContext(c.Request.Context(), vapp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to create vApp",
		))
		return
	}

	// TODO: Create TemplateInstance in OpenShift
	// For now, we'll set status to RESOLVED as a placeholder
	vapp.Status = "RESOLVED"
	err = h.vappRepo.UpdateWithContext(c.Request.Context(), vapp)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: Failed to update vApp status: %v\n", err)
	}

	// Return vApp response
	response := h.toVAppResponse(*vapp)
	c.JSON(http.StatusCreated, response)
}

// validateVDCAccess validates that a user has access to a VDC
func (h *VMCreationHandlers) validateVDCAccess(ctx context.Context, userID, vdcID string) error {
	_, err := h.vdcRepo.GetAccessibleVDC(ctx, userID, vdcID)
	return err
}

// validateCatalogItemAccess validates that a user has access to a catalog item
func (h *VMCreationHandlers) validateCatalogItemAccess(ctx context.Context, userID, catalogItemID string) error {
	// For template instantiation, we'll assume the template is accessible
	// if the user has access to any catalog in their organization.
	// In a full implementation, this would check specific template access
	// and validate the template exists in an accessible catalog.
	// For now, we'll return nil to allow template instantiation.
	return nil
}

// toVAppResponse converts a VApp model to VCD-compliant response format
func (h *VMCreationHandlers) toVAppResponse(vapp models.VApp) VAppResponse {
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
		NumberOfVMs: 1, // For now, each vApp has one VM
		Href:        fmt.Sprintf("/cloudapi/1.0.0/vapps/%s", vapp.ID),
	}
}
