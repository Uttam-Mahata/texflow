package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"compilation/internal/models"
	"go.uber.org/zap"
)

const (
	compilationStream = "compilation_queue"
	consumerGroup     = "compilation_workers"
)

// RedisQueue manages the compilation job queue using Redis Streams
type RedisQueue struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewRedisQueue creates a new Redis queue
func NewRedisQueue(redisClient *redis.Client, logger *zap.Logger) *RedisQueue {
	return &RedisQueue{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Initialize initializes the queue and consumer group
func (q *RedisQueue) Initialize(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	err := q.redisClient.XGroupCreateMkStream(ctx, compilationStream, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	q.logger.Info("Redis queue initialized",
		zap.String("stream", compilationStream),
		zap.String("group", consumerGroup),
	)

	return nil
}

// Enqueue adds a compilation job to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, job *models.CompilationJob) error {
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	_, err = q.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: compilationStream,
		Values: map[string]interface{}{
			"job":      jobData,
			"priority": job.Priority,
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	q.logger.Debug("Job enqueued",
		zap.String("compilation_id", job.CompilationID),
		zap.Int("priority", job.Priority),
	)

	return nil
}

// Dequeue retrieves a job from the queue
func (q *RedisQueue) Dequeue(ctx context.Context, workerID string, timeout time.Duration) (*models.CompilationJob, string, error) {
	streams, err := q.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: workerID,
		Streams:  []string{compilationStream, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, "", nil // No messages available
		}
		return nil, "", fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, "", nil
	}

	message := streams[0].Messages[0]
	jobDataStr, ok := message.Values["job"].(string)
	if !ok {
		return nil, "", fmt.Errorf("invalid job data format")
	}

	var job models.CompilationJob
	if err := json.Unmarshal([]byte(jobDataStr), &job); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, message.ID, nil
}

// Acknowledge acknowledges that a job has been processed
func (q *RedisQueue) Acknowledge(ctx context.Context, messageID string) error {
	return q.redisClient.XAck(ctx, compilationStream, consumerGroup, messageID).Err()
}

// GetQueueLength returns the current queue length
func (q *RedisQueue) GetQueueLength(ctx context.Context) (int64, error) {
	info, err := q.redisClient.XInfoStream(ctx, compilationStream).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	return info.Length, nil
}

// GetPendingCount returns the count of pending messages for a consumer
func (q *RedisQueue) GetPendingCount(ctx context.Context, consumerID string) (int64, error) {
	pending, err := q.redisClient.XPending(ctx, compilationStream, consumerGroup).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	return pending.Count, nil
}
