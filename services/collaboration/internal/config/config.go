package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the collaboration service
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
	JWTSecret         string
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string

	// Collaboration Settings
	SnapshotInterval     int   // Create snapshot every N updates
	MaxUpdatesPerFetch   int   // Maximum updates to return per request
	UpdateRetentionDays  int   // Days to keep updates before archiving
	MaxDocumentSizeBytes int64 // Maximum document size

	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists (development)
	godotenv.Load()

	snapshotInterval, err := strconv.Atoi(getEnv("SNAPSHOT_INTERVAL", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid SNAPSHOT_INTERVAL: %w", err)
	}

	maxUpdatesPerFetch, err := strconv.Atoi(getEnv("MAX_UPDATES_PER_FETCH", "1000"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_UPDATES_PER_FETCH: %w", err)
	}

	updateRetentionDays, err := strconv.Atoi(getEnv("UPDATE_RETENTION_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid UPDATE_RETENTION_DAYS: %w", err)
	}

	maxDocumentSize, err := strconv.ParseInt(getEnv("MAX_DOCUMENT_SIZE_BYTES", "10485760"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_DOCUMENT_SIZE_BYTES: %w", err)
	}

	config := &Config{
		Environment:          getEnv("ENVIRONMENT", "development"),
		Port:                 getEnv("COLLABORATION_SERVICE_PORT", "8083"),
		MongoURI:             getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:        getEnv("MONGO_DATABASE", "texflow"),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              0,
		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTPrivateKeyPath:    getEnv("JWT_PRIVATE_KEY_PATH", "./keys/jwt-private.pem"),
		JWTPublicKeyPath:     getEnv("JWT_PUBLIC_KEY_PATH", "./keys/jwt-public.pem"),
		SnapshotInterval:     snapshotInterval,
		MaxUpdatesPerFetch:   maxUpdatesPerFetch,
		UpdateRetentionDays:  updateRetentionDays,
		MaxDocumentSizeBytes: maxDocumentSize,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
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
	if c.JWTSecret == "" && c.JWTPublicKeyPath == "" {
		return fmt.Errorf("either JWT_SECRET or JWT_PUBLIC_KEY_PATH is required")
	}
	if c.SnapshotInterval <= 0 {
		return fmt.Errorf("SNAPSHOT_INTERVAL must be positive")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
