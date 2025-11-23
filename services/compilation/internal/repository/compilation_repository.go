package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/texflow/services/compilation/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CompilationRepository handles compilation data persistence
type CompilationRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewCompilationRepository creates a new compilation repository
func NewCompilationRepository(db *mongo.Database) *CompilationRepository {
	return &CompilationRepository{
		db:         db,
		collection: db.Collection("compilations"),
	}
}

// Create creates a new compilation record
func (r *CompilationRepository) Create(ctx context.Context, compilation *models.Compilation) error {
	compilation.ID = primitive.NewObjectID()
	compilation.CreatedAt = time.Now()
	compilation.UpdatedAt = time.Now()
	compilation.Status = models.StatusQueued

	_, err := r.collection.InsertOne(ctx, compilation)
	return err
}

// FindByID finds a compilation by ID
func (r *CompilationRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Compilation, error) {
	var compilation models.Compilation
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&compilation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("compilation not found")
		}
		return nil, err
	}

	return &compilation, nil
}

// FindByProjectID finds compilations for a project
func (r *CompilationRepository) FindByProjectID(ctx context.Context, projectID primitive.ObjectID, limit int) ([]*models.Compilation, error) {
	filter := bson.M{"project_id": projectID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var compilations []*models.Compilation
	if err := cursor.All(ctx, &compilations); err != nil {
		return nil, err
	}

	return compilations, nil
}

// FindByInputHash finds a compilation by input hash (for caching)
func (r *CompilationRepository) FindByInputHash(ctx context.Context, inputHash string) (*models.Compilation, error) {
	filter := bson.M{
		"input_hash": inputHash,
		"status":     models.StatusCompleted,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})

	var compilation models.Compilation
	err := r.collection.FindOne(ctx, filter, opts).Decode(&compilation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &compilation, nil
}

// UpdateStatus updates the status of a compilation
func (r *CompilationRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.CompilationStatus) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == models.StatusRunning {
		now := time.Now()
		update = bson.M{
			"$set": bson.M{
				"status":     status,
				"started_at": now,
				"updated_at": now,
			},
		}
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("compilation not found")
	}

	return nil
}

// UpdateResult updates the result of a compilation
func (r *CompilationRepository) UpdateResult(ctx context.Context, id primitive.ObjectID, result *models.CompilationResult) error {
	now := time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":          result.Status,
			"output_file_key": result.OutputURL,
			"log_file_key":    result.LogURL,
			"error_message":   result.ErrorMessage,
			"duration_ms":     result.DurationMs,
			"cached_result":   result.CachedResult,
			"completed_at":    now,
			"updated_at":      now,
		},
	}

	updateResult, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if updateResult.MatchedCount == 0 {
		return fmt.Errorf("compilation not found")
	}

	return nil
}

// CountActiveByUser counts active compilations for a user
func (r *CompilationRepository) CountActiveByUser(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"user_id": userID,
		"status": bson.M{
			"$in": []models.CompilationStatus{models.StatusQueued, models.StatusRunning},
		},
	}

	return r.collection.CountDocuments(ctx, filter)
}

// GetStats returns compilation statistics
func (r *CompilationRepository) GetStats(ctx context.Context, since time.Time) (*models.CompilationStats, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": since},
			},
		},
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": 1},
				"status_counts": bson.M{
					"$push": "$status",
				},
				"avg_duration": bson.M{
					"$avg": "$duration_ms",
				},
				"cached_count": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{"$cached_result", 1, 0},
					},
				},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		Total        int64                       `bson:"total"`
		StatusCounts []models.CompilationStatus  `bson:"status_counts"`
		AvgDuration  float64                     `bson:"avg_duration"`
		CachedCount  int64                       `bson:"cached_count"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.CompilationStats{
			StatusCounts: make(map[models.CompilationStatus]int64),
		}, nil
	}

	// Count statuses
	statusCounts := make(map[models.CompilationStatus]int64)
	for _, status := range results[0].StatusCounts {
		statusCounts[status]++
	}

	// Calculate cache hit rate
	cacheHitRate := 0.0
	if results[0].Total > 0 {
		cacheHitRate = float64(results[0].CachedCount) / float64(results[0].Total) * 100
	}

	return &models.CompilationStats{
		TotalCompilations: results[0].Total,
		StatusCounts:      statusCounts,
		AvgDurationMs:     results[0].AvgDuration,
		CacheHitRate:      cacheHitRate,
	}, nil
}

// CreateIndexes creates necessary indexes
func (r *CompilationRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "status", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "input_hash", Value: 1},
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
