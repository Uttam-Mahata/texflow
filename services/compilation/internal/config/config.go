package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the compilation service
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

	// MinIO
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool

	// JWT
	JWTSecret         string
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string

	// Compilation Settings
	CompilationTimeout   time.Duration
	CompilationMemory    int64 // Bytes
	CompilationCPUs      int64 // Nano CPUs (1 CPU = 1e9)
	MaxWorkers           int
	WorkerPollInterval   time.Duration
	EnableCache          bool
	CacheTTL             time.Duration
	MaxCompilationsPerUser int

	// Docker
	DockerHost        string
	TexLiveImage      string
	CompilationVolume string

	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists (development)
	godotenv.Load()

	compilationTimeout, err := time.ParseDuration(getEnv("COMPILATION_TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid COMPILATION_TIMEOUT: %w", err)
	}

	workerPollInterval, err := time.ParseDuration(getEnv("WORKER_POLL_INTERVAL", "1s"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_POLL_INTERVAL: %w", err)
	}

	cacheTTL, err := time.ParseDuration(getEnv("CACHE_TTL", "1h"))
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
	}

	compilationMemory, err := strconv.ParseInt(getEnv("COMPILATION_MEMORY_LIMIT", "2147483648"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid COMPILATION_MEMORY_LIMIT: %w", err)
	}

	compilationCPUs, err := strconv.ParseInt(getEnv("COMPILATION_CPU_LIMIT", "2"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid COMPILATION_CPU_LIMIT: %w", err)
	}

	maxWorkers, err := strconv.Atoi(getEnv("MAX_COMPILATION_WORKERS", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_COMPILATION_WORKERS: %w", err)
	}

	maxCompilationsPerUser, err := strconv.Atoi(getEnv("MAX_COMPILATIONS_PER_USER", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_COMPILATIONS_PER_USER: %w", err)
	}

	config := &Config{
		Environment:            getEnv("ENVIRONMENT", "development"),
		Port:                   getEnv("COMPILATION_SERVICE_PORT", "8084"),
		MongoURI:               getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:          getEnv("MONGO_DATABASE", "texflow"),
		RedisAddr:              getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		RedisDB:                0,
		MinIOEndpoint:          getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:         getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:         getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:            getEnv("MINIO_BUCKET", "texflow"),
		MinIOUseSSL:            getEnv("MINIO_USE_SSL", "false") == "true",
		JWTSecret:              getEnv("JWT_SECRET", ""),
		JWTPrivateKeyPath:      getEnv("JWT_PRIVATE_KEY_PATH", "./keys/jwt-private.pem"),
		JWTPublicKeyPath:       getEnv("JWT_PUBLIC_KEY_PATH", "./keys/jwt-public.pem"),
		CompilationTimeout:     compilationTimeout,
		CompilationMemory:      compilationMemory,
		CompilationCPUs:        compilationCPUs * 1e9, // Convert to nano CPUs
		MaxWorkers:             maxWorkers,
		WorkerPollInterval:     workerPollInterval,
		EnableCache:            getEnv("ENABLE_COMPILATION_CACHE", "true") == "true",
		CacheTTL:               cacheTTL,
		MaxCompilationsPerUser: maxCompilationsPerUser,
		DockerHost:             getEnv("DOCKER_HOST", ""),
		TexLiveImage:           getEnv("TEXLIVE_IMAGE", "texlive/texlive:latest"),
		CompilationVolume:      getEnv("COMPILATION_VOLUME", "/tmp/compilations"),
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
	if c.MinIOEndpoint == "" {
		return fmt.Errorf("MINIO_ENDPOINT is required")
	}
	if c.JWTSecret == "" && c.JWTPublicKeyPath == "" {
		return fmt.Errorf("either JWT_SECRET or JWT_PUBLIC_KEY_PATH is required")
	}
	if c.MaxWorkers <= 0 {
		return fmt.Errorf("MAX_COMPILATION_WORKERS must be positive")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
