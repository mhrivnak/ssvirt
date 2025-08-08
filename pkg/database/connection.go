package database

import (
	"fmt"
	"log"
	"strings"

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
	log.Printf("Database connection debug - Host: %s, Port: %s, Username: %q, Database: %s, SSLMode: %s, Password length: %d",
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

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// buildDSN constructs a PostgreSQL DSN (Data Source Name) using GORM recommended format
// DSN format: host=localhost user=gorm password=gorm dbname=gorm port=5432 sslmode=disable
func buildDSN(host, port, username, password, database, sslmode string) string {
	// Build DSN string with all parameters
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, username, password, database, port, sslmode)

	return dsn
}
