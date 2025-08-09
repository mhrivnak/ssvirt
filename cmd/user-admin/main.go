package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: user-admin <command> [args...]")
		fmt.Println("Commands:")
		fmt.Println("  create-user <username> <email> <password> [full_name] [description]")
		fmt.Println("  list-users")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	userRepo := repositories.NewUserRepository(db.DB)

	command := os.Args[1]
	switch command {
	case "create-user":
		if len(os.Args) < 5 {
			fmt.Println("Usage: user-admin create-user <username> <email> <password> [full_name] [description]")
			os.Exit(1)
		}

		req := &auth.CreateUserRequest{
			Username: os.Args[2],
			Email:    os.Args[3],
			Password: os.Args[4],
		}

		if len(os.Args) > 5 {
			req.FullName = os.Args[5]
		}
		if len(os.Args) > 6 {
			req.Description = os.Args[6]
		}

		// We don't have the full auth service here, so create user directly
		user, err := createUserDirect(userRepo, req)
		if err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}

		fmt.Printf("User created successfully!\n")
		fmt.Printf("ID: %s\n", user.ID)
		fmt.Printf("Username: %s\n", user.Username)
		fmt.Printf("Email: %s\n", user.Email)

	case "list-users":
		users, err := userRepo.List(100, 0)
		if err != nil {
			log.Fatalf("Failed to list users: %v", err)
		}

		fmt.Printf("Found %d users:\n", len(users))
		for _, user := range users {
			fmt.Printf("- %s (%s) - %s\n", user.Username, user.Email, user.FullName)
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func createUserDirect(userRepo *repositories.UserRepository, req *auth.CreateUserRequest) (*models.User, error) {
	// Check if user already exists
	if _, err := userRepo.GetByUsername(req.Username); err == nil {
		return nil, auth.ErrUserExists
	}

	if _, err := userRepo.GetByEmail(req.Email); err == nil {
		return nil, auth.ErrUserExists
	}

	user := &models.User{
		Username:    req.Username,
		Email:       req.Email,
		FullName:    req.FullName,
		Description: req.Description,
		Enabled:     true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		return nil, err
	}

	if err := userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}
