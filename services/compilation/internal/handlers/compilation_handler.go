package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/texflow/services/compilation/internal/models"
	"github.com/texflow/services/compilation/internal/service"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// CompilationHandler handles compilation HTTP requests
type CompilationHandler struct {
	compilationService *service.CompilationService
	projectService     *service.ProjectService
	logger             *zap.Logger
}

// NewCompilationHandler creates a new compilation handler
func NewCompilationHandler(
	compilationService *service.CompilationService,
	projectService *service.ProjectService,
	logger *zap.Logger,
) *CompilationHandler {
	return &CompilationHandler{
		compilationService: compilationService,
		projectService:     projectService,
		logger:             logger,
	}
}

// Compile handles compilation requests
// @Summary Compile a LaTeX project
// @Tags compilation
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CompileRequest true "Compile request"
// @Success 202 {object} models.Compilation
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /compilation/compile [post]
func (h *CompilationHandler) Compile(c *gin.Context) {
	var req models.CompileRequest
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

	// Get project files
	files, err := h.projectService.GetProjectFiles(c.Request.Context(), projectID)
	if err != nil {
		h.logger.Error("Failed to get project files", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project files"})
		return
	}

	// Request compilation
	compilation, err := h.compilationService.RequestCompilation(
		c.Request.Context(),
		projectID,
		userID,
		req.Compiler,
		req.MainFile,
		files,
	)
	if err != nil {
		h.logger.Error("Failed to request compilation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, compilation)
}

// GetCompilation retrieves a compilation by ID
// @Summary Get compilation status
// @Tags compilation
// @Produce json
// @Security BearerAuth
// @Param id path string true "Compilation ID"
// @Success 200 {object} models.Compilation
// @Failure 404 {object} map[string]string
// @Router /compilation/{id} [get]
func (h *CompilationHandler) GetCompilation(c *gin.Context) {
	compilationIDStr := c.Param("id")
	compilationID, err := primitive.ObjectIDFromHex(compilationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid compilation ID"})
		return
	}

	userIDStr, _ := c.Get("user_id")
	userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	compilation, err := h.compilationService.GetCompilation(c.Request.Context(), compilationID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, compilation)
}

// ListCompilations lists compilations for a project
// @Summary List project compilations
// @Tags compilation
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param limit query int false "Limit"
// @Success 200 {array} models.Compilation
// @Router /compilation/project/{project_id} [get]
func (h *CompilationHandler) ListCompilations(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	projectID, err := primitive.ObjectIDFromHex(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	compilations, err := h.compilationService.ListProjectCompilations(c.Request.Context(), projectID, limit)
	if err != nil {
		h.logger.Error("Failed to list compilations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list compilations"})
		return
	}

	c.JSON(http.StatusOK, compilations)
}

// GetStats retrieves compilation statistics
// @Summary Get compilation statistics
// @Tags compilation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.CompilationStats
// @Router /compilation/stats [get]
func (h *CompilationHandler) GetStats(c *gin.Context) {
	stats, err := h.compilationService.GetStats(c.Request.Context(), 24*7*time.Hour) // Last 7 days
	if err != nil {
		h.logger.Error("Failed to get stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetQueueStats retrieves queue statistics
// @Summary Get queue statistics
// @Tags compilation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.QueueStats
// @Router /compilation/queue [get]
func (h *CompilationHandler) GetQueueStats(c *gin.Context) {
	stats, err := h.compilationService.GetQueueStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get queue stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Health returns the health status of the service
// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *CompilationHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "compilation-service",
	})
}
