package repository

import (
	"context"
	"time"

	"github.com/texflow/services/collaboration/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SnapshotRepository handles Yjs snapshot data persistence
type SnapshotRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewSnapshotRepository creates a new snapshot repository
func NewSnapshotRepository(db *mongo.Database) *SnapshotRepository {
	return &SnapshotRepository{
		db:         db,
		collection: db.Collection("yjs_snapshots"),
	}
}

// CreateSnapshot stores a new snapshot
func (r *SnapshotRepository) CreateSnapshot(ctx context.Context, snapshot *models.YjsSnapshot) error {
	snapshot.ID = primitive.NewObjectID()
	snapshot.CreatedAt = time.Now()
	snapshot.SizeBytes = int64(len(snapshot.StateVector) + len(snapshot.Snapshot))

	_, err := r.collection.InsertOne(ctx, snapshot)
	return err
}

// GetLatestSnapshot retrieves the latest snapshot for a document
func (r *SnapshotRepository) GetLatestSnapshot(ctx context.Context, projectID primitive.ObjectID, documentName string) (*models.YjsSnapshot, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	var snapshot models.YjsSnapshot
	err := r.collection.FindOne(ctx, filter, opts).Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &snapshot, nil
}

// GetSnapshotByVersion retrieves a snapshot by version number
func (r *SnapshotRepository) GetSnapshotByVersion(ctx context.Context, projectID primitive.ObjectID, documentName string, version int64) (*models.YjsSnapshot, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
		"version":       version,
	}

	var snapshot models.YjsSnapshot
	err := r.collection.FindOne(ctx, filter).Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &snapshot, nil
}

// ListSnapshots lists all snapshots for a document
func (r *SnapshotRepository) ListSnapshots(ctx context.Context, projectID primitive.ObjectID, documentName string, limit int) ([]*models.YjsSnapshot, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "version", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var snapshots []*models.YjsSnapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

// CountSnapshots counts total snapshots for a document
func (r *SnapshotRepository) CountSnapshots(ctx context.Context, projectID primitive.ObjectID, documentName string) (int64, error) {
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}

	return r.collection.CountDocuments(ctx, filter)
}

// DeleteOldSnapshots deletes snapshots older than the specified date, keeping at least the latest N
func (r *SnapshotRepository) DeleteOldSnapshots(ctx context.Context, projectID primitive.ObjectID, documentName string, olderThan time.Time, keepLatest int) (int64, error) {
	// Get the latest N snapshot IDs to keep
	opts := options.Find().
		SetSort(bson.D{{Key: "version", Value: -1}}).
		SetLimit(int64(keepLatest)).
		SetProjection(bson.M{"_id": 1})

	cursor, err := r.collection.Find(ctx, bson.M{
		"project_id":    projectID,
		"document_name": documentName,
	}, opts)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var keepIDs []primitive.ObjectID
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		keepIDs = append(keepIDs, doc.ID)
	}

	// Delete old snapshots except the ones to keep
	filter := bson.M{
		"project_id":    projectID,
		"document_name": documentName,
		"created_at":    bson.M{"$lt": olderThan},
		"_id":           bson.M{"$nin": keepIDs},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// CreateIndexes creates necessary indexes for the collection
func (r *SnapshotRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "document_name", Value: 1},
				{Key: "version", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
