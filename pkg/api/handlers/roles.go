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

// RoleHandlers contains handlers for role-related CloudAPI endpoints
type RoleHandlers struct {
	roleRepo *repositories.RoleRepository
}

// NewRoleHandlers creates a new RoleHandlers instance
func NewRoleHandlers(roleRepo *repositories.RoleRepository) *RoleHandlers {
	return &RoleHandlers{
		roleRepo: roleRepo,
	}
}

// ListRoles handles GET /cloudapi/1.0.0/roles
func (h *RoleHandlers) ListRoles(c *gin.Context) {
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

	// Get total count of roles
	totalCount, err := h.roleRepo.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count roles"})
		return
	}

	// Get roles
	roles, err := h.roleRepo.List(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve roles"})
		return
	}

	// Create paginated response
	response := types.NewPage(roles, page, limit, int(totalCount))

	c.JSON(http.StatusOK, response)
}

// GetRole handles GET /cloudapi/1.0.0/roles/{id}
func (h *RoleHandlers) GetRole(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID format"})
		return
	}

	// Validate URN type is "role"
	urnType, err := models.GetURNType(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID format"})
		return
	}
	if urnType != "role" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID: expected role URN"})
		return
	}

	// Get role
	role, err := h.roleRepo.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve role"})
		return
	}

	c.JSON(http.StatusOK, role)
}
