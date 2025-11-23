package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/texflow/services/compilation/internal/models"
	"github.com/texflow/services/compilation/internal/storage"
	"go.uber.org/zap"
)

// DockerWorker handles LaTeX compilation using Docker containers
type DockerWorker struct {
	dockerClient  *client.Client
	minioClient   *storage.MinIOClient
	logger        *zap.Logger
	texliveImage  string
	timeout       time.Duration
	memoryLimit   int64
	cpuLimit      int64
	workDir       string
}

// NewDockerWorker creates a new Docker worker
func NewDockerWorker(
	dockerClient *client.Client,
	minioClient *storage.MinIOClient,
	logger *zap.Logger,
	texliveImage string,
	timeout time.Duration,
	memoryLimit int64,
	cpuLimit int64,
	workDir string,
) *DockerWorker {
	return &DockerWorker{
		dockerClient: dockerClient,
		minioClient:  minioClient,
		logger:       logger,
		texliveImage: texliveImage,
		timeout:      timeout,
		memoryLimit:  memoryLimit,
		cpuLimit:     cpuLimit,
		workDir:      workDir,
	}
}

// Compile performs a LaTeX compilation
func (w *DockerWorker) Compile(ctx context.Context, job *models.CompilationJob) (*models.CompilationResult, error) {
	startTime := time.Now()

	w.logger.Info("Starting compilation",
		zap.String("compilation_id", job.CompilationID),
		zap.String("compiler", job.Compiler),
		zap.String("main_file", job.MainFile),
	)

	// Create temporary working directory
	projectDir := filepath.Join(w.workDir, job.CompilationID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}
	defer os.RemoveAll(projectDir)

	// Write project files to disk
	if err := w.writeProjectFiles(projectDir, job.Files); err != nil {
		return nil, fmt.Errorf("failed to write project files: %w", err)
	}

	// Create Docker container
	containerID, err := w.createContainer(ctx, projectDir, job.Compiler, job.MainFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	defer w.cleanupContainer(ctx, containerID)

	// Start compilation with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	if err := w.dockerClient.ContainerStart(timeoutCtx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for completion
	statusCh, errCh := w.dockerClient.ContainerWait(timeoutCtx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("container wait error: %w", err)
		}
	case <-statusCh:
		// Container completed
	case <-timeoutCtx.Done():
		w.logger.Warn("Compilation timeout",
			zap.String("compilation_id", job.CompilationID),
			zap.Duration("timeout", w.timeout),
		)
		return &models.CompilationResult{
			CompilationID: job.CompilationID,
			Status:        models.StatusTimeout,
			ErrorMessage:  "Compilation timeout exceeded",
			DurationMs:    time.Since(startTime).Milliseconds(),
		}, nil
	}

	// Get container exit code
	inspect, err := w.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Copy output files
	outputFile := getOutputFileName(job.MainFile, job.Compiler)
	outputPath := filepath.Join(projectDir, outputFile)
	logPath := filepath.Join(projectDir, getLogFileName(job.MainFile))

	var result models.CompilationResult
	result.CompilationID = job.CompilationID
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Check if compilation was successful
	if inspect.State.ExitCode == 0 && fileExists(outputPath) {
		// Upload PDF to MinIO
		pdfKey := fmt.Sprintf("compilations/%s/%s", job.CompilationID, outputFile)
		if err := w.uploadFile(ctx, pdfKey, outputPath); err != nil {
			w.logger.Error("Failed to upload PDF", zap.Error(err))
		} else {
			result.OutputURL = pdfKey
		}

		// Upload log file
		if fileExists(logPath) {
			logKey := fmt.Sprintf("compilations/%s/%s", job.CompilationID, filepath.Base(logPath))
			if err := w.uploadFile(ctx, logKey, logPath); err != nil {
				w.logger.Error("Failed to upload log", zap.Error(err))
			} else {
				result.LogURL = logKey
			}
		}

		result.Status = models.StatusCompleted
		w.logger.Info("Compilation completed successfully",
			zap.String("compilation_id", job.CompilationID),
			zap.Int64("duration_ms", result.DurationMs),
		)
	} else {
		// Compilation failed
		errorMsg := "Compilation failed"
		if fileExists(logPath) {
			// Upload log for debugging
			logKey := fmt.Sprintf("compilations/%s/%s", job.CompilationID, filepath.Base(logPath))
			if err := w.uploadFile(ctx, logKey, logPath); err == nil {
				result.LogURL = logKey
			}

			// Read error from log
			if logContent, err := os.ReadFile(logPath); err == nil {
				errorMsg = extractError(string(logContent))
			}
		}

		result.Status = models.StatusFailed
		result.ErrorMessage = errorMsg
		w.logger.Warn("Compilation failed",
			zap.String("compilation_id", job.CompilationID),
			zap.Int("exit_code", inspect.State.ExitCode),
			zap.String("error", errorMsg),
		)
	}

	return &result, nil
}

