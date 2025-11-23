package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/texflow/services/project/internal/models"
	"github.com/texflow/services/project/internal/repository"
	"github.com/texflow/services/project/internal/storage"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// ProjectService handles project business logic
type ProjectService struct {
	projectRepo *repository.ProjectRepository
	fileRepo    *repository.FileRepository
	minioClient *storage.MinIOClient
	logger      *zap.Logger
}

// NewProjectService creates a new project service
func NewProjectService(
	projectRepo *repository.ProjectRepository,
	fileRepo *repository.FileRepository,
	minioClient *storage.MinIOClient,
	logger *zap.Logger,
) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		fileRepo:    fileRepo,
		minioClient: minioClient,
		logger:      logger,
	}
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, userID primitive.ObjectID, req *models.CreateProjectRequest) (*models.Project, error) {
	compiler := req.Compiler
	if compiler == "" {
		compiler = "pdflatex"
	}

	project := &models.Project{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
		Settings: models.ProjectSettings{
			Compiler:    compiler,
			MainFile:    "main.tex",
			SpellCheck:  true,
			AutoCompile: false,
		},
		IsPublic: req.IsPublic,
		Tags:     req.Tags,
	}

	if err := s.projectRepo.Create(ctx, project); err != nil {
		return nil, err
	}

	s.logger.Info("Project created",
		zap.String("project_id", project.ID.Hex()),
		zap.String("user_id", userID.Hex()),
	)

	return project, nil
}

// GetProject retrieves a project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID primitive.ObjectID, userID primitive.ObjectID) (*models.Project, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check access
	if !s.userHasAccess(project, userID) {
		return nil, fmt.Errorf("access denied")
	}

	return project, nil
}

