package config

import (
	"os"
	"strconv"
)

type Config struct {
	Environment      string
	Port             string
	MongoURI         string
	MongoDatabase    string
	RedisAddr        string
	RedisPassword    string
	MinIOEndpoint    string
	MinIOAccessKey   string
	MinIOSecretKey   string
	MinIOBucket      string
	MinIOUseSSL      bool
	JWTSecret        string
	LogLevel         string
}

func Load() (*Config, error) {
	return &Config{
		Environment:      getEnv("ENVIRONMENT", "development"),
		Port:             getEnv("PROJECT_SERVICE_PORT", "8081"),
		MongoURI:         getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:    getEnv("MONGO_DATABASE", "texflow"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		MinIOEndpoint:    getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:   getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:   getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:      getEnv("MINIO_BUCKET", "texflow"),
		MinIOUseSSL:      getEnvAsBool("MINIO_USE_SSL", false),
		JWTSecret:        getEnv("JWT_SECRET", "secret"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	valStr := getEnv(key, "")
	if valStr == "" {
		return fallback
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return fallback
	}
	return val
}
