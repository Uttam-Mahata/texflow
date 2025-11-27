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
	"github.com/texflow/services/websocket/internal/config"
	"github.com/texflow/services/websocket/internal/handlers"
	"github.com/texflow/services/websocket/internal/middleware"
	ws "github.com/texflow/services/websocket/internal/websocket"
	"github.com/texflow/services/websocket/pkg/auth"
	"github.com/texflow/services/websocket/pkg/logger"
	"github.com/texflow/services/websocket/pkg/metrics"
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

	log.Info("Starting WebSocket Service",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
	)

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

	// Initialize WebSocket hub
	hub := ws.NewHub(redisClient, log)

	// Start hub in background
	go hub.Run()
	log.Info("WebSocket hub started")

	// Initialize handlers
	wsHandler := handlers.NewWebSocketHandler(hub, log)

	// Initialize metrics
	metricsInst := metrics.NewMetrics("websocket_service")

	// Setup HTTP server
	router := setupRouter(wsHandler, jwtValidator, metricsInst, log, cfg.Environment)

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

func setupRouter(
	wsHandler *handlers.WebSocketHandler,
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
	router.GET("/health", wsHandler.Health)
	router.GET("/ready", wsHandler.Health)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// WebSocket stats endpoint (requires auth)
		v1.GET("/stats", middleware.AuthMiddleware(jwtValidator, log), wsHandler.GetStats)

		// WebSocket connection endpoint
		// Note: Authentication is handled via query parameter or header
		v1.GET("/ws/:project_id", middleware.AuthMiddleware(jwtValidator, log), wsHandler.HandleConnection)
	}

	return router
}
