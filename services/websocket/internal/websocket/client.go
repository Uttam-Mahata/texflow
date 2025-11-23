package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/texflow/services/websocket/internal/models"
	"go.uber.org/zap"
)

// Client represents a WebSocket client connection
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	projectID string
	userID    string
	username  string
	color     string
	logger    *zap.Logger

	// Connection settings
	readBufferSize  int
	writeBufferSize int
	maxMessageSize  int64
	pongWait        time.Duration
	pingInterval    time.Duration
	writeWait       time.Duration
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, projectID, userID, username, color string, logger *zap.Logger) *Client {
	return &Client{
		hub:             hub,
		conn:            conn,
		send:            make(chan []byte, 256),
		projectID:       projectID,
		userID:          userID,
		username:        username,
		color:           color,
		logger:          logger,
		readBufferSize:  1024,
		writeBufferSize: 1024,
		maxMessageSize:  512 * 1024, // 512 KB
		pongWait:        60 * time.Second,
		pingInterval:    54 * time.Second,
		writeWait:       10 * time.Second,
	}
}

// ReadPump reads messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetReadLimit(c.maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error",
					zap.String("user_id", c.userID),
					zap.String("project_id", c.projectID),
					zap.Error(err),
				)
			}
			break
		}

		// Parse the message
		var msg models.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			c.logger.Warn("Failed to parse message",
				zap.String("user_id", c.userID),
				zap.Error(err),
			)
			c.sendError("invalid_message", "Failed to parse message")
			continue
		}

		// Set message metadata
		msg.UserID = c.userID
		msg.Username = c.username
		msg.Timestamp = time.Now()

		// Handle different message types
		c.handleMessage(&msg)
	}
}

// WritePump writes messages to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles different types of incoming messages
func (c *Client) handleMessage(msg *models.Message) {
	switch msg.Type {
	case models.MessageTypePing:
		// Respond with pong
		pongMsg, _ := models.NewMessage(models.MessageTypePong, nil, c.userID, c.username)
		c.sendMessage(pongMsg)

	case models.MessageTypeYjsUpdate:
		// Broadcast Yjs update to all clients in the room
		c.hub.broadcast <- &BroadcastMessage{
			roomID:  c.projectID,
			message: msg,
			exclude: c, // Don't send back to sender
		}

	case models.MessageTypeYjsAwareness:
		// Broadcast awareness update
		c.hub.broadcast <- &BroadcastMessage{
			roomID:  c.projectID,
			message: msg,
			exclude: c,
		}

	case models.MessageTypeCursorUpdate:
		// Broadcast cursor position
		c.hub.broadcast <- &BroadcastMessage{
			roomID:  c.projectID,
			message: msg,
			exclude: c,
		}

	case models.MessageTypeSelection:
		// Broadcast selection
		c.hub.broadcast <- &BroadcastMessage{
			roomID:  c.projectID,
			message: msg,
			exclude: c,
		}

	case models.MessageTypeUserTyping:
		// Broadcast typing indicator
		c.hub.broadcast <- &BroadcastMessage{
			roomID:  c.projectID,
			message: msg,
			exclude: c,
		}

	default:
		c.logger.Warn("Unknown message type",
			zap.String("type", string(msg.Type)),
			zap.String("user_id", c.userID),
		)
	}
}

// sendMessage sends a message to the client
func (c *Client) sendMessage(msg *models.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("Failed to marshal message", zap.Error(err))
		return
	}

	select {
	case c.send <- data:
	default:
		// Channel is full, close the connection
		close(c.send)
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	errorMsg, err := models.NewMessage(
		models.MessageTypeError,
		models.ErrorPayload{
			Code:    code,
			Message: message,
		},
		"system",
		"system",
	)
	if err != nil {
		c.logger.Error("Failed to create error message", zap.Error(err))
		return
	}

	c.sendMessage(errorMsg)
}
