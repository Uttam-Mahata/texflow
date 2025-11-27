package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"compilation/internal/models"
	"compilation/internal/queue"
	"compilation/internal/repository"
	"compilation/internal/storage"
	"compilation/internal/worker"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// CompilationService handles compilation business logic
type CompilationService struct {
	compilationRepo *repository.CompilationRepository
	queue           *queue.RedisQueue
	redisClient     *redis.Client
	minioClient     *storage.MinIOClient
	logger          *zap.Logger
	enableCache     bool
	cacheTTL        time.Duration
	maxPerUser      int
}

// NewCompilationService creates a new compilation service
func NewCompilationService(
	compilationRepo *repository.CompilationRepository,
	queue *queue.RedisQueue,
	redisClient *redis.Client,
	minioClient *storage.MinIOClient,
	logger *zap.Logger,
	enableCache bool,
	cacheTTL time.Duration,
	maxPerUser int,
) *CompilationService {
	return &CompilationService{
		compilationRepo: compilationRepo,
		queue:           queue,
		redisClient:     redisClient,
		minioClient:     minioClient,
		logger:          logger,
		enableCache:     enableCache,
		cacheTTL:        cacheTTL,
		maxPerUser:      maxPerUser,
	}
}

// RequestCompilation requests a new compilation
func (s *CompilationService) RequestCompilation(
	ctx context.Context,
	projectID, userID primitive.ObjectID,
	compiler, mainFile string,
	files map[string]string,
) (*models.Compilation, error) {
	// Validate compiler
	if compiler == "" {
		compiler = "pdflatex"
	}
	if !isValidCompiler(compiler) {
		return nil, fmt.Errorf("invalid compiler: %s", compiler)
	}

	// Check user's active compilations
	activeCount, err := s.compilationRepo.CountActiveByUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to count active compilations", zap.Error(err))
	} else if activeCount >= int64(s.maxPerUser) {
		return nil, fmt.Errorf("maximum concurrent compilations reached (%d)", s.maxPerUser)
	}

	// Calculate input hash for caching
	inputHash := worker.CalculateInputHash(files, compiler, mainFile)

	// Check cache if enabled
	if s.enableCache {
		if cached, err := s.checkCache(ctx, inputHash); err == nil && cached != nil {
			s.logger.Info("Cache hit for compilation",
				zap.String("input_hash", inputHash),
				zap.String("cached_id", cached.ID.Hex()),
			)

			// Create new compilation record pointing to cached result
			compilation := &models.Compilation{
				ProjectID:     projectID,
				UserID:        userID,
				Status:        models.StatusCompleted,
				Compiler:      compiler,
				MainFile:      mainFile,
				InputHash:     inputHash,
				OutputFileKey: cached.OutputFileKey,
				LogFileKey:    cached.LogFileKey,
				CachedResult:  true,
				DurationMs:    0, // Instant from cache
			}

			if err := s.compilationRepo.Create(ctx, compilation); err != nil {
				return nil, err
			}

			// Generate presigned URLs
			if compilation.OutputFileKey != "" {
				url, _ := s.minioClient.GeneratePresignedURL(ctx, compilation.OutputFileKey, 1*time.Hour)
				compilation.OutputURL = url
			}

			return compilation, nil
		}
	}

	// Create compilation record
	compilation := &models.Compilation{
		ProjectID: projectID,
		UserID:    userID,
		Status:    models.StatusQueued,
		Compiler:  compiler,
		MainFile:  mainFile,
		InputHash: inputHash,
	}

	if err := s.compilationRepo.Create(ctx, compilation); err != nil {
		return nil, fmt.Errorf("failed to create compilation: %w", err)
	}

	// Enqueue compilation job
	job := &models.CompilationJob{
		CompilationID: compilation.ID.Hex(),
		ProjectID:     projectID.Hex(),
		UserID:        userID.Hex(),
		Compiler:      compiler,
		MainFile:      mainFile,
		InputHash:     inputHash,
		Files:         files,
		Priority:      0,
	}

	if err := s.queue.Enqueue(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	s.logger.Info("Compilation queued",
		zap.String("compilation_id", compilation.ID.Hex()),
		zap.String("compiler", compiler),
		zap.String("main_file", mainFile),
	)

	return compilation, nil
}