// createContainer creates a Docker container for compilation
func (w *DockerWorker) createContainer(ctx context.Context, projectDir, compiler, mainFile string) (string, error) {
	// Determine compiler command
	var cmd []string
	switch compiler {
	case "xelatex":
		cmd = []string{"xelatex", "-interaction=nonstopmode", "-output-directory=/workspace", mainFile}
	case "lualatex":
		cmd = []string{"lualatex", "-interaction=nonstopmode", "-output-directory=/workspace", mainFile}
	default: // pdflatex
		cmd = []string{"pdflatex", "-interaction=nonstopmode", "-output-directory=/workspace", mainFile}
	}

	config := &container.Config{
		Image:      w.texliveImage,
		Cmd:        cmd,
		WorkingDir: "/workspace",
		User:       "65534:65534", // nobody user
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/workspace", projectDir),
		},
		Resources: container.Resources{
			Memory:   w.memoryLimit,
			NanoCPUs: w.cpuLimit,
			PidsLimit: func() *int64 {
				limit := int64(256)
				return &limit
			}(),
		},
		NetworkMode: "none", // No network access
		ReadonlyRootfs: false, // TeX needs to write temp files
		Tmpfs: map[string]string{
			"/tmp": "rw,noexec,nosuid,size=512m",
		},
		SecurityOpt: []string{
			"no-new-privileges",
		},
	}

	resp, err := w.dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// cleanupContainer removes a Docker container
func (w *DockerWorker) cleanupContainer(ctx context.Context, containerID string) {
	if err := w.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		w.logger.Error("Failed to remove container", zap.String("container_id", containerID), zap.Error(err))
	}
}

// writeProjectFiles writes project files to disk
func (w *DockerWorker) writeProjectFiles(projectDir string, files map[string]string) error {
	for filename, content := range files {
		// Prevent path traversal
		if strings.Contains(filename, "..") {
			continue
		}

		filePath := filepath.Join(projectDir, filename)

		// Create subdirectories if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// uploadFile uploads a file to MinIO
func (w *DockerWorker) uploadFile(ctx context.Context, key, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	contentType := "application/pdf"
	if strings.HasSuffix(filePath, ".log") {
		contentType = "text/plain"
	}

	return w.minioClient.UploadFile(ctx, key, file, stat.Size(), contentType)
}

// CalculateInputHash calculates SHA256 hash of input files
func CalculateInputHash(files map[string]string, compiler, mainFile string) string {
	h := sha256.New()

	// Sort filenames for consistent hashing
	filenames := make([]string, 0, len(files))
	for filename := range files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	// Hash compiler and main file
	h.Write([]byte(compiler))
	h.Write([]byte(mainFile))

	// Hash all files
	for _, filename := range filenames {
		h.Write([]byte(filename))
		h.Write([]byte(files[filename]))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func getOutputFileName(mainFile, compiler string) string {
	base := strings.TrimSuffix(mainFile, filepath.Ext(mainFile))
	return base + ".pdf"
}

func getLogFileName(mainFile string) string {
	base := strings.TrimSuffix(mainFile, filepath.Ext(mainFile))
	return base + ".log"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractError(logContent string) string {
	lines := strings.Split(logContent, "\n")
	for _, line := range lines {
		if strings.Contains(line, "!") || strings.Contains(line, "Error") {
			return strings.TrimSpace(line)
		}
	}
	return "Compilation failed - check log for details"
}
