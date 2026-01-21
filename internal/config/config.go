package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
// In Go, we use structs to group related data.
// The `mapstructure` tags tell Viper how to map env vars to struct fields.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	SendGrid SendGridConfig
	Firebase FirebaseConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Port         int    `mapstructure:"PORT"`
	Environment  string `mapstructure:"ENVIRONMENT"`
	AllowOrigins string `mapstructure:"ALLOW_ORIGINS"`
}

type DatabaseConfig struct {
	URL             string `mapstructure:"DATABASE_URL"`
	MaxOpenConns    int    `mapstructure:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int    `mapstructure:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime int    `mapstructure:"DB_CONN_MAX_LIFETIME"`
}

type RedisConfig struct {
	URL string `mapstructure:"REDIS_URL"`
}

type SendGridConfig struct {
	APIKey    string `mapstructure:"SENDGRID_API_KEY"`
	FromEmail string `mapstructure:"SENDGRID_FROM_EMAIL"`
	FromName  string `mapstructure:"SENDGRID_FROM_NAME"`
}

type FirebaseConfig struct {
	CredentialsPath string `mapstructure:"FIREBASE_CREDENTIALS_PATH"`
}

type AuthConfig struct {
	JWTSecret string   `mapstructure:"JWT_SECRET"`
	APIKeys   []string // Parsed from comma-separated INTERNAL_API_KEYS
}

// Load reads configuration from environment variables.
// This follows the 12-factor app methodology.
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("PORT", 5003)
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 20)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME", 300) // 5 minutes in seconds
	viper.SetDefault("ALLOW_ORIGINS", "http://localhost:3000,http://localhost:5001")
	viper.SetDefault("SENDGRID_FROM_NAME", "PrepMyApp")

	// Read from .env file if it exists (for local development)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")

	// Ignore error if .env doesn't exist - we'll use env vars
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Enable reading from environment variables
	viper.AutomaticEnv()

	cfg := &Config{}

	// Unmarshal server config
	if err := viper.Unmarshal(&cfg.Server); err != nil {
		return nil, fmt.Errorf("failed to unmarshal server config: %w", err)
	}

	// Unmarshal database config
	if err := viper.Unmarshal(&cfg.Database); err != nil {
		return nil, fmt.Errorf("failed to unmarshal database config: %w", err)
	}

	// Unmarshal redis config
	if err := viper.Unmarshal(&cfg.Redis); err != nil {
		return nil, fmt.Errorf("failed to unmarshal redis config: %w", err)
	}

	// Unmarshal sendgrid config
	if err := viper.Unmarshal(&cfg.SendGrid); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sendgrid config: %w", err)
	}

	// Unmarshal firebase config
	if err := viper.Unmarshal(&cfg.Firebase); err != nil {
		return nil, fmt.Errorf("failed to unmarshal firebase config: %w", err)
	}

	// Unmarshal auth config
	if err := viper.Unmarshal(&cfg.Auth); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth config: %w", err)
	}

	// Parse comma-separated API keys
	apiKeysStr := viper.GetString("INTERNAL_API_KEYS")
	if apiKeysStr != "" {
		cfg.Auth.APIKeys = strings.Split(apiKeysStr, ",")
		// Trim whitespace from each key
		for i, key := range cfg.Auth.APIKeys {
			cfg.Auth.APIKeys[i] = strings.TrimSpace(key)
		}
	}

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration values are set.
// In Go, methods are defined outside the struct, with a receiver.
func (c *Config) Validate() error {
	var missing []string

	// In production, these are required
	if c.Server.Environment == "production" {
		if c.Database.URL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		if c.Auth.JWTSecret == "" {
			missing = append(missing, "JWT_SECRET")
		}
		if c.SendGrid.APIKey == "" {
			missing = append(missing, "SENDGRID_API_KEY")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}
