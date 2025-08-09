package auth

import (
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

var (
	// ErrInvalidCredentials is returned when login credentials are incorrect
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserNotFound is returned when a requested user does not exist
	ErrUserNotFound = errors.New("user not found")
	// ErrUserExists is returned when attempting to create a user that already exists
	ErrUserExists = errors.New("user already exists")
	// ErrUserInactive is returned when attempting to authenticate with an inactive user account
	ErrUserInactive = errors.New("user account is inactive")
)

// Service provides authentication operations including login, user creation, and token validation
type Service struct {
	userRepo   *repositories.UserRepository
	jwtManager *JWTManager
}

// NewService creates a new authentication service with the provided repositories and JWT manager
func NewService(userRepo *repositories.UserRepository, jwtManager *JWTManager) *Service {
	return &Service{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

// LoginRequest represents the data required for user authentication
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse contains the authentication token and user information returned after successful login
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo represents basic user information returned in authentication responses
type UserInfo struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
}

// CreateUserRequest represents the data required to create a new user account
type CreateUserRequest struct {
	Username  string `json:"username" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Login authenticates a user with username/password and returns a JWT token if successful
func (s *Service) Login(req *LoginRequest) (*LoginResponse, error) {
	if req == nil {
		log.Printf("login request cannot be nil")
		return nil, errors.New("login request cannot be nil")
	}
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("user %s not found", req.Username)
			return nil, ErrInvalidCredentials
		}
		log.Printf("failed to get user %s: %v", req.Username, err)
		return nil, err
	}

	if !user.IsActive {
		log.Printf("user %s is inactive", req.Username)
		return nil, ErrUserInactive
	}

	if !user.CheckPassword(req.Password) {
		log.Printf("invalid password for user %s", req.Username)
		return nil, ErrInvalidCredentials
	}

	token, err := s.jwtManager.Generate(user.ID, user.Username)
	if err != nil {
		log.Printf("failed to generate token for user %s: %v", req.Username, err)
		return nil, err
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(s.jwtManager.tokenDuration),
		User: UserInfo{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
	}, nil
}

// CreateUser creates a new user account with the provided information
func (s *Service) CreateUser(req *CreateUserRequest) (*models.User, error) {
	if req == nil {
		return nil, errors.New("create user request cannot be nil")
	}
	// Check if user already exists
	if _, err := s.userRepo.GetByUsername(req.Username); err == nil {
		return nil, ErrUserExists
	}

	if _, err := s.userRepo.GetByEmail(req.Email); err == nil {
		return nil, ErrUserExists
	}

	user := &models.User{
		Username:  req.Username,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		return nil, err
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUser retrieves a user by their ID
func (s *Service) GetUser(userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(userID)
}

// GetUserWithRoles retrieves a user by their ID including their organization roles
func (s *Service) GetUserWithRoles(userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetWithRoles(userID)
}

// ValidateToken verifies a JWT token and returns the parsed claims if valid
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwtManager.Verify(tokenString)
}
