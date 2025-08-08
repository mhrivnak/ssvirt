package database

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type DB struct {
	*gorm.DB
}

func NewConnection(cfg *config.Config) (*DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Debug logging to see what database config we're getting
	log.Printf("Database connection debug - Host: %s, Port: %d, Username: %q, Database: %s, SSLMode: %s, Password length: %d",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Username, cfg.Database.Database, cfg.Database.SSLMode, len(cfg.Database.Password))

	// Build DSN connection string using GORM recommended format
	dsn := buildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.Username, cfg.Database.Password, cfg.Database.Database, cfg.Database.SSLMode)

	// Log the DSN without password for debugging
	debugDSN := dsn
	if cfg.Database.Password != "" {
		debugDSN = strings.Replace(dsn, fmt.Sprintf("password=%s", cfg.Database.Password), "password=***", 1)
	}
	log.Printf("Database DSN (sanitized): %s", debugDSN)

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	return &DB{db}, nil
}

func (db *DB) AutoMigrate() error {
	log.Println("Running database auto-migration...")

	err := db.DB.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.VDC{},
		&models.Catalog{},
		&models.VAppTemplate{},
		&models.VApp{},
		&models.VM{},
		&models.UserRole{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	log.Println("Database auto-migration completed successfully")
	return nil
}

// BootstrapInitialAdmin creates an initial admin user if configured, using a concurrency-safe approach
func (db *DB) BootstrapInitialAdmin(cfg *config.Config) error {
	if !cfg.InitialAdmin.Enabled {
		log.Println("Initial admin not enabled, skipping creation")
		return nil
	}

	if cfg.InitialAdmin.Username == "" {
		log.Println("Initial admin username not configured, skipping creation")
		return nil
	}

	password := cfg.InitialAdmin.Password
	if password == "" {
		// Generate a secure random password
		generatedPassword, err := generateSecurePassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate admin password: %w", err)
		}
		password = generatedPassword
		// Log password hash for verification without exposing the actual password
		if hashedPassword, err := hashPassword(password); err == nil {
			log.Printf("Generated initial admin password (hash: %s)", hashedPassword[:16]+"...")
		}
		log.Println("IMPORTANT: Auto-generated password is available only during this startup")
	}

	// Use a concurrency-safe upsert approach - create only if no admin exists
	return db.createInitialAdminIdempotent(cfg.InitialAdmin.Username, password, cfg.InitialAdmin.Email, cfg.InitialAdmin.FirstName, cfg.InitialAdmin.LastName)
}

// createInitialAdminIdempotent creates the initial admin user using a concurrency-safe approach
func (db *DB) createInitialAdminIdempotent(username, password, email, firstName, lastName string) error {
	user := &models.User{
		Username:      username,
		Email:         email,
		FirstName:     firstName,
		LastName:      lastName,
		IsActive:      true,
		IsSystemAdmin: true,
	}

	if err := user.SetPassword(password); err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	// Use a transaction to ensure atomicity and check if any system admin already exists
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Check if any system admin already exists
		var existingAdminCount int64
		if err := tx.Model(&models.User{}).Where("is_system_admin = ?", true).Count(&existingAdminCount).Error; err != nil {
			return fmt.Errorf("failed to count existing system admins: %w", err)
		}

		if existingAdminCount > 0 {
			log.Println("System admin already exists, skipping initial admin creation")
			return nil
		}

		// Create the user with ON CONFLICT DO NOTHING behavior for username uniqueness
		result := tx.Where("username = ?", user.Username).FirstOrCreate(user)
		if result.Error != nil {
			return fmt.Errorf("failed to create initial admin user: %w", result.Error)
		}

		// If user was found (not created), check if it's already a system admin
		if result.RowsAffected == 0 {
			// User exists, check if it's already a system admin
			if user.IsSystemAdmin {
				log.Printf("User %s already exists and is a system admin", username)
				return nil
			}
			// Upgrade existing user to system admin
			if err := tx.Model(user).Update("is_system_admin", true).Error; err != nil {
				return fmt.Errorf("failed to upgrade existing user to system admin: %w", err)
			}
			log.Printf("Upgraded existing user %s to system admin", username)
		} else {
			log.Printf("Initial admin user created successfully: %s (ID: %s)", user.Username, user.ID)
		}

		return nil
	})

	return err
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// hashPassword creates a bcrypt hash of the password for logging purposes
func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// buildDSN constructs a PostgreSQL DSN (Data Source Name) using GORM recommended format
// DSN format: host=localhost user=gorm password=gorm dbname=gorm port=5432 sslmode=disable
func buildDSN(host string, port int, username, password, database, sslmode string) string {
	// Build DSN string with all parameters
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		host, username, password, database, port, sslmode)

	return dsn
}
