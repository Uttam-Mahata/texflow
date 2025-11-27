package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"collaboration/internal/models"
	"collaboration/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// CollaborationService handles collaboration business logic
type CollaborationService struct {
	updateRepo       *repository.UpdateRepository
	snapshotRepo     *repository.SnapshotRepository
	logger           *zap.Logger
	snapshotInterval int
	maxUpdatesPerFetch int

	// Version counters per document (thread-safe)
	versionCounters map[string]*versionCounter
	mu              sync.RWMutex
}

type versionCounter struct {
	currentVersion int64
	mu             sync.Mutex
}

// NewCollaborationService creates a new collaboration service
func NewCollaborationService(
	updateRepo *repository.UpdateRepository,
	snapshotRepo *repository.SnapshotRepository,
	logger *zap.Logger,
	snapshotInterval int,
	maxUpdatesPerFetch int,
) *CollaborationService {
	return &CollaborationService{
		updateRepo:         updateRepo,
		snapshotRepo:       snapshotRepo,
		logger:             logger,
		snapshotInterval:   snapshotInterval,
		maxUpdatesPerFetch: maxUpdatesPerFetch,
		versionCounters:    make(map[string]*versionCounter),
	}
}

// StoreUpdate stores a Yjs update and handles snapshot creation
func (s *CollaborationService) StoreUpdate(ctx context.Context, projectID primitive.ObjectID, documentName string, updateData []byte, userID primitive.ObjectID, clientID string) (*models.YjsUpdate, error) {
	// Validate update size
	if len(updateData) == 0 {
		return nil, fmt.Errorf("update data is empty")
	}

	// Get or create version counter for this document
	docKey := fmt.Sprintf("%s:%s", projectID.Hex(), documentName)
	counter := s.getVersionCounter(docKey)

	// Get next version number (thread-safe)
	counter.mu.Lock()
	if counter.currentVersion == 0 {
		// Initialize from database
		latestVersion, err := s.updateRepo.GetLatestVersion(ctx, projectID, documentName)
		if err != nil {
			counter.mu.Unlock()
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
		counter.currentVersion = latestVersion
	}
	counter.currentVersion++
	version := counter.currentVersion
	counter.mu.Unlock()

	// Create update object
	update := &models.YjsUpdate{
		ProjectID:    projectID,
		DocumentName: documentName,
		Update:       updateData,
		Clock:        time.Now().UnixNano(),
		Version:      version,
		UserID:       userID,
		ClientID:     clientID,
	}

	// Store update
	if err := s.updateRepo.StoreUpdate(ctx, update); err != nil {
		// Rollback version counter
		counter.mu.Lock()
		counter.currentVersion--
		counter.mu.Unlock()
		return nil, fmt.Errorf("failed to store update: %w", err)
	}

	s.logger.Debug("Stored Yjs update",
		zap.String("project_id", projectID.Hex()),
		zap.String("document", documentName),
		zap.Int64("version", version),
		zap.Int("size", len(updateData)),
	)

	// Check if we need to create a snapshot
	if version%int64(s.snapshotInterval) == 0 {
		go s.createSnapshotAsync(projectID, documentName, version)
	}

	return update, nil
}

// GetDocumentState retrieves the current state of a document
func (s *CollaborationService) GetDocumentState(ctx context.Context, projectID primitive.ObjectID, documentName string, sinceVersion int64) (*models.DocumentStateResponse, error) {
	// Get latest snapshot
	snapshot, err := s.snapshotRepo.GetLatestSnapshot(ctx, projectID, documentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	response := &models.DocumentStateResponse{}

	if snapshot != nil {
		// We have a snapshot
		response.Version = snapshot.Version

		// If client needs updates since a specific version
		if sinceVersion > 0 {
			if sinceVersion < snapshot.Version {
				// Client is too far behind, send snapshot + updates after snapshot
				response.Snapshot = encodeBase64(snapshot.Snapshot)
				response.StateVector = encodeBase64(snapshot.StateVector)

				// Get updates after snapshot
				updates, err := s.updateRepo.GetUpdatesSince(ctx, projectID, documentName, snapshot.Version, s.maxUpdatesPerFetch)
				if err != nil {
					return nil, fmt.Errorf("failed to get updates: %w", err)
				}
				response.Updates = s.convertUpdates(updates)
			} else {
				// Client is recent, just send missing updates
				updates, err := s.updateRepo.GetUpdatesSince(ctx, projectID, documentName, sinceVersion, s.maxUpdatesPerFetch)
				if err != nil {
					return nil, fmt.Errorf("failed to get updates: %w", err)
				}
				response.Updates = s.convertUpdates(updates)
			}
		} else {
			// Client wants full state
			response.Snapshot = encodeBase64(snapshot.Snapshot)
			response.StateVector = encodeBase64(snapshot.StateVector)

			// Get updates after snapshot
			updates, err := s.updateRepo.GetUpdatesSince(ctx, projectID, documentName, snapshot.Version, s.maxUpdatesPerFetch)
			if err != nil {
				return nil, fmt.Errorf("failed to get updates: %w", err)
			}
			response.Updates = s.convertUpdates(updates)
		}
	} else {
		// No snapshot exists, send all updates
		updates, err := s.updateRepo.GetAllUpdates(ctx, projectID, documentName)
		if err != nil {
			return nil, fmt.Errorf("failed to get updates: %w", err)
		}
		response.Updates = s.convertUpdates(updates)
		if len(updates) > 0 {
			response.Version = updates[len(updates)-1].Version
		}
	}

	// Get total update count
	count, err := s.updateRepo.CountUpdates(ctx, projectID, documentName)
	if err == nil {
		response.UpdateCount = int(count)
	}

	return response, nil
}

// GetUpdatesSince retrieves updates since a specific version
func (s *CollaborationService) GetUpdatesSince(ctx context.Context, projectID primitive.ObjectID, documentName string, sinceVersion int64, limit int) ([]*models.YjsUpdate, error) {
	if limit <= 0 || limit > s.maxUpdatesPerFetch {
		limit = s.maxUpdatesPerFetch
	}

	updates, err := s.updateRepo.GetUpdatesSince(ctx, projectID, documentName, sinceVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get updates: %w", err)
	}

	return updates, nil
}

// GetDocumentMetrics retrieves metrics for a document
func (s *CollaborationService) GetDocumentMetrics(ctx context.Context, projectID primitive.ObjectID, documentName string) (*models.DocumentMetrics, error) {
	metrics, err := s.updateRepo.GetUpdateStats(ctx, projectID, documentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Get snapshot count
	snapshotCount, err := s.snapshotRepo.CountSnapshots(ctx, projectID, documentName)
	if err == nil {
		metrics.TotalSnapshots = snapshotCount
	}

	return metrics, nil
}

// createSnapshotAsync creates a snapshot asynchronously
func (s *CollaborationService) createSnapshotAsync(projectID primitive.ObjectID, documentName string, version int64) {
	ctx := context.Background()

	s.logger.Info("Creating snapshot",
		zap.String("project_id", projectID.Hex()),
		zap.String("document", documentName),
		zap.Int64("version", version),
	)

	// Get all updates up to this version
	updates, err := s.updateRepo.GetAllUpdates(ctx, projectID, documentName)
	if err != nil {
		s.logger.Error("Failed to get updates for snapshot", zap.Error(err))
		return
	}

	if len(updates) == 0 {
		return
	}

	// In a real implementation, you would:
	// 1. Merge all updates into a single Yjs document
	// 2. Encode the document state
	// 3. Generate a state vector
	// For now, we'll store a placeholder

	snapshot := &models.YjsSnapshot{
		ProjectID:    projectID,
		DocumentName: documentName,
		Version:      version,
		UpdateCount:  len(updates),
		// In production, these would be actual Yjs state vector and snapshot
		StateVector:  []byte(fmt.Sprintf("state_vector_v%d", version)),
		Snapshot:     []byte(fmt.Sprintf("snapshot_v%d", version)),
	}

	if err := s.snapshotRepo.CreateSnapshot(ctx, snapshot); err != nil {
		s.logger.Error("Failed to create snapshot",
			zap.Error(err),
			zap.String("project_id", projectID.Hex()),
		)
		return
	}

	s.logger.Info("Snapshot created successfully",
		zap.String("project_id", projectID.Hex()),
		zap.String("document", documentName),
		zap.Int64("version", version),
		zap.Int("update_count", len(updates)),
	)
}

// getVersionCounter gets or creates a version counter for a document
func (s *CollaborationService) getVersionCounter(docKey string) *versionCounter {
	s.mu.RLock()
	counter, exists := s.versionCounters[docKey]
	s.mu.RUnlock()

	if exists {
		return counter
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := s.versionCounters[docKey]; exists {
		return counter
	}

	counter = &versionCounter{}
	s.versionCounters[docKey] = counter
	return counter
}

// convertUpdates converts updates to response format with base64 encoding
func (s *CollaborationService) convertUpdates(updates []*models.YjsUpdate) []models.YjsUpdate {
	result := make([]models.YjsUpdate, len(updates))
	for i, update := range updates {
		result[i] = *update
		result[i].EncodeUpdate()
		// Don't send raw bytes in JSON
		result[i].Update = nil
	}
	return result
}

// encodeBase64 encodes bytes to base64 string
func encodeBase64(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	// Base64 encoding is done in the model's EncodeUpdate method
	// For snapshots, we need a helper
	return string(data) // TODO: Implement proper base64 encoding
}

// CleanupOldData removes old updates and snapshots
func (s *CollaborationService) CleanupOldData(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old updates
	deletedUpdates, err := s.updateRepo.DeleteOldUpdates(ctx, cutoffDate)
	if err != nil {
		s.logger.Error("Failed to delete old updates", zap.Error(err))
	} else {
		s.logger.Info("Deleted old updates",
			zap.Int64("count", deletedUpdates),
			zap.Time("older_than", cutoffDate),
		)
	}

	return nil
}
