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
	CredentialsJSON string `mapstructure:"FIREBASE_CREDENTIALS_JSON"` // Alternative: JSON string for Replit Secrets
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

	// Build DATABASE_URL from individual PG* variables if not set (for Replit)
	if cfg.Database.URL == "" {
		pgHost := viper.GetString("PGHOST")
		pgPort := viper.GetString("PGPORT")
		pgUser := viper.GetString("PGUSER")
		pgPassword := viper.GetString("PGPASSWORD")
		pgDatabase := viper.GetString("PGDATABASE")

		if pgHost != "" && pgDatabase != "" {
			if pgPort == "" {
				pgPort = "5432"
			}
			cfg.Database.URL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require",
				pgUser, pgPassword, pgHost, pgPort, pgDatabase)
		}
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

	// Read secrets directly from environment
	// (Viper's Unmarshal doesn't properly read env vars for nested struct fields)
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = viper.GetString("JWT_SECRET")
	}
	if cfg.SendGrid.APIKey == "" {
		cfg.SendGrid.APIKey = viper.GetString("SENDGRID_API_KEY")
	}
	if cfg.Firebase.CredentialsJSON == "" {
		cfg.Firebase.CredentialsJSON = viper.GetString("FIREBASE_CREDENTIALS_JSON")
	}
	if cfg.Firebase.CredentialsPath == "" {
		cfg.Firebase.CredentialsPath = viper.GetString("FIREBASE_CREDENTIALS_PATH")
	}

	// Parse comma-separated API keys
	apiKeysStr := viper.GetString("INTERNAL_API_KEYS")
	if apiKeysStr != "" {
		cfg.Auth.APIKeys = strings.Split(apiKeysStr, ",")
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

	// In production, only DATABASE_URL is strictly required
	// JWT_SECRET and SENDGRID_API_KEY are optional (service will work with limited functionality)
	if c.Server.Environment == "production" {
		if c.Database.URL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		// Log warnings for missing optional secrets
		if c.Auth.JWTSecret == "" {
			fmt.Println("WARNING: JWT_SECRET not set - JWT authentication will not work")
		}
		if c.SendGrid.APIKey == "" {
			fmt.Println("WARNING: SENDGRID_API_KEY not set - email notifications will not work")
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
