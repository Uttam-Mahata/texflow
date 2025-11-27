package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/texflow/services/project/internal/service"
	"go.uber.org/zap"
)

type ProjectHandler struct {
	projectService *service.ProjectService
	logger         *zap.Logger
}

func NewProjectHandler(projectService *service.ProjectService, logger *zap.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		logger:         logger,
	}
}

func (h *ProjectHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"service": "project-service",
	})
}
