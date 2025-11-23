package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the auth service
type Config struct {
	Environment string
	Port        string

	// MongoDB
	MongoURI      string
	MongoDatabase string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret              string
	JWTPrivateKeyPath      string
	JWTPublicKeyPath       string
	JWTAccessTokenExpiry   time.Duration
	JWTRefreshTokenExpiry  time.Duration

	// Security
	BCryptCost int

	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists (development)
	godotenv.Load()

	accessTokenExpiry, err := time.ParseDuration(getEnv("JWT_ACCESS_TOKEN_EXPIRY", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TOKEN_EXPIRY: %w", err)
	}

	refreshTokenExpiry, err := time.ParseDuration(getEnv("JWT_REFRESH_TOKEN_EXPIRY", "168h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TOKEN_EXPIRY: %w", err)
	}

	config := &Config{
		Environment:            getEnv("ENVIRONMENT", "development"),
		Port:                   getEnv("AUTH_SERVICE_PORT", "8080"),
		MongoURI:               getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:          getEnv("MONGO_DATABASE", "texflow"),
		RedisAddr:              getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		RedisDB:                0,
		JWTSecret:              getEnv("JWT_SECRET", ""),
		JWTPrivateKeyPath:      getEnv("JWT_PRIVATE_KEY_PATH", "./keys/jwt-private.pem"),
		JWTPublicKeyPath:       getEnv("JWT_PUBLIC_KEY_PATH", "./keys/jwt-public.pem"),
		JWTAccessTokenExpiry:   accessTokenExpiry,
		JWTRefreshTokenExpiry:  refreshTokenExpiry,
		BCryptCost:             12,
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.MongoURI == "" {
		return fmt.Errorf("MONGO_URI is required")
	}
	if c.JWTSecret == "" && c.JWTPrivateKeyPath == "" {
		return fmt.Errorf("either JWT_SECRET or JWT_PRIVATE_KEY_PATH is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
