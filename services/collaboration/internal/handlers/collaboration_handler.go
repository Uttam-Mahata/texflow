package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/texflow/services/collaboration/internal/models"
	"github.com/texflow/services/collaboration/internal/service"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// CollaborationHandler handles collaboration HTTP requests
type CollaborationHandler struct {
	collabService *service.CollaborationService
	logger        *zap.Logger
}

// NewCollaborationHandler creates a new collaboration handler
func NewCollaborationHandler(collabService *service.CollaborationService, logger *zap.Logger) *CollaborationHandler {
	return &CollaborationHandler{
		collabService: collabService,
		logger:        logger,
	}
}

// StoreUpdate stores a Yjs update
// @Summary Store a Yjs CRDT update
// @Tags collaboration
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.StoreUpdateRequest true "Update request"
// @Success 201 {object} models.YjsUpdate
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /collaboration/updates [post]
func (h *CollaborationHandler) StoreUpdate(c *gin.Context) {
	var req models.StoreUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	projectID, err := primitive.ObjectIDFromHex(req.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Decode base64 update
	updateData, err := base64.StdEncoding.DecodeString(req.Update)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid update encoding"})
		return
	}

	// Store update
	update, err := h.collabService.StoreUpdate(
		c.Request.Context(),
		projectID,
		req.DocumentName,
		updateData,
		userID,
		req.ClientID,
	)
	if err != nil {
		h.logger.Error("Failed to store update", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store update"})
		return
	}

	// Encode update for response
	update.EncodeUpdate()
	update.Update = nil // Don't send raw bytes

	c.JSON(http.StatusCreated, update)
}

// GetDocumentState retrieves the current state of a document
// @Summary Get document state
// @Tags collaboration
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param document_name path string true "Document name"
// @Param since_version query int false "Since version"
// @Success 200 {object} models.DocumentStateResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /collaboration/state/:project_id/:document_name [get]
func (h *CollaborationHandler) GetDocumentState(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	documentName := c.Param("document_name")

	projectID, err := primitive.ObjectIDFromHex(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Get optional since_version parameter
	var sinceVersion int64
	if sv := c.Query("since_version"); sv != "" {
		fmt.Sscanf(sv, "%d", &sinceVersion)
	}

	state, err := h.collabService.GetDocumentState(
		c.Request.Context(),
		projectID,
		documentName,
		sinceVersion,
	)
	if err != nil {
		h.logger.Error("Failed to get document state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document state"})
		return
	}

	c.JSON(http.StatusOK, state)
}

// GetUpdates retrieves updates since a specific version
// @Summary Get updates since version
// @Tags collaboration
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param document_name path string true "Document name"
// @Param since_version query int true "Since version"
// @Param limit query int false "Limit"
// @Success 200 {array} models.YjsUpdate
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /collaboration/updates/:project_id/:document_name [get]
func (h *CollaborationHandler) GetUpdates(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	documentName := c.Param("document_name")

	projectID, err := primitive.ObjectIDFromHex(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var sinceVersion int64
	if sv := c.Query("since_version"); sv != "" {
		fmt.Sscanf(sv, "%d", &sinceVersion)
	}

	var limit int = 1000
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	updates, err := h.collabService.GetUpdatesSince(
		c.Request.Context(),
		projectID,
		documentName,
		sinceVersion,
		limit,
	)
	if err != nil {
		h.logger.Error("Failed to get updates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updates"})
		return
	}

	// Encode updates for response
	response := make([]models.YjsUpdate, len(updates))
	for i, update := range updates {
		response[i] = *update
		response[i].EncodeUpdate()
		response[i].Update = nil
	}

	c.JSON(http.StatusOK, response)
}

// GetMetrics retrieves metrics for a document
// @Summary Get document metrics
// @Tags collaboration
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param document_name path string true "Document name"
// @Success 200 {object} models.DocumentMetrics
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /collaboration/metrics/:project_id/:document_name [get]
func (h *CollaborationHandler) GetMetrics(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	documentName := c.Param("document_name")

	projectID, err := primitive.ObjectIDFromHex(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	metrics, err := h.collabService.GetDocumentMetrics(
		c.Request.Context(),
		projectID,
		documentName,
	)
	if err != nil {
		h.logger.Error("Failed to get metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// Health returns the health status of the service
// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *CollaborationHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "collaboration-service",
	})
}
