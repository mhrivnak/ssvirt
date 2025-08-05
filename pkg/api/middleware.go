package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// corsMiddleware handles Cross-Origin Resource Sharing (CORS)
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// errorHandlerMiddleware provides consistent error handling
func (s *Server) errorHandlerMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Internal Server Error",
				"message": "An unexpected error occurred",
				"details": err,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Internal Server Error",
				"message": "An unexpected error occurred",
			})
		}
		c.Abort()
	})
}

// APIError represents a structured API error response
type APIError struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// NewAPIError creates a new API error response
func NewAPIError(code int, error string, message string, details ...string) *APIError {
	apiErr := &APIError{
		Code:    code,
		Error:   error,
		Message: message,
	}
	if len(details) > 0 {
		apiErr.Details = details[0]
	}
	return apiErr
}

// SendError sends a structured error response
func SendError(c *gin.Context, apiErr *APIError) {
	c.JSON(apiErr.Code, apiErr)
}

// SendSuccess sends a structured success response
func SendSuccess(c *gin.Context, code int, data interface{}) {
	response := gin.H{
		"success": true,
		"data":    data,
	}
	c.JSON(code, response)
}