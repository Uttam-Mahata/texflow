package service

import (
	"context"
	"fmt"
	"strings"

	"compilation/internal/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// ProjectService handles project-related operations for compilation
type ProjectService struct {
	db          *mongo.Database
	minioClient *storage.MinIOClient
	logger      *zap.Logger
}

// NewProjectService creates a new project service
func NewProjectService(db *mongo.Database, minioClient *storage.MinIOClient, logger *zap.Logger) *ProjectService {
	return &ProjectService{
		db:          db,
		minioClient: minioClient,
		logger:      logger,
	}
}

// GetProjectFiles retrieves all files for a project
func (s *ProjectService) GetProjectFiles(ctx context.Context, projectID primitive.ObjectID) (map[string]string, error) {
	// Get file list from MongoDB
	filesCollection := s.db.Collection("files")
	cursor, err := filesCollection.Find(ctx, bson.M{"project_id": projectID})
	if err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}
	defer cursor.Close(ctx)

	files := make(map[string]string)

	for cursor.Next(ctx) {
		var fileDoc struct {
			Name       string `bson:"name"`
			Path       string `bson:"path"`
			StorageKey string `bson:"storage_key"`
			IsBinary   bool   `bson:"is_binary"`
		}

		if err := cursor.Decode(&fileDoc); err != nil {
			s.logger.Error("Failed to decode file document", zap.Error(err))
			continue
		}

		// Skip binary files (images, etc.)
		if fileDoc.IsBinary {
			continue
		}

		// Download file content from MinIO
		content, err := s.minioClient.DownloadBytes(ctx, fileDoc.StorageKey)
		if err != nil {
			s.logger.Error("Failed to download file",
				zap.String("file", fileDoc.Name),
				zap.String("storage_key", fileDoc.StorageKey),
				zap.Error(err),
			)
			continue
		}

		// Use the file path as key, stripping leading slash for proper path handling
		filePath := strings.TrimPrefix(fileDoc.Path, "/")
		if filePath == "" {
			filePath = fileDoc.Name
		}
		files[filePath] = string(content)

		s.logger.Debug("File loaded for compilation",
			zap.String("path", filePath),
			zap.Int("size", len(content)),
		)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return files, nil
}
