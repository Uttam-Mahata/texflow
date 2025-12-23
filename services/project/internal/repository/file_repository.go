package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/texflow/services/project/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FileRepository handles file data persistence
type FileRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *mongo.Database) *FileRepository {
	return &FileRepository{
		db:         db,
		collection: db.Collection("files"),
	}
}

// Create creates a new file record
func (r *FileRepository) Create(ctx context.Context, file *models.File) error {
	file.ID = primitive.NewObjectID()
	file.CreatedAt = time.Now()
	file.UpdatedAt = time.Now()
	file.Version = 1

	_, err := r.collection.InsertOne(ctx, file)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("file already exists at this path")
		}
		return err
	}

	return nil
}

// FindByID finds a file by ID
func (r *FileRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.File, error) {
	var file models.File
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&file)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}

	return &file, nil
}

// FindByProjectID finds all files in a project
func (r *FileRepository) FindByProjectID(ctx context.Context, projectID primitive.ObjectID) ([]*models.File, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"project_id": projectID},
		options.Find().SetSort(bson.D{{Key: "path", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var files []*models.File
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// FindByPath finds a file by project ID and path
func (r *FileRepository) FindByPath(ctx context.Context, projectID primitive.ObjectID, path string) (*models.File, error) {
	var file models.File
	err := r.collection.FindOne(ctx, bson.M{
		"project_id": projectID,
		"path":       path,
	}).Decode(&file)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}

	return &file, nil
}

// FindByName finds all files with a specific name across all projects
func (r *FileRepository) FindByName(ctx context.Context, name string) ([]*models.File, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"name": name},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var files []*models.File
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// Update updates a file record
func (r *FileRepository) Update(ctx context.Context, file *models.File) error {
	file.UpdatedAt = time.Now()
	file.Version++

	filter := bson.M{"_id": file.ID}
	update := bson.M{"$set": file}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("file not found")
	}

	return nil
}

// Delete deletes a file record
func (r *FileRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("file not found")
	}

	return nil
}

// DeleteByProjectID deletes all files in a project
func (r *FileRepository) DeleteByProjectID(ctx context.Context, projectID primitive.ObjectID) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{"project_id": projectID})
	return err
}

// GetProjectFileStats returns file count and total size for a project
func (r *FileRepository) GetProjectFileStats(ctx context.Context, projectID primitive.ObjectID) (int, int64, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"project_id": projectID}},
		{"$group": bson.M{
			"_id":        nil,
			"count":      bson.M{"$sum": 1},
			"total_size": bson.M{"$sum": "$size_bytes"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	var result struct {
		Count     int   `bson:"count"`
		TotalSize int64 `bson:"total_size"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, 0, err
		}
		return result.Count, result.TotalSize, nil
	}

	return 0, 0, nil
}

// CreateIndexes creates necessary indexes
func (r *FileRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "project_id", Value: 1}, {Key: "path", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "hash", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
