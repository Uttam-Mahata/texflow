package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CompilationStatus represents the status of a compilation
type CompilationStatus string

const (
	StatusQueued     CompilationStatus = "queued"
	StatusRunning    CompilationStatus = "running"
	StatusCompleted  CompilationStatus = "completed"
	StatusFailed     CompilationStatus = "failed"
	StatusCancelled  CompilationStatus = "cancelled"
	StatusTimeout    CompilationStatus = "timeout"
)

// Compilation represents a LaTeX compilation job
type Compilation struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID   primitive.ObjectID `bson:"project_id" json:"project_id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	Status      CompilationStatus  `bson:"status" json:"status"`
	Compiler    string             `bson:"compiler" json:"compiler"` // pdflatex, xelatex, lualatex
	MainFile    string             `bson:"main_file" json:"main_file"`

	// Input hash for caching
	InputHash   string             `bson:"input_hash" json:"input_hash"`

	// Results
	OutputFileKey string           `bson:"output_file_key,omitempty" json:"output_file_key,omitempty"` // MinIO key
	LogFileKey    string           `bson:"log_file_key,omitempty" json:"log_file_key,omitempty"`
	OutputURL     string           `bson:"-" json:"output_url,omitempty"` // Presigned URL

	// Metrics
	StartedAt     *time.Time         `bson:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt   *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	DurationMs    int64              `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"`

	// Error information
	ErrorMessage  string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	ExitCode      int                `bson:"exit_code,omitempty" json:"exit_code,omitempty"`

	// Cache information
	CachedResult  bool               `bson:"cached_result" json:"cached_result"`

	// Timestamps
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

// CompilationJob represents a compilation job in the queue
type CompilationJob struct {
	CompilationID string                 `json:"compilation_id"`
	ProjectID     string                 `json:"project_id"`
	UserID        string                 `json:"user_id"`
	Compiler      string                 `json:"compiler"`
	MainFile      string                 `json:"main_file"`
	InputHash     string                 `json:"input_hash"`
	Files         map[string]string      `json:"files"` // filename -> content
	Priority      int                    `json:"priority"`
}

// CompileRequest represents a compilation request from a client
type CompileRequest struct {
	ProjectID string `json:"project_id" binding:"required"`
	Compiler  string `json:"compiler" binding:"omitempty,oneof=pdflatex xelatex lualatex"`
	MainFile  string `json:"main_file" binding:"required"`
}

// CompilationResult represents the result of a compilation
type CompilationResult struct {
	CompilationID string            `json:"compilation_id"`
	Status        CompilationStatus `json:"status"`
	OutputURL     string            `json:"output_url,omitempty"`
	LogURL        string            `json:"log_url,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	DurationMs    int64             `json:"duration_ms,omitempty"`
	CachedResult  bool              `json:"cached_result"`
}

// CompilationStats represents compilation statistics
type CompilationStats struct {
	TotalCompilations int64                        `json:"total_compilations"`
	StatusCounts      map[CompilationStatus]int64  `json:"status_counts"`
	AvgDurationMs     float64                      `json:"avg_duration_ms"`
	CacheHitRate      float64                      `json:"cache_hit_rate"`
	TopCompilers      []CompilerStats              `json:"top_compilers"`
}

// CompilerStats represents statistics for a compiler
type CompilerStats struct {
	Compiler string `json:"compiler"`
	Count    int64  `json:"count"`
}

// WorkerStatus represents the status of a compilation worker
type WorkerStatus struct {
	WorkerID       string    `json:"worker_id"`
	Status         string    `json:"status"` // idle, busy
	CurrentJob     string    `json:"current_job,omitempty"`
	JobsProcessed  int64     `json:"jobs_processed"`
	LastActivity   time.Time `json:"last_activity"`
}

// QueueStats represents statistics about the compilation queue
type QueueStats struct {
	QueueLength    int64          `json:"queue_length"`
	ActiveWorkers  int            `json:"active_workers"`
	TotalWorkers   int            `json:"total_workers"`
	Workers        []WorkerStatus `json:"workers"`
}
