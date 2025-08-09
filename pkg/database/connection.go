package database

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
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
		&models.Role{},
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

// BootstrapDefaultData creates default roles and Provider organization
func (db *DB) BootstrapDefaultData() error {
	log.Println("Bootstrapping default data...")

	// Use single transaction to ensure atomicity
	return db.DB.Transaction(func(tx *gorm.DB) error {
		// Create role repository with transaction
		roleRepo := repositories.NewRoleRepository(tx)

		// Create default roles
		if err := roleRepo.CreateDefaultRoles(); err != nil {
			return fmt.Errorf("failed to create default roles: %w", err)
		}
		log.Println("Default roles created successfully")

		// Create organization repository with transaction
		orgRepo := repositories.NewOrganizationRepository(tx)

		// Create default Provider organization
		_, err := orgRepo.CreateDefaultOrganization()
		if err != nil {
			return fmt.Errorf("failed to create default organization: %w", err)
		}
		log.Println("Default Provider organization created successfully")

		log.Println("Default data bootstrap completed successfully")
		return nil
	})
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
		return fmt.Errorf("initial admin password not configured - check secret loading or environment variables")
	}

	// Use a concurrency-safe upsert approach - create only if no admin exists
	fullName := cfg.InitialAdmin.FirstName
	if cfg.InitialAdmin.LastName != "" {
		fullName = fullName + " " + cfg.InitialAdmin.LastName
	}
	return db.createInitialAdminIdempotent(cfg.InitialAdmin.Username, password, cfg.InitialAdmin.Email, fullName)
}

// createInitialAdminIdempotent creates the initial admin user using a concurrency-safe approach
func (db *DB) createInitialAdminIdempotent(username, password, email, fullName string) error {
	user := &models.User{
		Username:    username,
		Email:       email,
		FullName:    fullName,
		Description: "Initial System Administrator",
		Enabled:     true,
	}

	if err := user.SetPassword(password); err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	// Use a transaction to ensure atomicity
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Check if any user with System Administrator role already exists
		var existingAdminCount int64
		if err := tx.Table("user_roles").
			Joins("JOIN roles ON user_roles.role_id = roles.id").
			Where("roles.name = ?", models.RoleSystemAdmin).
			Count(&existingAdminCount).Error; err != nil {
			return fmt.Errorf("failed to count existing system admins: %w", err)
		}

		if existingAdminCount > 0 {
			log.Println("System admin already exists, skipping initial admin creation")
			return nil
		}

		// Get the Provider organization
		var providerOrg models.Organization
		if err := tx.Where("name = ?", models.DefaultOrgName).First(&providerOrg).Error; err != nil {
			return fmt.Errorf("failed to find Provider organization: %w", err)
		}

		// Get the System Administrator role
		var sysAdminRole models.Role
		if err := tx.Where("name = ?", models.RoleSystemAdmin).First(&sysAdminRole).Error; err != nil {
			return fmt.Errorf("failed to find System Administrator role: %w", err)
		}

		// Set organization for the user and populate OrganizationName
		user.OrganizationID = providerOrg.ID
		user.OrganizationName = providerOrg.Name

		// Create the user with ON CONFLICT DO NOTHING behavior for username uniqueness
		result := tx.Where("username = ?", user.Username).FirstOrCreate(user)
		if result.Error != nil {
			return fmt.Errorf("failed to create initial admin user: %w", result.Error)
		}

		// If user was found (not created), check if it already has the system admin role
		if result.RowsAffected == 0 {
			log.Printf("User %s already exists", username)
			// Update existing user's organization if needed
			if err := tx.Model(user).Updates(map[string]interface{}{
				"organization_id":   providerOrg.ID,
				"organization_name": providerOrg.Name,
			}).Error; err != nil {
				return fmt.Errorf("failed to update user organization: %w", err)
			}
		} else {
			log.Printf("Initial admin user created successfully: %s (ID: %s)", user.Username, user.ID)
		}

		// Create UserRole assignment for System Administrator role using upsert to avoid duplicates
		userRole := &models.UserRole{
			UserID:         user.ID,
			RoleID:         sysAdminRole.ID,
			OrganizationID: providerOrg.ID,
		}
		// Check if the role assignment already exists
		var existingUserRole models.UserRole
		err := tx.Where("user_id = ? AND role_id = ? AND organization_id = ?", user.ID, sysAdminRole.ID, providerOrg.ID).First(&existingUserRole).Error
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Create the role assignment
			if err := tx.Create(userRole).Error; err != nil {
				return fmt.Errorf("failed to assign System Administrator role to user: %w", err)
			}
			log.Printf("Assigned System Administrator role to user %s", username)
		} else if err != nil {
			return fmt.Errorf("failed to check existing user role: %w", err)
		} else {
			log.Printf("User %s already has System Administrator role", username)
		}

		return nil
	})

	return err
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
