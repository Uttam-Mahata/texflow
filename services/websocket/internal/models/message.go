package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// User presence events
	MessageTypeUserJoined MessageType = "user_joined"
	MessageTypeUserLeft   MessageType = "user_left"
	MessageTypeUserTyping MessageType = "user_typing"

	// Document events
	MessageTypeDocumentUpdate MessageType = "document_update"
	MessageTypeCursorUpdate   MessageType = "cursor_update"
	MessageTypeSelection      MessageType = "selection"

	// Collaboration events
	MessageTypeYjsUpdate    MessageType = "yjs_update"
	MessageTypeYjsAwareness MessageType = "yjs_awareness"

	// Compilation events
	MessageTypeCompilationStarted   MessageType = "compilation_started"
	MessageTypeCompilationCompleted MessageType = "compilation_completed"
	MessageTypeCompilationFailed    MessageType = "compilation_failed"

	// System events
	MessageTypePing  MessageType = "ping"
	MessageTypePong  MessageType = "pong"
	MessageTypeError MessageType = "error"
)

// Message represents a WebSocket message
type Message struct {
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	UserID    string          `json:"user_id,omitempty"`
	Username  string          `json:"username,omitempty"`
}

// UserPresence represents user presence information
type UserPresence struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Color     string    `json:"color"`
	JoinedAt  time.Time `json:"joined_at"`
}

// CursorPosition represents a user's cursor position
type CursorPosition struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Color    string `json:"color"`
}

// Selection represents a text selection
type Selection struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
	Color     string `json:"color"`
}

// YjsUpdate represents a Yjs CRDT update
type YjsUpdate struct {
	Update    []byte `json:"update"`
	Origin    string `json:"origin,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// CompilationEvent represents a compilation status event
type CompilationEvent struct {
	ProjectID     string `json:"project_id"`
	CompilationID string `json:"compilation_id"`
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	Progress      int    `json:"progress,omitempty"`
}

// ErrorPayload represents an error message payload
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConnectionInfo stores information about a WebSocket connection
type ConnectionInfo struct {
	ID           string
	UserID       primitive.ObjectID
	Username     string
	ProjectID    primitive.ObjectID
	IPAddress    string
	ConnectedAt  time.Time
	LastActivity time.Time
}

// NewMessage creates a new message
func NewMessage(msgType MessageType, payload interface{}, userID, username string) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		Payload:   payloadBytes,
		Timestamp: time.Now(),
		UserID:    userID,
		Username:  username,
	}, nil
}

// UnmarshalPayload unmarshals the message payload into the given type
func (m *Message) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}
