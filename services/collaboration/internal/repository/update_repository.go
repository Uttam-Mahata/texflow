package repository

import (
	"context"
	"time"

	"collaboration/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UpdateRepository handles Yjs update data persistence
type UpdateRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewUpdateRepository creates a new update repository
func NewUpdateRepository(db *mongo.Database) *UpdateRepository {
	return &UpdateRepository{
		db:         db,
		collection: db.Collection("yjs_updates"),
	}
}

// StoreUpdate stores a Yjs update
func (r *UpdateRepository) StoreUpdate(ctx context.Context, update *models.YjsUpdate) error {
	update.ID = primitive.NewObjectID()
	update.CreatedAt = time.Now()
	update.SizeBytes = len(update.Update)

	_, err := r.collection.InsertOne(ctx, update)
	return err
}

// GetUpdatesSince retrieves updates since a specific version
func (r *UpdateRepository) GetUpdatesSince(ctx context.Context, projectID primitive.ObjectID, documentName string, sinceVersion int64, limit int) ([]*models.YjsUpdate, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
		"version":       bson.M{"$gt": sinceVersion},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "version", Value: 1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var updates []*models.YjsUpdate
	if err := cursor.All(ctx, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

// GetAllUpdates retrieves all updates for a document
func (r *UpdateRepository) GetAllUpdates(ctx context.Context, projectID primitive.ObjectID, documentName string) ([]*models.YjsUpdate, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	opts := options.Find().SetSort(bson.D{{Key: "version", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var updates []*models.YjsUpdate
	if err := cursor.All(ctx, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

// GetLatestVersion gets the latest version number for a document
func (r *UpdateRepository) GetLatestVersion(ctx context.Context, projectID primitive.ObjectID, documentName string) (int64, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	var update models.YjsUpdate
	err := r.collection.FindOne(ctx, filter, opts).Decode(&update)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}

	return update.Version, nil
}

// CountUpdates counts total updates for a document
func (r *UpdateRepository) CountUpdates(ctx context.Context, projectID primitive.ObjectID, documentName string) (int64, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	return r.collection.CountDocuments(ctx, filter)
}

// CountUpdatesSinceSnapshot counts updates since a specific version
func (r *UpdateRepository) CountUpdatesSinceSnapshot(ctx context.Context, projectID primitive.ObjectID, documentName string, snapshotVersion int64) (int64, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
		"version":       bson.M{"$gt": snapshotVersion},
	}

	return r.collection.CountDocuments(ctx, filter)
}

// DeleteOldUpdates deletes updates older than the specified date
func (r *UpdateRepository) DeleteOldUpdates(ctx context.Context, olderThan time.Time) (int64, error) {
	filter := bson.M{
		"created_at": bson.M{"$lt": olderThan},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// GetUpdateStats returns statistics about updates
func (r *UpdateRepository) GetUpdateStats(ctx context.Context, projectID primitive.ObjectID, documentName string) (*models.DocumentMetrics, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"project_id":    projectID,
				"document_name": documentName,
			},
		},
		{
			"$group": bson.M{
				"_id":            "$project_id",
				"total_updates":  bson.M{"$sum": 1},
				"current_version": bson.M{"$max": "$version"},
				"last_updated":   bson.M{"$max": "$created_at"},
				"contributors":   bson.M{"$addToSet": "$user_id"},
				"total_size":     bson.M{"$sum": "$size_bytes"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID             primitive.ObjectID   `bson:"_id"`
		TotalUpdates   int64                `bson:"total_updates"`
		CurrentVersion int64                `bson:"current_version"`
		LastUpdated    time.Time            `bson:"last_updated"`
		Contributors   []primitive.ObjectID `bson:"contributors"`
		TotalSize      int64                `bson:"total_size"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.DocumentMetrics{
			ProjectID:    projectID,
			DocumentName: documentName,
		}, nil
	}

	return &models.DocumentMetrics{
		ProjectID:      results[0].ID,
		DocumentName:   documentName,
		TotalUpdates:   results[0].TotalUpdates,
		CurrentVersion: results[0].CurrentVersion,
		LastUpdated:    results[0].LastUpdated,
		Contributors:   results[0].Contributors,
		SizeBytes:      results[0].TotalSize,
	}, nil
}

// CreateIndexes creates necessary indexes for the collection
func (r *UpdateRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "document_name", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "document_name", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(2592000), // 30 days TTL
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
