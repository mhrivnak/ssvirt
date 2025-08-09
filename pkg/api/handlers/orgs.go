package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// OrgHandlers contains handlers for organization-related CloudAPI endpoints
type OrgHandlers struct {
	orgRepo *repositories.OrganizationRepository
}

// NewOrgHandlers creates a new OrgHandlers instance
func NewOrgHandlers(orgRepo *repositories.OrganizationRepository) *OrgHandlers {
	return &OrgHandlers{
		orgRepo: orgRepo,
	}
}

// ListOrgs handles GET /cloudapi/1.0.0/orgs
func (h *OrgHandlers) ListOrgs(c *gin.Context) {
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

	// Get total count of organizations
	totalCount, err := h.orgRepo.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count organizations"})
		return
	}

	// Get organizations with entity references populated
	orgs, err := h.orgRepo.ListWithEntityRefs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organizations"})
		return
	}

	// Create paginated response
	response := types.NewPage(orgs, page, limit, int(totalCount))

	c.JSON(http.StatusOK, response)
}

// GetOrg handles GET /cloudapi/1.0.0/orgs/{id}
func (h *OrgHandlers) GetOrg(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}

	// Get organization with entity references populated
	org, err := h.orgRepo.GetWithEntityRefs(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organization"})
		return
	}

	c.JSON(http.StatusOK, org)
}
