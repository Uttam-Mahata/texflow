package handlers

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	ws "github.com/texflow/services/websocket/internal/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Configure this properly for production
		return true
	},
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub    *ws.Hub
	logger *zap.Logger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub:    hub,
		logger: logger,
	}
}

// HandleConnection handles a WebSocket connection request
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	// Get user info from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	username, exists := c.Get("username")
	if !exists {
		username = "Anonymous"
	}

	// Generate a random color for the user
	color := generateUserColor()

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection",
			zap.String("user_id", userID.(string)),
			zap.Error(err),
		)
		return
	}

	// Create new client
	client := ws.NewClient(
		h.hub,
		conn,
		projectID,
		userID.(string),
		username.(string),
		color,
		h.logger,
	)

	// Register client with the hub
	h.hub.RegisterClient(client)

	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("WebSocket connection established",
		zap.String("user_id", userID.(string)),
		zap.String("username", username.(string)),
		zap.String("project_id", projectID),
	)
}

// GetStats returns WebSocket statistics
func (h *WebSocketHandler) GetStats(c *gin.Context) {
	stats := h.hub.GetRoomStats()
	c.JSON(http.StatusOK, stats)
}

// Health returns the health status of the service
func (h *WebSocketHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "websocket-service",
	})
}

// generateUserColor generates a random color for user identification
func generateUserColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#FFA07A",
		"#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E2",
		"#F8B739", "#52B788", "#E63946", "#A8DADC",
	}
	return colors[rand.Intn(len(colors))]
}

// ValidateToken validates a JWT token from query parameter or header
func ValidateToken(c *gin.Context) string {
	// Try to get token from query parameter first (for WebSocket upgrade)
	token := c.Query("token")
	if token != "" {
		return token
	}

	// Try to get from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	return ""
}
