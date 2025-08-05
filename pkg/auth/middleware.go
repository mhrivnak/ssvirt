package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// AuthorizationHeader is the HTTP header name for authorization tokens
	AuthorizationHeader = "Authorization"
	// BearerPrefix is the expected prefix for Bearer tokens in the Authorization header
	BearerPrefix        = "Bearer "
	// UserContextKey is the Gin context key for storing user ID
	UserContextKey      = "user"
	// ClaimsContextKey is the Gin context key for storing JWT claims
	ClaimsContextKey    = "claims"
)

// JWTMiddleware creates a Gin middleware that requires valid JWT authentication
func JWTMiddleware(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
		claims, err := jwtManager.Verify(tokenString)
		if err != nil {
			var message string
			switch err {
			case ErrExpiredToken:
				message = "Token has expired"
			case ErrInvalidToken:
				message = "Invalid token"
			default:
				message = "Token verification failed"
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": message})
			c.Abort()
			return
		}

		c.Set(ClaimsContextKey, claims)
		c.Set(UserContextKey, claims.UserID)
		c.Next()
	}
}

// OptionalJWTMiddleware creates a Gin middleware that extracts JWT claims if present but doesn't require authentication
func OptionalJWTMiddleware(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader != "" && strings.HasPrefix(authHeader, BearerPrefix) {
			tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
			if claims, err := jwtManager.Verify(tokenString); err == nil {
				c.Set(ClaimsContextKey, claims)
				c.Set(UserContextKey, claims.UserID)
			}
		}
		c.Next()
	}
}

// GetClaims extracts JWT claims from the Gin context if they exist
func GetClaims(c *gin.Context) (*Claims, bool) {
	claims, exists := c.Get(ClaimsContextKey)
	if !exists {
		return nil, false
	}
	userClaims, ok := claims.(*Claims)
	return userClaims, ok
}

// RequireRole creates a middleware that requires the authenticated user to have one of the specified roles
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := GetClaims(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		if claims.Role == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Role required"})
			c.Abort()
			return
		}

		for _, role := range allowedRoles {
			if *claims.Role == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}