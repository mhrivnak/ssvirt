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

// UserHandlers contains handlers for user-related CloudAPI endpoints
type UserHandlers struct {
	userRepo *repositories.UserRepository
}

// NewUserHandlers creates a new UserHandlers instance
func NewUserHandlers(userRepo *repositories.UserRepository) *UserHandlers {
	return &UserHandlers{
		userRepo: userRepo,
	}
}

// ListUsers handles GET /cloudapi/1.0.0/users
func (h *UserHandlers) ListUsers(c *gin.Context) {
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

	// Get total count of users
	totalCount, err := h.userRepo.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count users"})
		return
	}

	// Get users with entity references populated
	users, err := h.userRepo.ListWithEntityRefs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	// Remove password field from response
	for i := range users {
		users[i].Password = ""
	}

	// Create paginated response
	response := types.NewPage(users, page, limit, int(totalCount))

	c.JSON(http.StatusOK, response)
}

// GetUser handles GET /cloudapi/1.0.0/users/{id}
func (h *UserHandlers) GetUser(c *gin.Context) {
	id := c.Param("id")

	// Validate URN format
	if _, err := models.ParseURN(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Validate URN type is "user"
	urnType, err := models.GetURNType(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}
	if urnType != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID: expected user URN"})
		return
	}

	// Get user with entity references populated
	user, err := h.userRepo.GetWithEntityRefs(id)
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
