package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username        string `json:"username" binding:"required"`
	FullName        string `json:"fullName" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required,min=6"`
	Description     string `json:"description"`
	OrganizationID  string `json:"organizationId"`
	DeployedVmQuota int    `json:"deployedVmQuota"`
	StoredVmQuota   int    `json:"storedVmQuota"`
	Enabled         *bool  `json:"enabled"`
	ProviderType    string `json:"providerType"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Username        string `json:"username"`
	FullName        string `json:"fullName"`
	Email           string `json:"email"`
	Password        string `json:"password,omitempty"`
	Description     string `json:"description"`
	OrganizationID  string `json:"organizationId"`
	DeployedVmQuota *int   `json:"deployedVmQuota"`
	StoredVmQuota   *int   `json:"storedVmQuota"`
	Enabled         *bool  `json:"enabled"`
	ProviderType    string `json:"providerType"`
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
	response := types.NewPage(users, page, limit, totalCount)

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

// CreateUser handles POST /cloudapi/1.0.0/users
func (h *UserHandlers) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Validate organization ID if provided
	if req.OrganizationID != "" {
		if _, err := models.ParseURN(req.OrganizationID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
			return
		}
		urnType, err := models.GetURNType(req.OrganizationID)
		if err != nil || urnType != "org" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID: expected org URN"})
			return
		}
	}

	// Check if username already exists
	existingUser, err := h.userRepo.GetByUsername(req.Username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing username"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Check if email already exists
	existingUser, err = h.userRepo.GetByEmail(req.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing email"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	// Create user model
	user := &models.User{
		Username:        req.Username,
		FullName:        req.FullName,
		Email:           req.Email,
		Description:     req.Description,
		OrganizationID:  req.OrganizationID,
		DeployedVmQuota: req.DeployedVmQuota,
		StoredVmQuota:   req.StoredVmQuota,
		ProviderType:    req.ProviderType,
	}

	// Set enabled flag (default true if not provided)
	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	} else {
		user.Enabled = true
	}

	// Set default provider type if not provided
	if user.ProviderType == "" {
		user.ProviderType = "LOCAL"
	}

	// Hash password
	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user in database
	if err := h.userRepo.Create(user); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "User with username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Get created user with entity references
	createdUser, err := h.userRepo.GetWithEntityRefs(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve created user"})
		return
	}

	// Remove password from response
	createdUser.Password = ""

	c.JSON(http.StatusCreated, createdUser)
}

// UpdateUser handles PUT /cloudapi/1.0.0/users/{id}
func (h *UserHandlers) UpdateUser(c *gin.Context) {
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

	// Get existing user
	user, err := h.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Validate organization ID if provided
	if req.OrganizationID != "" {
		if _, err := models.ParseURN(req.OrganizationID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID format"})
			return
		}
		urnType, err := models.GetURNType(req.OrganizationID)
		if err != nil || urnType != "org" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID: expected org URN"})
			return
		}
	}

	// Update fields if provided
	if req.Username != "" {
		// Check if username already exists (excluding current user)
		existingUser, err := h.userRepo.GetByUsername(req.Username)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing username"})
			return
		}
		if existingUser != nil && existingUser.ID != user.ID {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		user.Username = req.Username
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}

	if req.Email != "" {
		// Check if email already exists (excluding current user)
		existingUser, err := h.userRepo.GetByEmail(req.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing email"})
			return
		}
		if existingUser != nil && existingUser.ID != user.ID {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		user.Email = req.Email
	}

	user.Description = req.Description
	user.OrganizationID = req.OrganizationID

	if req.DeployedVmQuota != nil {
		user.DeployedVmQuota = *req.DeployedVmQuota
	}

	if req.StoredVmQuota != nil {
		user.StoredVmQuota = *req.StoredVmQuota
	}

	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	}

	if req.ProviderType != "" {
		user.ProviderType = req.ProviderType
	}

	// Update password if provided
	if req.Password != "" {
		if err := user.SetPassword(req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
	}

	// Update user in database
	if err := h.userRepo.Update(user); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "User with username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Get updated user with entity references
	updatedUser, err := h.userRepo.GetWithEntityRefs(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated user"})
		return
	}

	// Remove password from response
	updatedUser.Password = ""

	c.JSON(http.StatusOK, updatedUser)
}

// DeleteUser handles DELETE /cloudapi/1.0.0/users/{id}
func (h *UserHandlers) DeleteUser(c *gin.Context) {
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

	// Check if user exists
	_, err = h.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	// Delete user
	if err := h.userRepo.Delete(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
