package unit

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

func setupTestAuthDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.User{}, &models.Organization{}, &models.UserRole{})
	require.NoError(t, err)

	return db
}

func TestJWTManager(t *testing.T) {
	secretKey := "test-secret-key"
	tokenDuration := time.Hour

	jwtManager := auth.NewJWTManager(secretKey, tokenDuration)

	userID := uuid.New()
	username := "testuser"

	t.Run("Generate and verify token", func(t *testing.T) {
		token, err := jwtManager.Generate(userID, username)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := jwtManager.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, username, claims.Username)
		assert.Nil(t, claims.OrganizationID)
		assert.Nil(t, claims.Role)
	})

	t.Run("Generate and verify token with role", func(t *testing.T) {
		orgID := uuid.New()
		role := "OrgAdmin"

		token, err := jwtManager.GenerateWithRole(userID, username, orgID, role)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := jwtManager.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, username, claims.Username)
		require.NotNil(t, claims.OrganizationID)
		assert.Equal(t, orgID, *claims.OrganizationID)
		require.NotNil(t, claims.Role)
		assert.Equal(t, role, *claims.Role)
	})

	t.Run("Verify invalid token", func(t *testing.T) {
		_, err := jwtManager.Verify("invalid-token")
		assert.Error(t, err)
	})

	t.Run("Verify expired token", func(t *testing.T) {
		shortDurationManager := auth.NewJWTManager(secretKey, time.Nanosecond)
		token, err := shortDurationManager.Generate(userID, username)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)

		_, err = shortDurationManager.Verify(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

func TestUserModel(t *testing.T) {
	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
	}

	t.Run("Set and check password", func(t *testing.T) {
		password := "testpassword123"
		
		err := user.SetPassword(password)
		require.NoError(t, err)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEqual(t, password, user.PasswordHash)

		assert.True(t, user.CheckPassword(password))
		assert.False(t, user.CheckPassword("wrongpassword"))
	})
}

func TestUserRepository(t *testing.T) {
	db := setupTestAuthDB(t)
	userRepo := repositories.NewUserRepository(db)

	user := &models.User{
		Username:  "testuser",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
	}
	require.NoError(t, user.SetPassword("password123"))

	t.Run("Create user", func(t *testing.T) {
		err := userRepo.Create(user)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, user.ID)
	})

	t.Run("Get user by ID", func(t *testing.T) {
		foundUser, err := userRepo.GetByID(user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, foundUser.Username)
		assert.Equal(t, user.Email, foundUser.Email)
	})

	t.Run("Get user by username", func(t *testing.T) {
		foundUser, err := userRepo.GetByUsername(user.Username)
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundUser.ID)
	})

	t.Run("Get user by email", func(t *testing.T) {
		foundUser, err := userRepo.GetByEmail(user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundUser.ID)
	})

	t.Run("Update user", func(t *testing.T) {
		user.FirstName = "Updated"
		err := userRepo.Update(user)
		require.NoError(t, err)

		foundUser, err := userRepo.GetByID(user.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", foundUser.FirstName)
	})

	t.Run("List users", func(t *testing.T) {
		users, err := userRepo.List(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 1)
	})

	t.Run("Create nil user returns error", func(t *testing.T) {
		err := userRepo.Create(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user cannot be nil")
	})
}

func TestAuthService(t *testing.T) {
	db := setupTestAuthDB(t)
	userRepo := repositories.NewUserRepository(db)
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)
	authService := auth.NewService(userRepo, jwtManager)

	t.Run("Create user", func(t *testing.T) {
		req := &auth.CreateUserRequest{
			Username:  "testuser",
			Email:     "test@example.com",
			Password:  "password123",
			FirstName: "Test",
			LastName:  "User",
		}

		user, err := authService.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, req.Username, user.Username)
		assert.Equal(t, req.Email, user.Email)
		assert.True(t, user.IsActive)
	})

	t.Run("Login with valid credentials", func(t *testing.T) {
		req := &auth.LoginRequest{
			Username: "testuser",
			Password: "password123",
		}

		response, err := authService.Login(req)
		require.NoError(t, err)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, "testuser", response.User.Username)
		assert.Equal(t, "test@example.com", response.User.Email)
	})

	t.Run("Login with invalid credentials", func(t *testing.T) {
		req := &auth.LoginRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}

		_, err := authService.Login(req)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})

	t.Run("Login with non-existent user", func(t *testing.T) {
		req := &auth.LoginRequest{
			Username: "nonexistent",
			Password: "password123",
		}

		_, err := authService.Login(req)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})

	t.Run("Create duplicate user", func(t *testing.T) {
		req := &auth.CreateUserRequest{
			Username: "testuser",
			Email:    "different@example.com",
			Password: "password123",
		}

		_, err := authService.CreateUser(req)
		assert.ErrorIs(t, err, auth.ErrUserExists)
	})
}