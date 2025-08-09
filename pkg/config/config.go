package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		Host            string        `mapstructure:"host"`
		Port            int           `mapstructure:"port"`
		Username        string        `mapstructure:"username"`
		Password        string        `mapstructure:"password"`
		Database        string        `mapstructure:"database"`
		SSLMode         string        `mapstructure:"sslmode"`
		MaxConnections  int           `mapstructure:"max_connections"`
		MaxIdleConns    int           `mapstructure:"max_idle_connections"`
		ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
		ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	} `mapstructure:"database"`

	API struct {
		Port    int    `mapstructure:"port"`
		TLSCert string `mapstructure:"tls_cert"`
		TLSKey  string `mapstructure:"tls_key"`
	} `mapstructure:"api"`

	Auth struct {
		JWTSecret   string        `mapstructure:"jwt_secret"`
		TokenExpiry time.Duration `mapstructure:"token_expiry"`
	} `mapstructure:"auth"`

	Kubernetes struct {
		Namespace string `mapstructure:"namespace"`
	} `mapstructure:"kubernetes"`

	Log struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"log"`

	InitialAdmin struct {
		Enabled   bool   `mapstructure:"enabled"`
		Username  string `mapstructure:"username"`
		Password  string `mapstructure:"password"`
		Email     string `mapstructure:"email"`
		FirstName string `mapstructure:"first_name"`
		LastName  string `mapstructure:"last_name"`
	} `mapstructure:"initial_admin"`
}

func Load() (*Config, error) {
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.username", "postgres")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "ssvirt")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_connections", 25)
	viper.SetDefault("database.max_idle_connections", 10)
	viper.SetDefault("database.conn_max_lifetime", "1h")
	viper.SetDefault("database.conn_max_idle_time", "10m")
	viper.SetDefault("api.port", 8080)
	// JWT secret MUST be explicitly configured - no insecure default
	if os.Getenv("SSVIRT_AUTH_JWT_SECRET") == "" {
		log.Println("WARNING: JWT secret not configured. Set SSVIRT_AUTH_JWT_SECRET environment variable.")
		viper.SetDefault("auth.jwt_secret", "development-secret-change-in-production")
	}
	viper.SetDefault("auth.token_expiry", "24h")
	viper.SetDefault("kubernetes.namespace", "ssvirt-system")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("initial_admin.enabled", false)
	viper.SetDefault("initial_admin.username", "admin")
	viper.SetDefault("initial_admin.email", "admin@example.com")
	viper.SetDefault("initial_admin.first_name", "System")
	viper.SetDefault("initial_admin.last_name", "Administrator")

	viper.SetEnvPrefix("SSVIRT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/ssvirt/")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Load initial admin credentials from Kubernetes secret if specified
	if err := loadInitialAdminFromSecret(&config); err != nil {
		// If initial admin is enabled but we can't load from secret, this is an error
		if config.InitialAdmin.Enabled && config.InitialAdmin.Password == "" {
			return nil, fmt.Errorf("initial admin enabled but failed to load credentials from secret: %w", err)
		}
		log.Printf("Warning: Failed to load initial admin credentials from secret: %v", err)
	}

	return &config, nil
}

// loadInitialAdminFromSecret loads initial admin credentials from a Kubernetes secret
func loadInitialAdminFromSecret(config *Config) error {
	// Always try to load from the standard initial admin secret mount if it exists
	secretPath := "/var/run/secrets/initial-admin" // #nosec G101 - This is a mount path, not hardcoded credentials

	// Check if the secret mount exists
	if _, err := os.Stat(secretPath); os.IsNotExist(err) {
		// No secret mounted, nothing to load
		return nil
	}

	// Helper function to read and decode a secret field
	readSecretField := func(fieldName string) (string, error) {
		// Validate field name to prevent path traversal
		if strings.Contains(fieldName, "..") || strings.Contains(fieldName, "/") {
			return "", fmt.Errorf("invalid field name: %s", fieldName)
		}

		filePath := filepath.Join(secretPath, fieldName)
		data, err := os.ReadFile(filePath) // #nosec G304 - Path is validated above to prevent traversal
		if err != nil {
			return "", err
		}

		// Try to decode as base64 first, if that fails, use as plain text
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			// Not base64, use as plain text
			return strings.TrimSpace(string(data)), nil
		}
		return strings.TrimSpace(string(decoded)), nil
	}

	// Load credentials from secret
	if username, err := readSecretField("username"); err == nil && username != "" {
		config.InitialAdmin.Username = username
		config.InitialAdmin.Enabled = true
	}

	if password, err := readSecretField("password"); err == nil && password != "" {
		config.InitialAdmin.Password = password
	}

	if email, err := readSecretField("email"); err == nil && email != "" {
		config.InitialAdmin.Email = email
	}

	// Try both underscore and dash formats for backward compatibility
	if firstName, err := readSecretField("first_name"); err == nil && firstName != "" {
		config.InitialAdmin.FirstName = firstName
	} else if firstName, err := readSecretField("first-name"); err == nil && firstName != "" {
		config.InitialAdmin.FirstName = firstName
	}

	if lastName, err := readSecretField("last_name"); err == nil && lastName != "" {
		config.InitialAdmin.LastName = lastName
	} else if lastName, err := readSecretField("last-name"); err == nil && lastName != "" {
		config.InitialAdmin.LastName = lastName
	}

	log.Printf("Loaded initial admin credentials from secret")
	return nil
}
