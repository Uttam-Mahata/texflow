package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Project represents a LaTeX project
type Project struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name            string             `bson:"name" json:"name"`
	Description     string             `bson:"description,omitempty" json:"description,omitempty"`
	OwnerID         primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Collaborators   []Collaborator     `bson:"collaborators,omitempty" json:"collaborators,omitempty"`
	Settings        ProjectSettings    `bson:"settings" json:"settings"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
	LastCompiledAt  *time.Time         `bson:"last_compiled_at,omitempty" json:"last_compiled_at,omitempty"`
	TemplateID      *primitive.ObjectID `bson:"template_id,omitempty" json:"template_id,omitempty"`
	FileCount       int                `bson:"file_count" json:"file_count"`
	TotalSizeBytes  int64              `bson:"total_size_bytes" json:"total_size_bytes"`
	IsPublic        bool               `bson:"is_public" json:"is_public"`
	Tags            []string           `bson:"tags,omitempty" json:"tags,omitempty"`
}

// Collaborator represents a project collaborator
type Collaborator struct {
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	Role       string             `bson:"role" json:"role"` // owner, editor, viewer
	InvitedAt  time.Time          `bson:"invited_at" json:"invited_at"`
	AcceptedAt *time.Time         `bson:"accepted_at,omitempty" json:"accepted_at,omitempty"`
}

// ProjectSettings holds project-specific settings
type ProjectSettings struct {
	Compiler    string `bson:"compiler" json:"compiler"`
	MainFile    string `bson:"main_file" json:"main_file"`
	SpellCheck  bool   `bson:"spell_check" json:"spell_check"`
	AutoCompile bool   `bson:"auto_compile" json:"auto_compile"`
}

// File represents a file within a project
type File struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID   primitive.ObjectID `bson:"project_id" json:"project_id"`
	Name        string             `bson:"name" json:"name"`
	Path        string             `bson:"path" json:"path"`
	ContentType string             `bson:"content_type" json:"content_type"`
	SizeBytes   int64              `bson:"size_bytes" json:"size_bytes"`
	StorageKey  string             `bson:"storage_key" json:"storage_key"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
	CreatedBy   primitive.ObjectID `bson:"created_by" json:"created_by"`
	Version     int                `bson:"version" json:"version"`
	IsBinary    bool               `bson:"is_binary" json:"is_binary"`
	Hash        string             `bson:"hash" json:"hash"`
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	Description string   `json:"description" binding:"max=500"`
	Compiler    string   `json:"compiler" binding:"omitempty,oneof=pdflatex xelatex lualatex"`
	IsPublic    bool     `json:"is_public"`
	Tags        []string `json:"tags" binding:"max=10"`
}

// UpdateProjectRequest represents a request to update a project
type UpdateProjectRequest struct {
	Name        *string  `json:"name" binding:"omitempty,min=1,max=100"`
	Description *string  `json:"description" binding:"omitempty,max=500"`
	Compiler    *string  `json:"compiler" binding:"omitempty,oneof=pdflatex xelatex lualatex"`
	MainFile    *string  `json:"main_file"`
	IsPublic    *bool    `json:"is_public"`
	Tags        []string `json:"tags" binding:"omitempty,max=10"`
}

// CreateFileRequest represents a request to create/upload a file
type CreateFileRequest struct {
	Name     string `json:"name" binding:"required"`
	Path     string `json:"path" binding:"required"`
	Content  []byte `json:"content"`
	IsBinary bool   `json:"is_binary"`
}

// ShareProjectRequest represents a request to share a project
type ShareProjectRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=editor viewer"`
}
