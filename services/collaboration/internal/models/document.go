package models

import (
	"encoding/base64"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// YjsUpdate represents a Yjs CRDT update
type YjsUpdate struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID    primitive.ObjectID `bson:"project_id" json:"project_id"`
	DocumentName string             `bson:"document_name" json:"document_name"`
	Update       []byte             `bson:"update" json:"update"`
	UpdateBase64 string             `bson:"-" json:"update_base64,omitempty"` // For JSON serialization
	Clock        int64              `bson:"clock" json:"clock"`
	Version      int64              `bson:"version" json:"version"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UserID       primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`
	ClientID     string             `bson:"client_id,omitempty" json:"client_id,omitempty"`
	SizeBytes    int                `bson:"size_bytes" json:"size_bytes"`
}

// YjsSnapshot represents a document snapshot
type YjsSnapshot struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID    primitive.ObjectID `bson:"project_id" json:"project_id"`
	DocumentName string             `bson:"document_name" json:"document_name"`
	StateVector  []byte             `bson:"state_vector" json:"state_vector"`
	Snapshot     []byte             `bson:"snapshot" json:"snapshot"`
	Version      int64              `bson:"version" json:"version"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	SizeBytes    int64              `bson:"size_bytes" json:"size_bytes"`
	UpdateCount  int                `bson:"update_count" json:"update_count"` // Number of updates at snapshot time
}

// DocumentState represents the current state of a document
type DocumentState struct {
	ProjectID    primitive.ObjectID `bson:"_id" json:"project_id"`
	DocumentName string             `bson:"document_name" json:"document_name"`
	CurrentVersion int64            `bson:"current_version" json:"current_version"`
	UpdateCount    int              `bson:"update_count" json:"update_count"`
	LastSnapshot   *YjsSnapshot     `bson:"last_snapshot,omitempty" json:"last_snapshot,omitempty"`
	LastUpdatedAt  time.Time        `bson:"last_updated_at" json:"last_updated_at"`
	TotalSizeBytes int64            `bson:"total_size_bytes" json:"total_size_bytes"`
}

// StoreUpdateRequest represents a request to store a Yjs update
type StoreUpdateRequest struct {
	ProjectID    string `json:"project_id" binding:"required"`
	DocumentName string `json:"document_name" binding:"required"`
	Update       string `json:"update" binding:"required"` // Base64 encoded
	ClientID     string `json:"client_id"`
}

// GetUpdatesRequest represents a request to get updates
type GetUpdatesRequest struct {
	ProjectID    string `json:"project_id" binding:"required"`
	DocumentName string `json:"document_name" binding:"required"`
	SinceVersion int64  `json:"since_version"`
	Limit        int    `json:"limit"`
}

// GetStateRequest represents a request to get document state
type GetStateRequest struct {
	ProjectID    string `json:"project_id" binding:"required"`
	DocumentName string `json:"document_name" binding:"required"`
}

// DocumentStateResponse represents the response for document state
type DocumentStateResponse struct {
	StateVector  string       `json:"state_vector"` // Base64 encoded
	Snapshot     string       `json:"snapshot,omitempty"` // Base64 encoded
	Updates      []YjsUpdate  `json:"updates,omitempty"`
	Version      int64        `json:"version"`
	UpdateCount  int          `json:"update_count"`
}

// EncodeUpdate converts update bytes to base64
func (u *YjsUpdate) EncodeUpdate() {
	if len(u.Update) > 0 {
		u.UpdateBase64 = base64.StdEncoding.EncodeToString(u.Update)
	}
}

// DecodeUpdate converts base64 to update bytes
func (u *YjsUpdate) DecodeUpdate(base64Str string) error {
	decoded, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return err
	}
	u.Update = decoded
	u.SizeBytes = len(decoded)
	return nil
}

// AwarenessUpdate represents a Yjs awareness update
type AwarenessUpdate struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	ClientID  string             `bson:"client_id" json:"client_id"`
	State     map[string]interface{} `bson:"state" json:"state"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"` // TTL
}

// DocumentMetrics represents metrics for a document
type DocumentMetrics struct {
	ProjectID       primitive.ObjectID `bson:"_id" json:"project_id"`
	DocumentName    string             `bson:"document_name" json:"document_name"`
	TotalUpdates    int64              `bson:"total_updates" json:"total_updates"`
	TotalSnapshots  int64              `bson:"total_snapshots" json:"total_snapshots"`
	CurrentVersion  int64              `bson:"current_version" json:"current_version"`
	LastUpdated     time.Time          `bson:"last_updated" json:"last_updated"`
	Contributors    []primitive.ObjectID `bson:"contributors" json:"contributors"`
	SizeBytes       int64              `bson:"size_bytes" json:"size_bytes"`
}