// GetCompilation retrieves a compilation by ID
func (s *CompilationService) GetCompilation(ctx context.Context, compilationID primitive.ObjectID, userID primitive.ObjectID) (*models.Compilation, error) {
	compilation, err := s.compilationRepo.FindByID(ctx, compilationID)
	if err != nil {
		return nil, err
	}

	// Check if user owns the compilation
	if compilation.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	// Generate presigned URLs if compilation is complete
	if compilation.Status == models.StatusCompleted && compilation.OutputFileKey != "" {
		url, err := s.minioClient.GeneratePresignedURL(ctx, compilation.OutputFileKey, 1*time.Hour)
		if err == nil {
			compilation.OutputURL = url
		}
	}

	if compilation.LogFileKey != "" {
		logURL, err := s.minioClient.GeneratePresignedURL(ctx, compilation.LogFileKey, 1*time.Hour)
		if err == nil {
			// Store in a temporary field for response
			compilation.OutputURL = compilation.OutputURL // Keep PDF URL
			// Log URL would be returned separately
			_ = logURL
		}
	}

	return compilation, nil
}

// ListProjectCompilations lists compilations for a project
func (s *CompilationService) ListProjectCompilations(ctx context.Context, projectID primitive.ObjectID, limit int) ([]*models.Compilation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	compilations, err := s.compilationRepo.FindByProjectID(ctx, projectID, limit)
	if err != nil {
		return nil, err
	}

	// Generate presigned URLs for completed compilations
	for _, compilation := range compilations {
		if compilation.Status == models.StatusCompleted && compilation.OutputFileKey != "" {
			url, err := s.minioClient.GeneratePresignedURL(ctx, compilation.OutputFileKey, 1*time.Hour)
			if err == nil {
				compilation.OutputURL = url
			}
		}
	}

	return compilations, nil
}

// GetStats retrieves compilation statistics
func (s *CompilationService) GetStats(ctx context.Context, since time.Duration) (*models.CompilationStats, error) {
	sinceTime := time.Now().Add(-since)
	return s.compilationRepo.GetStats(ctx, sinceTime)
}

// GetQueueStats retrieves queue statistics
func (s *CompilationService) GetQueueStats(ctx context.Context) (*models.QueueStats, error) {
	queueLength, err := s.queue.GetQueueLength(ctx)
	if err != nil {
		return nil, err
	}

	return &models.QueueStats{
		QueueLength:   queueLength,
		ActiveWorkers: 0, // Would be populated from worker registry
		TotalWorkers:  0,
	}, nil
}

// checkCache checks if a cached compilation exists
func (s *CompilationService) checkCache(ctx context.Context, inputHash string) (*models.Compilation, error) {
	// Check Redis cache first
	cacheKey := fmt.Sprintf("compilation_cache:%s", inputHash)
	cachedID, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		// Found in Redis cache
		compilationID, err := primitive.ObjectIDFromHex(cachedID)
		if err == nil {
			compilation, err := s.compilationRepo.FindByID(ctx, compilationID)
			if err == nil && compilation.Status == models.StatusCompleted {
				return compilation, nil
			}
		}
	}

	// Check MongoDB
	compilation, err := s.compilationRepo.FindByInputHash(ctx, inputHash)
	if err != nil {
		return nil, err
	}

	if compilation != nil {
		// Store in Redis cache for faster lookup next time
		s.redisClient.Set(ctx, cacheKey, compilation.ID.Hex(), s.cacheTTL)
	}

	return compilation, nil
}

func isValidCompiler(compiler string) bool {
	validCompilers := map[string]bool{
		"pdflatex":  true,
		"xelatex":   true,
		"lualatex":  true,
	}
	return validCompilers[compiler]
}
