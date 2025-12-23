package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"compilation/internal/models"
	"compilation/internal/storage"
	"go.uber.org/zap"
)

// DockerWorker handles LaTeX compilation (now using direct exec instead of Docker)
type DockerWorker struct {
	minioClient *storage.MinIOClient
	logger      *zap.Logger
	timeout     time.Duration
	workDir     string
}

// NewDockerWorker creates a new worker
func NewDockerWorker(
	dockerClient interface{}, // kept for API compatibility, ignored
	minioClient *storage.MinIOClient,
	logger *zap.Logger,
	texliveImage string, // kept for API compatibility, ignored
	timeout time.Duration,
	memoryLimit int64, // kept for API compatibility, ignored
	cpuLimit int64, // kept for API compatibility, ignored
	workDir string,
) *DockerWorker {
	return &DockerWorker{
		minioClient: minioClient,
		logger:      logger,
		timeout:     timeout,
		workDir:     workDir,
	}
}

// Compile performs a LaTeX compilation using direct exec
func (w *DockerWorker) Compile(ctx context.Context, job *models.CompilationJob) (*models.CompilationResult, error) {
	startTime := time.Now()

	w.logger.Info("Starting compilation",
		zap.String("compilation_id", job.CompilationID),
		zap.String("compiler", job.Compiler),
		zap.String("main_file", job.MainFile),
		zap.Int("file_count", len(job.Files)),
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

	// Run compilation with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	exitCode, compileErr := w.runCompilation(timeoutCtx, projectDir, job.Compiler, job.MainFile)

	// Check for timeout
	if timeoutCtx.Err() == context.DeadlineExceeded {
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

	// Process results
	outputFile := getOutputFileName(job.MainFile, job.Compiler)
	outputPath := filepath.Join(projectDir, outputFile)
	logPath := filepath.Join(projectDir, getLogFileName(job.MainFile))

	var result models.CompilationResult
	result.CompilationID = job.CompilationID
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Check if compilation was successful
	if exitCode == 0 && fileExists(outputPath) {
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
		if compileErr != nil {
			errorMsg = compileErr.Error()
		}

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
			zap.Int("exit_code", exitCode),
			zap.String("error", errorMsg),
		)
	}

	return &result, nil
}

// runCompilation executes the LaTeX compiler directly
func (w *DockerWorker) runCompilation(ctx context.Context, projectDir, compiler, mainFile string) (int, error) {
	// Determine compiler command
	var compilerCmd string
	switch compiler {
	case "xelatex":
		compilerCmd = "xelatex"
	case "lualatex":
		compilerCmd = "lualatex"
	default:
		compilerCmd = "pdflatex"
	}

	// Build command arguments
	args := []string{
		"-interaction=nonstopmode",
		"-output-directory=" + projectDir,
		filepath.Join(projectDir, mainFile),
	}

	w.logger.Info("Running LaTeX compiler",
		zap.String("compiler", compilerCmd),
		zap.Strings("args", args),
	)

	cmd := exec.CommandContext(ctx, compilerCmd, args...)
	cmd.Dir = projectDir

	// Capture output for debugging
	output, err := cmd.CombinedOutput()

	// Log output for debugging
	if len(output) > 0 {
		w.logger.Debug("Compiler output",
			zap.String("output", string(output)),
		)
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			w.logger.Error("Compilation exec error", zap.Error(err))
			return -1, err
		}
	}

	return exitCode, nil
}

// writeProjectFiles writes project files to disk
func (w *DockerWorker) writeProjectFiles(projectDir string, files map[string]string) error {
	for filename, content := range files {
		// Prevent path traversal
		if strings.Contains(filename, "..") {
			w.logger.Warn("Skipping file with path traversal attempt", zap.String("filename", filename))
			continue
		}

		// Strip leading slashes to ensure proper path joining
		cleanFilename := strings.TrimPrefix(filename, "/")
		if cleanFilename == "" {
			w.logger.Warn("Skipping empty filename")
			continue
		}

		filePath := filepath.Join(projectDir, cleanFilename)

		// Create subdirectories if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", cleanFilename, err)
		}

		// Write file
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", cleanFilename, err)
		}

		w.logger.Debug("Wrote project file",
			zap.String("filename", cleanFilename),
			zap.Int("size", len(content)),
		)
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
