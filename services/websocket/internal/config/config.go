package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the websocket service
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

	// WebSocket
	ReadBufferSize      int
	WriteBufferSize     int
	PingInterval        time.Duration
	PongWait            time.Duration
	WriteWait           time.Duration
	MaxMessageSize      int64
	MaxConnectionsPerIP int

	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists (development)
	godotenv.Load()

	pingInterval, err := time.ParseDuration(getEnv("WS_PING_INTERVAL", "54s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WS_PING_INTERVAL: %w", err)
	}

	pongWait, err := time.ParseDuration(getEnv("WS_PONG_WAIT", "60s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WS_PONG_WAIT: %w", err)
	}

	writeWait, err := time.ParseDuration(getEnv("WS_WRITE_WAIT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WS_WRITE_WAIT: %w", err)
	}

	config := &Config{
		Environment:         getEnv("ENVIRONMENT", "development"),
		Port:                getEnv("WEBSOCKET_SERVICE_PORT", "8082"),
		MongoURI:            getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:       getEnv("MONGO_DATABASE", "texflow"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", ""),
		RedisDB:             0,
		JWTSecret:           getEnv("JWT_SECRET", ""),
		JWTPrivateKeyPath:   getEnv("JWT_PRIVATE_KEY_PATH", "./keys/jwt-private.pem"),
		JWTPublicKeyPath:    getEnv("JWT_PUBLIC_KEY_PATH", "./keys/jwt-public.pem"),
		ReadBufferSize:      1024,
		WriteBufferSize:     1024,
		PingInterval:        pingInterval,
		PongWait:            pongWait,
		WriteWait:           writeWait,
		MaxMessageSize:      512 * 1024, // 512 KB
		MaxConnectionsPerIP: 10,
		LogLevel:            getEnv("LOG_LEVEL", "info"),
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
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	if c.JWTSecret == "" && c.JWTPublicKeyPath == "" {
		return fmt.Errorf("either JWT_SECRET or JWT_PUBLIC_KEY_PATH is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
