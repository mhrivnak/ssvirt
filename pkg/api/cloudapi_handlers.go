package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// Users API Handlers - VMware Cloud Director compatible

// listUsersHandler handles GET /cloudapi/1.0.0/users
func (s *Server) listUsersHandler(c *gin.Context) {
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

	// Get users with entity references populated
	users, err := s.userRepo.ListWithEntityRefs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	// Remove password field from response
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, users)
}

// getUserHandler handles GET /cloudapi/1.0.0/users/{id}
func (s *Server) getUserHandler(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get user with entity references populated
	user, err := s.userRepo.GetWithEntityRefs(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	// Remove password field from response
	user.Password = ""

	c.JSON(http.StatusOK, user)
}

// Roles API Handlers - VMware Cloud Director compatible

// listRolesHandler handles GET /cloudapi/1.0.0/roles
func (s *Server) listRolesHandler(c *gin.Context) {
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

	// Get roles
	roles, err := s.roleRepo.List(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// getRoleHandler handles GET /cloudapi/1.0.0/roles/{id}
func (s *Server) getRoleHandler(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID format"})
		return
	}

	// Get role
	role, err := s.roleRepo.GetByID(id)
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

// Organizations API Handlers - VMware Cloud Director compatible

// listOrgsHandler handles GET /cloudapi/1.0.0/orgs
func (s *Server) listOrgsHandler(c *gin.Context) {
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

	// Get organizations with entity references populated
	orgs, err := s.orgRepo.ListWithEntityRefs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organizations"})
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// getOrgHandler handles GET /cloudapi/1.0.0/orgs/{id}
func (s *Server) getOrgHandler(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
		return
	}

	// Get organization with entity references populated
	org, err := s.orgRepo.GetWithEntityRefs(id)
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
