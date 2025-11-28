package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"compilation/internal/config"
	"compilation/internal/handlers"
	"compilation/internal/middleware"
	"compilation/internal/queue"
	"compilation/internal/repository"
	"compilation/internal/service"
	"compilation/internal/storage"
	"compilation/internal/worker"
	"compilation/pkg/auth"
	"compilation/pkg/logger"
	"compilation/pkg/metrics"
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

	log.Info("Starting Compilation Service",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
		zap.Int("max_workers", cfg.MaxWorkers),
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

	// Initialize MinIO client
	minioClient, err := storage.NewMinIOClient(
		cfg.MinIOEndpoint,
		cfg.MinIOPublicEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
		log,
	)
	if err != nil {
		log.Fatal("Failed to initialize MinIO client", zap.Error(err))
	}

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal("Failed to create Docker client", zap.Error(err))
	}
	defer dockerClient.Close()

	log.Info("Connected to Docker daemon successfully")

	// Pull TeX Live image if not exists
	if err := pullTexLiveImage(context.Background(), dockerClient, cfg.TexLiveImage, log); err != nil {
		log.Warn("Failed to pull TeX Live image", zap.Error(err))
	}

	// Initialize JWT manager
	jwtManager, err := auth.NewJWTManager(
		cfg.JWTPrivateKeyPath,
		cfg.JWTPublicKeyPath,
		cfg.JWTSecret,
		15*time.Minute, // access token duration (unused for validation)
		7*24*time.Hour, // refresh token duration (unused for validation)
	)
	if err != nil {
		log.Fatal("Failed to initialize JWT manager", zap.Error(err))
	}

	// Initialize repositories
	compilationRepo := repository.NewCompilationRepository(db)

	// Create indexes
	if err := compilationRepo.CreateIndexes(context.Background()); err != nil {
		log.Error("Failed to create indexes", zap.Error(err))
	}

	// Initialize Redis queue
	redisQueue := queue.NewRedisQueue(redisClient, log)
	if err := redisQueue.Initialize(context.Background()); err != nil {
		log.Fatal("Failed to initialize Redis queue", zap.Error(err))
	}

	// Initialize project service
	projectService := service.NewProjectService(db, minioClient, log)

	// Initialize compilation service
	compilationService := service.NewCompilationService(
		compilationRepo,
		redisQueue,
		redisClient,
		minioClient,
		log,
		cfg.EnableCache,
		cfg.CacheTTL,
		cfg.MaxCompilationsPerUser,
	)

	// Initialize Docker worker
	dockerWorker := worker.NewDockerWorker(
		dockerClient,
		minioClient,
		log,
		cfg.TexLiveImage,
		cfg.CompilationTimeout,
		cfg.CompilationMemory,
		cfg.CompilationCPUs,
		cfg.CompilationVolume,
	)

	// Initialize worker manager
	workerManager := worker.NewManager(
		redisQueue,
		compilationRepo,
		dockerWorker,
		log,
		cfg.MaxWorkers,
	)

	// Start worker manager
	if err := workerManager.Start(context.Background()); err != nil {
		log.Fatal("Failed to start worker manager", zap.Error(err))
	}

	// Initialize handlers
	compilationHandler := handlers.NewCompilationHandler(
		compilationService,
		projectService,
		log,
	)

	// Initialize metrics
	metricsInst := metrics.NewMetrics("compilation_service")

	// Setup HTTP server
	router := setupRouter(compilationHandler, jwtManager, metricsInst, log, cfg.Environment)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second, // Longer for compilation requests
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

	// Shutdown worker manager first
	if err := workerManager.Shutdown(ctx); err != nil {
		log.Error("Worker manager shutdown error", zap.Error(err))
	}

	// Shutdown HTTP server
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

func pullTexLiveImage(ctx context.Context, dockerClient *client.Client, image string, log *zap.Logger) error {
	log.Info("Checking TeX Live image", zap.String("image", image))

	// Check if image exists locally
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return err
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == image {
				log.Info("TeX Live image already exists locally")
				return nil
			}
		}
	}

	log.Info("Pulling TeX Live image (this may take a while)...")

	// Pull the image
	reader, err := dockerClient.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Read the output to ensure the pull completes
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read pull output: %w", err)
	}

	log.Info("TeX Live image pulled successfully", zap.String("image", image))
	return nil
}

func setupRouter(
	compilationHandler *handlers.CompilationHandler,
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

	// Health check endpoints (no auth required)
	router.GET("/health", compilationHandler.Health)
	router.GET("/ready", compilationHandler.Health)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(jwtManager, log))
	{
		compilation := v1.Group("/compilation")
		{
			// Compile a project
			compilation.POST("/compile", compilationHandler.Compile)

			// Get compilation status
			compilation.GET("/:id", compilationHandler.GetCompilation)

			// List project compilations
			compilation.GET("/project/:project_id", compilationHandler.ListCompilations)

			// Get statistics
			compilation.GET("/stats", compilationHandler.GetStats)

			// Get queue statistics
			compilation.GET("/queue", compilationHandler.GetQueueStats)
		}
	}

	return router
}
