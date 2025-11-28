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
	"github.com/texflow/services/project/internal/config"
	"github.com/texflow/services/project/internal/handlers"
	"github.com/texflow/services/project/internal/repository"
	"github.com/texflow/services/project/internal/service"
	"github.com/texflow/services/project/internal/storage"
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
	var log *zap.Logger
	if cfg.Environment == "production" {
		log, _ = zap.NewProduction()
	} else {
		log, _ = zap.NewDevelopment()
	}
	defer log.Sync()

	log.Info("Starting Project Service",
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

	// Initialize MinIO
	minioClient, err := storage.NewMinIOClient(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
		log,
	)
	if err != nil {
		log.Fatal("Failed to initialize MinIO client", zap.Error(err))
	}

	// Initialize repositories
	projectRepo := repository.NewProjectRepository(db)
	fileRepo := repository.NewFileRepository(db)

	// Initialize services
	projectService := service.NewProjectService(projectRepo, fileRepo, minioClient, log)

	// Initialize handlers
	projectHandler := handlers.NewProjectHandler(projectService, log)

	// Setup Router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Health check
	router.GET("/health", projectHandler.Health)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API Routes
	api := router.Group("/api/v1")
	{
		projects := api.Group("/projects")
		{
			projects.POST("", projectHandler.CreateProject)
			projects.GET("", projectHandler.GetProjects)
			projects.GET("/:id", projectHandler.GetProject)
			projects.PUT("/:id", projectHandler.UpdateProject)
			projects.DELETE("/:id", projectHandler.DeleteProject)
			projects.POST("/:id/share", projectHandler.ShareProject)
			projects.POST("/:id/files", projectHandler.CreateFile)
			projects.GET("/:id/files", projectHandler.ListFiles)
			projects.GET("/:id/files/:fileId", projectHandler.GetFileMetadata)
			projects.GET("/:id/files/:fileId/content", projectHandler.GetFileContent)
			projects.PUT("/:id/files/:fileId", projectHandler.UpdateFile)
		}
	}

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("listen: ", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", zap.Error(err))
	}
	log.Info("Server exiting")
}

func connectMongoDB(uri string, log *zap.Logger) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return client, nil
}
