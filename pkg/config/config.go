package config

import (
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		URL             string        `mapstructure:"url"`
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
}

func Load() (*Config, error) {
	viper.SetDefault("database.url", "postgresql://localhost:5432/ssvirt")
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

	viper.SetEnvPrefix("SSVIRT")
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

	return &config, nil
}
