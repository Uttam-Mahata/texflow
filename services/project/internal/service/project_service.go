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

	// Create default main.tex with escaped project name
	escapedTitle := escapeLatex(project.Name)
	defaultContent := fmt.Sprintf(`\documentclass{article}
\usepackage[utf8]{inputenc}

\title{%s}
\author{TexFlow User}
\date{\today}

\begin{document}

\maketitle

\section{Introduction}
Start writing your document here.

\end{document}`, escapedTitle)

	fileReq := &models.CreateFileRequest{
		Name:     "main.tex",
		Path:     "/main.tex",
		Content:  []byte(defaultContent),
		IsBinary: false,
	}

	_, err := s.CreateFile(ctx, project.ID, userID, fileReq)
	if err != nil {
		s.logger.Error("Failed to create default main.tex", zap.Error(err))
		// Clean up project if file creation fails
		if delErr := s.projectRepo.Delete(ctx, project.ID); delErr != nil {
			s.logger.Error("Failed to delete project after file creation failure", zap.Error(delErr))
		}
		return nil, fmt.Errorf("failed to create default project files: %w", err)
	}

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

// ListFiles retrieves all files for a project (metadata only)
func (s *ProjectService) ListFiles(ctx context.Context, projectID, userID primitive.ObjectID) ([]*models.File, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if !s.userHasAccess(project, userID) {
		return nil, fmt.Errorf("access denied")
	}

	files, err := s.fileRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if files == nil {
		files = []*models.File{}
	}
	return files, nil
}

// GetFileMetadata retrieves file metadata
func (s *ProjectService) GetFileMetadata(ctx context.Context, fileID, userID primitive.ObjectID) (*models.File, error) {
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, file.ProjectID)
	if err != nil {
		return nil, err
	}

	if !s.userHasAccess(project, userID) {
		return nil, fmt.Errorf("access denied")
	}

	return file, nil
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
	// Remove leading slash for storage key
	cleanPath = strings.TrimPrefix(cleanPath, "/")

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

// UpdateFile updates a file's content
func (s *ProjectService) UpdateFile(ctx context.Context, fileID, userID primitive.ObjectID, req *models.UpdateFileRequest) (*models.File, error) {
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}

	project, err := s.projectRepo.FindByID(ctx, file.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	// Check edit access
	if !s.userCanEdit(project, userID) {
		return nil, fmt.Errorf("permission denied")
	}

	// Convert string content to bytes
	contentBytes := []byte(req.Content)

	// Calculate new hash
	hash := fmt.Sprintf("%x", sha256.Sum256(contentBytes))

	// Upload to MinIO (overwrites existing file)
	if err := s.minioClient.UploadBytes(ctx, file.StorageKey, contentBytes, file.ContentType); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	// Update file record
	file.SizeBytes = int64(len(contentBytes))
	file.Hash = hash
	// Note: Version is incremented by fileRepo.Update()

	if err := s.fileRepo.Update(ctx, file); err != nil {
		return nil, err
	}

	// Update project stats
	s.updateProjectFileStats(ctx, file.ProjectID)

	return file, nil
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

// escapeLatex escapes special LaTeX characters in a string
func escapeLatex(s string) string {
	// LaTeX special characters that need escaping: \ # $ % & _ { } ~ ^
	replacer := strings.NewReplacer(
		"\\", "\\textbackslash{}",
		"#", "\\#",
		"$", "\\$",
		"%", "\\%",
		"&", "\\&",
		"_", "\\_",
		"{", "\\{",
		"}", "\\}",
		"~", "\\textasciitilde{}",
		"^", "\\textasciicircum{}",
	)
	return replacer.Replace(s)
}

// fixLatexTitle finds \title{...} in LaTeX content and escapes special characters
func fixLatexTitle(content string) string {
	// Find \title{...} pattern
	titleStart := strings.Index(content, "\\title{")
	if titleStart == -1 {
		return content // No title found
	}

	// Find the matching closing brace
	braceCount := 0
	titleContentStart := titleStart + 7 // Length of "\title{"
	titleEnd := -1

	for i := titleContentStart; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			if braceCount == 0 {
				titleEnd = i
				break
			}
			braceCount--
		}
	}

	if titleEnd == -1 {
		return content // No closing brace found
	}

	// Extract title text
	titleText := content[titleContentStart:titleEnd]

	// Check if already escaped (contains \_ or \\)
	if strings.Contains(titleText, "\\_") || strings.Contains(titleText, "\\#") {
		return content // Already escaped, don't double-escape
	}

	// Escape the title text
	escapedTitle := escapeLatex(titleText)

	// Reconstruct the content with escaped title
	return content[:titleContentStart] + escapedTitle + content[titleEnd:]
}

// MigrateLatexFiles fixes existing main.tex files with unescaped LaTeX characters
func (s *ProjectService) MigrateLatexFiles(ctx context.Context) (int, []string, error) {
	// Find all main.tex files
	files, err := s.fileRepo.FindByName(ctx, "main.tex")
	if err != nil {
		return 0, nil, fmt.Errorf("failed to find main.tex files: %w", err)
	}

	s.logger.Info("Starting LaTeX migration",
		zap.Int("file_count", len(files)),
	)

	fixed := 0
	errors := []string{}

	for _, file := range files {
		// Download file content from MinIO
		content, err := s.minioClient.DownloadBytes(ctx, file.StorageKey)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to download %s: %v", file.StorageKey, err)
			s.logger.Error(errMsg)
			errors = append(errors, errMsg)
			continue
		}

		// Fix LaTeX title
		originalContent := string(content)
		fixedContent := fixLatexTitle(originalContent)

		// Check if content changed
		if fixedContent == originalContent {
			s.logger.Debug("File already escaped or no title found",
				zap.String("file_id", file.ID.Hex()),
				zap.String("storage_key", file.StorageKey),
			)
			continue
		}

		// Upload fixed content back to MinIO
		err = s.minioClient.UploadBytes(ctx, file.StorageKey, []byte(fixedContent), "text/x-tex")
		if err != nil {
			errMsg := fmt.Sprintf("Failed to upload %s: %v", file.StorageKey, err)
			s.logger.Error(errMsg)
			errors = append(errors, errMsg)
			continue
		}

		// Update file hash and version
		file.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(fixedContent)))
		file.SizeBytes = int64(len(fixedContent))

		err = s.fileRepo.Update(ctx, file)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to update file record %s: %v", file.ID.Hex(), err)
			s.logger.Error(errMsg)
			errors = append(errors, errMsg)
			continue
		}

		s.logger.Info("Fixed LaTeX file",
			zap.String("file_id", file.ID.Hex()),
			zap.String("project_id", file.ProjectID.Hex()),
		)
		fixed++
	}

	s.logger.Info("LaTeX migration completed",
		zap.Int("total_files", len(files)),
		zap.Int("fixed", fixed),
		zap.Int("errors", len(errors)),
	)

	return fixed, errors, nil
}
