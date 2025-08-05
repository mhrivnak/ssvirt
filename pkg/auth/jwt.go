package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken is returned when a JWT token is malformed or has invalid signature
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when a JWT token has passed its expiration time
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the JWT claims structure for SSVirt authentication
type Claims struct {
	UserID         uuid.UUID  `json:"user_id"`
	Username       string     `json:"username"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	Role           *string    `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and verification for authentication
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager with the specified secret key and token duration
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:     secretKey,
		tokenDuration: tokenDuration,
	}
}

// Generate creates a new JWT token for the specified user without organization context
func (manager *JWTManager) Generate(userID uuid.UUID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// GenerateWithRole creates a new JWT token for the specified user with organization and role context
func (manager *JWTManager) GenerateWithRole(userID uuid.UUID, username string, organizationID uuid.UUID, role string) (string, error) {
	claims := &Claims{
		UserID:         userID,
		Username:       username,
		OrganizationID: &organizationID,
		Role:           &role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// Verify validates a JWT token and returns the parsed claims if valid
func (manager *JWTManager) Verify(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}