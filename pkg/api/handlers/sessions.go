package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// APIError represents a structured API error response
type APIError struct {
	Code    int    `json:"code"`
	Type    string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Type, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewAPIError creates a new API error response
func NewAPIError(code int, errorType string, message string, details ...string) *APIError {
	apiErr := &APIError{
		Code:    code,
		Type:    errorType,
		Message: message,
	}
	if len(details) > 0 {
		apiErr.Details = details[0]
	}
	return apiErr
}

type SessionHandlers struct {
	userRepo   *repositories.UserRepository
	authSvc    *auth.Service
	jwtManager *auth.JWTManager
	config     *config.Config
}

func NewSessionHandlers(userRepo *repositories.UserRepository, authSvc *auth.Service, jwtManager *auth.JWTManager, config *config.Config) *SessionHandlers {
	return &SessionHandlers{
		userRepo:   userRepo,
		authSvc:    authSvc,
		jwtManager: jwtManager,
		config:     config,
	}
}

// CreateSession handles POST /cloudapi/1.0.0/sessions
func (h *SessionHandlers) CreateSession(c *gin.Context) {
	// Parse Basic Authentication
	username, password, err := h.parseBasicAuth(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, NewAPIError(401, "Unauthorized", "Invalid or missing Authorization header"))
		return
	}

	// Authenticate user
	user, err := h.userRepo.GetByUsername(username)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, NewAPIError(401, "Unauthorized", "Invalid username or password"))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Authentication error"))
		return
	}

	// Check password
	if !user.CheckPassword(password) {
		c.JSON(http.StatusUnauthorized, NewAPIError(401, "Unauthorized", "Invalid username or password"))
		return
	}

	// Check if user is enabled
	if !user.Enabled {
		c.JSON(http.StatusForbidden, NewAPIError(403, "Forbidden", "User account is inactive"))
		return
	}

	// Get user with roles and organization
	userWithRoles, err := h.userRepo.GetWithRoles(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Failed to load user data"))
		return
	}

	// Build session response
	session, err := h.buildSessionResponse(userWithRoles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Failed to create session"))
		return
	}

	// Create JWT token with session ID
	token, err := h.jwtManager.GenerateWithSessionID(user.ID, user.Username, session.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Failed to generate session token"))
		return
	}

	// Set Authorization header for subsequent requests
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, session)
}

// GetCurrentSession handles GET /cloudapi/1.0.0/sessions/{sessionId}
func (h *SessionHandlers) GetCurrentSession(c *gin.Context) {
	sessionId := c.Param("sessionId")
	
	// Validate session ownership
	if !h.validateSessionOwnership(c, sessionId) {
		return
	}

	// Get current user from context (set by JWT middleware)
	userID := c.GetString(auth.UserContextKey)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, NewAPIError(401, "Unauthorized", "Invalid session"))
		return
	}

	// Get user with roles
	user, err := h.userRepo.GetWithRoles(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Failed to load user data"))
		return
	}

	// Build session response
	session, err := h.buildSessionResponse(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(500, "Internal Server Error", "Failed to build session"))
		return
	}

	// Use the session ID from the URL
	session.ID = sessionId
	c.JSON(http.StatusOK, session)
}

// DeleteSession handles DELETE /cloudapi/1.0.0/sessions/{sessionId}
func (h *SessionHandlers) DeleteSession(c *gin.Context) {
	sessionId := c.Param("sessionId")
	
	// Validate session ownership
	if !h.validateSessionOwnership(c, sessionId) {
		return
	}

	// In a stateless JWT implementation, we don't need to explicitly delete anything
	// The session becomes invalid when the JWT expires
	// For now, we just return success
	c.Status(http.StatusNoContent)
}

// parseBasicAuth extracts username and password from Basic Authentication header
func (h *SessionHandlers) parseBasicAuth(c *gin.Context) (string, string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", "", NewAPIError(401, "Unauthorized", "Authorization header required")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return "", "", NewAPIError(401, "Unauthorized", "Basic authentication required")
	}

	// Decode base64 credentials
	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", NewAPIError(401, "Unauthorized", "Invalid base64 encoding")
	}

	// Split username:password
	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return "", "", NewAPIError(401, "Unauthorized", "Invalid credentials format")
	}

	return parts[0], parts[1], nil
}

// validateSessionOwnership ensures users can only access their own sessions
func (h *SessionHandlers) validateSessionOwnership(c *gin.Context, sessionId string) bool {
	// Get session ID from JWT token
	tokenSessionID := c.GetString(auth.SessionContextKey)
	if tokenSessionID == "" {
		c.JSON(http.StatusUnauthorized, NewAPIError(401, "Unauthorized", "Invalid session token"))
		return false
	}

	// Compare session IDs
	if tokenSessionID != sessionId {
		c.JSON(http.StatusForbidden, NewAPIError(403, "Forbidden", "Cannot access another user's session"))
		return false
	}

	return true
}

// buildSessionResponse creates a VCD-compliant session response
func (h *SessionHandlers) buildSessionResponse(user *models.User) (*models.Session, error) {
	session := &models.Session{
		ID:                        models.GenerateSessionURN(),
		SessionIdleTimeoutMinutes: h.config.Session.IdleTimeoutMinutes,
		Location:                  h.config.Session.Location,
	}

	// Build site reference
	session.Site = models.EntityRef{
		Name: h.config.Session.Site.Name,
		ID:   h.config.Session.Site.ID,
	}

	// Build user reference
	session.User = models.EntityRef{
		Name: user.Username,
		ID:   user.ID,
	}

	// Build organization references
	if user.Organization != nil {
		session.Org = models.EntityRef{
			Name: user.Organization.Name,
			ID:   user.Organization.ID,
		}
		// Operating org is the same as primary org for now
		session.OperatingOrg = session.Org
	}

	// Build roles arrays
	session.Roles = make([]string, 0)
	session.RoleRefs = make([]models.EntityRef, 0)
	
	for _, role := range user.Roles {
		session.Roles = append(session.Roles, role.Name)
		session.RoleRefs = append(session.RoleRefs, models.EntityRef{
			Name: role.Name,
			ID:   role.ID,
		})
	}

	return session, nil
}