// ListUserProjects lists all projects owned by or shared with a user
func (s *ProjectService) ListUserProjects(ctx context.Context, userID primitive.ObjectID, page, limit int) ([]*models.Project, int64, error) {
	// Get owned projects
	ownedProjects, ownedTotal, err := s.projectRepo.FindByOwner(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	// Get shared projects
	sharedProjects, sharedTotal, err := s.projectRepo.FindSharedWithUser(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	// Combine results
	allProjects := append(ownedProjects, sharedProjects...)
	totalCount := ownedTotal + sharedTotal

	return allProjects, totalCount, nil
}

// UpdateProject updates a project
func (s *ProjectService) UpdateProject(ctx context.Context, projectID, userID primitive.ObjectID, req *models.UpdateProjectRequest) (*models.Project, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check if user is owner
	if project.OwnerID != userID {
		return nil, fmt.Errorf("only project owner can update settings")
	}

	// Update fields
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.Compiler != nil {
		project.Settings.Compiler = *req.Compiler
	}
	if req.MainFile != nil {
		project.Settings.MainFile = *req.MainFile
	}
	if req.IsPublic != nil {
		project.IsPublic = *req.IsPublic
	}
	if req.Tags != nil {
		project.Tags = req.Tags
	}

	if err := s.projectRepo.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// DeleteProject deletes a project and all its files
func (s *ProjectService) DeleteProject(ctx context.Context, projectID, userID primitive.ObjectID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	// Check if user is owner
	if project.OwnerID != userID {
		return fmt.Errorf("only project owner can delete project")
	}

	// Delete all files from MinIO
	prefix := fmt.Sprintf("projects/%s/", projectID.Hex())
	objects, err := s.minioClient.ListObjects(ctx, prefix)
	if err != nil {
		s.logger.Error("Failed to list objects for deletion", zap.Error(err))
	} else {
		for _, obj := range objects {
			if err := s.minioClient.DeleteFile(ctx, obj.Key); err != nil {
				s.logger.Error("Failed to delete object", zap.String("key", obj.Key), zap.Error(err))
			}
		}
	}

	// Delete file records
	if err := s.fileRepo.DeleteByProjectID(ctx, projectID); err != nil {
		s.logger.Error("Failed to delete file records", zap.Error(err))
	}

	// Delete project
	return s.projectRepo.Delete(ctx, projectID)
}

// ShareProject shares a project with another user
func (s *ProjectService) ShareProject(ctx context.Context, projectID, ownerID, collaboratorID primitive.ObjectID, role string) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	// Check if user is owner
	if project.OwnerID != ownerID {
		return fmt.Errorf("only project owner can share project")
	}

	// Check if already shared
	for _, collab := range project.Collaborators {
		if collab.UserID == collaboratorID {
			return fmt.Errorf("project already shared with this user")
		}
	}

	collaborator := models.Collaborator{
		UserID:    collaboratorID,
		Role:      role,
		InvitedAt: project.CreatedAt,
	}

	return s.projectRepo.AddCollaborator(ctx, projectID, collaborator)
}

// CreateFile creates a file in a project
func (s *ProjectService) CreateFile(ctx context.Context, projectID, userID primitive.ObjectID, req *models.CreateFileRequest) (*models.File, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check access
	if !s.userCanEdit(project, userID) {
		return nil, fmt.Errorf("permission denied")
	}

	// Clean and validate path
	cleanPath := filepath.Clean(req.Path)
	if strings.HasPrefix(cleanPath, "..") {
		return nil, fmt.Errorf("invalid file path")
	}

	// Calculate hash
	hash := fmt.Sprintf("%x", sha256.Sum256(req.Content))

	// Upload to MinIO
	storageKey := fmt.Sprintf("projects/%s/files/%s", projectID.Hex(), cleanPath)
	contentType := getContentType(req.Name)
	if err := s.minioClient.UploadBytes(ctx, storageKey, req.Content, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	// Create file record
	file := &models.File{
		ProjectID:   projectID,
		Name:        req.Name,
		Path:        cleanPath,
		ContentType: contentType,
		SizeBytes:   int64(len(req.Content)),
		StorageKey:  storageKey,
		CreatedBy:   userID,
		IsBinary:    req.IsBinary,
		Hash:        hash,
	}

	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil, err
	}

	// Update project stats
	s.updateProjectFileStats(ctx, projectID)

	return file, nil
}

// GetFile retrieves a file
func (s *ProjectService) GetFile(ctx context.Context, fileID, userID primitive.ObjectID) (*models.File, []byte, error) {
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, file.ProjectID)
	if err != nil {
		return nil, nil, err
	}

	// Check access
	if !s.userHasAccess(project, userID) {
		return nil, nil, fmt.Errorf("access denied")
	}

	// Download from MinIO
	content, err := s.minioClient.DownloadBytes(ctx, file.StorageKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download file: %w", err)
	}

	return file, content, nil
}

// Helper methods
func (s *ProjectService) userHasAccess(project *models.Project, userID primitive.ObjectID) bool {
	if project.OwnerID == userID {
		return true
	}

	for _, collab := range project.Collaborators {
		if collab.UserID == userID {
			return true
		}
	}

	return project.IsPublic
}

func (s *ProjectService) userCanEdit(project *models.Project, userID primitive.ObjectID) bool {
	if project.OwnerID == userID {
		return true
	}

	for _, collab := range project.Collaborators {
		if collab.UserID == userID && collab.Role == "editor" {
			return true
		}
	}

	return false
}

func (s *ProjectService) updateProjectFileStats(ctx context.Context, projectID primitive.ObjectID) {
	count, size, err := s.fileRepo.GetProjectFileStats(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get file stats", zap.Error(err))
		return
	}

	if err := s.projectRepo.UpdateFileStats(ctx, projectID, count, size); err != nil {
		s.logger.Error("Failed to update file stats", zap.Error(err))
	}
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".tex":
		return "text/x-tex"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".bib":
		return "text/x-bibtex"
	default:
		return "application/octet-stream"
	}
}
