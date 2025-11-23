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

// ProjectRepository handles project data persistence
type ProjectRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

// NewProjectRepository creates a new project repository
func NewProjectRepository(db *mongo.Database) *ProjectRepository {
	return &ProjectRepository{
		db:         db,
		collection: db.Collection("projects"),
	}
}

// Create creates a new project
func (r *ProjectRepository) Create(ctx context.Context, project *models.Project) error {
	project.ID = primitive.NewObjectID()
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()
	project.FileCount = 0
	project.TotalSizeBytes = 0

	_, err := r.collection.InsertOne(ctx, project)
	return err
}

// FindByID finds a project by ID
func (r *ProjectRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Project, error) {
	var project models.Project
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("project not found")
		}
		return nil, err
	}

	return &project, nil
}

// FindByOwner finds all projects owned by a user
func (r *ProjectRepository) FindByOwner(ctx context.Context, ownerID primitive.ObjectID, page, limit int) ([]*models.Project, int64, error) {
	skip := (page - 1) * limit

	filter := bson.M{"owner_id": ownerID}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Get projects
	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.D{{Key: "updated_at", Value: -1}}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var projects []*models.Project
	if err := cursor.All(ctx, &projects); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// FindSharedWithUser finds projects shared with a user
func (r *ProjectRepository) FindSharedWithUser(ctx context.Context, userID primitive.ObjectID, page, limit int) ([]*models.Project, int64, error) {
	skip := (page - 1) * limit

	filter := bson.M{
		"collaborators.user_id": userID,
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Get projects
	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.D{{Key: "updated_at", Value: -1}}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var projects []*models.Project
	if err := cursor.All(ctx, &projects); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// Update updates a project
func (r *ProjectRepository) Update(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now()

	filter := bson.M{"_id": project.ID}
	update := bson.M{"$set": project}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// AddCollaborator adds a collaborator to a project
func (r *ProjectRepository) AddCollaborator(ctx context.Context, projectID primitive.ObjectID, collaborator models.Collaborator) error {
	filter := bson.M{"_id": projectID}
	update := bson.M{
		"$push": bson.M{"collaborators": collaborator},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// RemoveCollaborator removes a collaborator from a project
func (r *ProjectRepository) RemoveCollaborator(ctx context.Context, projectID, userID primitive.ObjectID) error {
	filter := bson.M{"_id": projectID}
	update := bson.M{
		"$pull": bson.M{"collaborators": bson.M{"user_id": userID}},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// Delete deletes a project
func (r *ProjectRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// UpdateFileStats updates file count and total size for a project
func (r *ProjectRepository) UpdateFileStats(ctx context.Context, projectID primitive.ObjectID, fileCount int, totalSize int64) error {
	filter := bson.M{"_id": projectID}
	update := bson.M{
		"$set": bson.M{
			"file_count":       fileCount,
			"total_size_bytes": totalSize,
			"updated_at":       time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// CreateIndexes creates necessary indexes
func (r *ProjectRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "updated_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "collaborators.user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "tags", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
