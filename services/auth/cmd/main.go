package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"auth/internal/config"
	"auth/internal/handlers"
	"auth/internal/middleware"
	"auth/internal/repository"
	"auth/internal/service"
	"auth/pkg/auth"
	"auth/pkg/logger"
	"auth/pkg/metrics"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.InitLogger(cfg.Environment)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Auth Service",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
	)

	// Connect to MongoDB
	mongoClient, err := connectMongoDB(cfg.MongoURI, log)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer mongoClient.Disconnect(context.Background())

	db := mongoClient.Database(cfg.MongoDatabase)

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize JWT manager
	jwtManager, err := auth.NewJWTManager(
		cfg.JWTPrivateKeyPath,
		cfg.JWTPublicKeyPath,
		cfg.JWTSecret,
		cfg.JWTAccessTokenExpiry,
		cfg.JWTRefreshTokenExpiry,
	)
	if err != nil {
		log.Fatal("Failed to initialize JWT manager", zap.Error(err))
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)

	// Create indexes
	if err := userRepo.CreateIndexes(context.Background()); err != nil {
		log.Error("Failed to create indexes", zap.Error(err))
	}

	// Initialize services
	authService := service.NewAuthService(userRepo, jwtManager, redisClient, log, cfg.BCryptCost)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, log)

	// Initialize metrics
	metricsInst := metrics.NewMetrics("auth_service")

	// Setup HTTP server
	router := setupRouter(authHandler, jwtManager, metricsInst, log, cfg.Environment)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Server starting", zap.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

func connectMongoDB(uri string, log *zap.Logger) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	log.Info("Connected to MongoDB successfully")
	return client, nil
}

func setupRouter(
	authHandler *handlers.AuthHandler,
	jwtManager *auth.JWTManager,
	metricsInst *metrics.Metrics,
	log *zap.Logger,
	environment string,
) *gin.Engine {
	// Set Gin mode
	if environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.MetricsMiddleware(metricsInst))

	// Health check endpoint (no auth required)
	router.GET("/health", authHandler.Health)
	router.GET("/ready", authHandler.Health)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			// Public endpoints
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)

			// Protected endpoints
			protected := auth.Group("")
			protected.Use(middleware.AuthMiddleware(jwtManager, log))
			{
				protected.POST("/logout", authHandler.Logout)
				protected.GET("/me", authHandler.Me)
			}
		}
	}

	return router
}
