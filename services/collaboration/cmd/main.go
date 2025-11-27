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
	"collaboration/internal/config"
	"collaboration/internal/handlers"
	"collaboration/internal/middleware"
	"collaboration/internal/repository"
	"collaboration/internal/service"
	"collaboration/pkg/auth"
	"collaboration/pkg/logger"
	"collaboration/pkg/metrics"
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

	log.Info("Starting Collaboration Service",
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
	log.Info("Connected to Redis successfully")

	// Initialize JWT validator
	jwtValidator, err := auth.NewJWTValidator(
		cfg.JWTPublicKeyPath,
		cfg.JWTSecret,
	)
	if err != nil {
		log.Fatal("Failed to initialize JWT validator", zap.Error(err))
	}

	// Initialize repositories
	updateRepo := repository.NewUpdateRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)

	// Create indexes
	if err := updateRepo.CreateIndexes(context.Background()); err != nil {
		log.Error("Failed to create update indexes", zap.Error(err))
	}
	if err := snapshotRepo.CreateIndexes(context.Background()); err != nil {
		log.Error("Failed to create snapshot indexes", zap.Error(err))
	}

	// Initialize services
	collabService := service.NewCollaborationService(
		updateRepo,
		snapshotRepo,
		log,
		cfg.SnapshotInterval,
		cfg.MaxUpdatesPerFetch,
	)

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := collabService.CleanupOldData(context.Background(), cfg.UpdateRetentionDays); err != nil {
				log.Error("Failed to cleanup old data", zap.Error(err))
			}
		}
	}()

	// Initialize handlers
	collabHandler := handlers.NewCollaborationHandler(collabService, log)

	// Initialize metrics
	metricsInst := metrics.NewMetrics("collaboration_service")

	// Setup HTTP server
	router := setupRouter(collabHandler, jwtValidator, metricsInst, log, cfg.Environment)

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
	collabHandler *handlers.CollaborationHandler,
	jwtValidator *auth.JWTValidator,
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

	// Health check endpoints (no auth required)
	router.GET("/health", collabHandler.Health)
	router.GET("/ready", collabHandler.Health)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(jwtValidator, log))
	{
		collab := v1.Group("/collaboration")
		{
			// Store update
			collab.POST("/updates", collabHandler.StoreUpdate)

			// Get document state
			collab.GET("/state/:project_id/:document_name", collabHandler.GetDocumentState)

			// Get updates
			collab.GET("/updates/:project_id/:document_name", collabHandler.GetUpdates)

			// Get metrics
			collab.GET("/metrics/:project_id/:document_name", collabHandler.GetMetrics)
		}
	}

	return router
}
