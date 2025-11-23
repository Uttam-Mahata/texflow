package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/texflow/services/compilation/internal/models"
	"github.com/texflow/services/compilation/internal/queue"
	"github.com/texflow/services/compilation/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// Manager coordinates compilation workers
type Manager struct {
	queue              *queue.RedisQueue
	repo               *repository.CompilationRepository
	dockerWorker       *DockerWorker
	logger             *zap.Logger
	numWorkers         int
	shutdownChan       chan struct{}
	wg                 sync.WaitGroup
	dequeueTimeout     time.Duration
}

// NewManager creates a new worker manager
func NewManager(
	queue *queue.RedisQueue,
	repo *repository.CompilationRepository,
	dockerWorker *DockerWorker,
	logger *zap.Logger,
	numWorkers int,
) *Manager {
	return &Manager{
		queue:          queue,
		repo:           repo,
		dockerWorker:   dockerWorker,
		logger:         logger,
		numWorkers:     numWorkers,
		shutdownChan:   make(chan struct{}),
		dequeueTimeout: 5 * time.Second,
	}
}

// Start starts the worker pool
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting worker manager",
		zap.Int("num_workers", m.numWorkers),
	)

	// Start worker goroutines
	for i := 0; i < m.numWorkers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		m.wg.Add(1)
		go m.runWorker(ctx, workerID)
	}

	m.logger.Info("Worker manager started successfully")
	return nil
}

// Shutdown gracefully shuts down all workers
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down worker manager")

	// Signal all workers to stop
	close(m.shutdownChan)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("All workers shut down gracefully")
		return nil
	case <-ctx.Done():
		m.logger.Warn("Worker shutdown timeout exceeded")
		return ctx.Err()
	}
}

// runWorker runs a single worker that processes jobs from the queue
func (m *Manager) runWorker(ctx context.Context, workerID string) {
	defer m.wg.Done()

	m.logger.Info("Worker started", zap.String("worker_id", workerID))

	for {
		select {
		case <-m.shutdownChan:
			m.logger.Info("Worker shutting down", zap.String("worker_id", workerID))
			return
		case <-ctx.Done():
			m.logger.Info("Worker context cancelled", zap.String("worker_id", workerID))
			return
		default:
			// Try to dequeue a job
			job, messageID, err := m.queue.Dequeue(ctx, workerID, m.dequeueTimeout)
			if err != nil {
				m.logger.Error("Failed to dequeue job",
					zap.String("worker_id", workerID),
					zap.Error(err),
				)
				time.Sleep(1 * time.Second)
				continue
			}

			// No job available
			if job == nil {
				continue
			}

			// Process the job
			m.processJob(ctx, workerID, job, messageID)
		}
	}
}

// processJob processes a single compilation job
func (m *Manager) processJob(ctx context.Context, workerID string, job *models.CompilationJob, messageID string) {
	m.logger.Info("Processing compilation job",
		zap.String("worker_id", workerID),
		zap.String("compilation_id", job.CompilationID),
		zap.String("compiler", job.Compiler),
	)

	compilationID, err := primitive.ObjectIDFromHex(job.CompilationID)
	if err != nil {
		m.logger.Error("Invalid compilation ID",
			zap.String("compilation_id", job.CompilationID),
			zap.Error(err),
		)
		m.queue.Acknowledge(ctx, messageID)
		return
	}

	// Update status to running
	if err := m.repo.UpdateStatus(ctx, compilationID, models.StatusRunning, "", ""); err != nil {
		m.logger.Error("Failed to update compilation status to running",
			zap.String("compilation_id", job.CompilationID),
			zap.Error(err),
		)
		// Don't acknowledge - let it retry
		return
	}

	// Run compilation
	result, err := m.dockerWorker.Compile(ctx, job)

	// Update compilation record based on result
	if err != nil {
		m.logger.Error("Compilation failed",
			zap.String("compilation_id", job.CompilationID),
			zap.Error(err),
		)

		updateErr := m.repo.UpdateStatus(ctx, compilationID, models.StatusFailed, "", err.Error())
		if updateErr != nil {
			m.logger.Error("Failed to update compilation status to failed",
				zap.String("compilation_id", job.CompilationID),
				zap.Error(updateErr),
			)
		}
	} else {
		m.logger.Info("Compilation completed successfully",
			zap.String("compilation_id", job.CompilationID),
			zap.String("output_url", result.OutputURL),
			zap.Duration("duration", result.Duration),
		)

		updateErr := m.repo.UpdateStatus(
			ctx,
			compilationID,
			models.StatusCompleted,
			result.OutputURL,
			result.Log,
		)
		if updateErr != nil {
			m.logger.Error("Failed to update compilation status to completed",
				zap.String("compilation_id", job.CompilationID),
				zap.Error(updateErr),
			)
		}

		// Update metrics
		if updateErr := m.repo.UpdateMetrics(ctx, compilationID, result.Duration, result.LogSize, result.OutputSize); updateErr != nil {
			m.logger.Error("Failed to update compilation metrics",
				zap.String("compilation_id", job.CompilationID),
				zap.Error(updateErr),
			)
		}
	}

	// Acknowledge the message
	if err := m.queue.Acknowledge(ctx, messageID); err != nil {
		m.logger.Error("Failed to acknowledge message",
			zap.String("message_id", messageID),
			zap.Error(err),
		)
	}
}
