# LaTeX Rendering System - Complete Guide

This guide provides comprehensive information about the LaTeX rendering system in TexFlow.

## Quick Links

- **Quick Start**: See [VERIFICATION_SUMMARY.md](../VERIFICATION_SUMMARY.md)
- **Technical Details**: See [LATEX_RENDERING_VERIFICATION.md](../LATEX_RENDERING_VERIFICATION.md)
- **Test Results**: See [VERIFICATION_RESULTS.md](../VERIFICATION_RESULTS.md)
- **Automated Testing**: Run `scripts/verify_latex_rendering.sh`

## How It Works

### User Perspective

1. **Edit LaTeX**: Write your LaTeX code in the Monaco editor
2. **Select Compiler**: Choose pdflatex, xelatex, or lualatex
3. **Compile**: Click the "Compile" button
4. **View PDF**: PDF appears in the right panel
5. **Download**: Download the PDF if needed

### Technical Flow

```
User Action → Frontend API Call → Backend Service → Docker Container → PDF Output
```

#### Detailed Steps:

1. **Frontend** (`CompilationPanel`):
   - User clicks "Compile"
   - Sends POST request to `/api/v1/compilation/compile`
   - Includes: project ID, compiler choice, main file

2. **API Gateway** (Kong):
   - Validates JWT token
   - Routes to compilation service
   - Applies rate limiting

3. **Compilation Service**:
   - Validates user has access to project
   - Retrieves LaTeX files from MinIO
   - Queues compilation job in Redis
   - Returns job ID to frontend

4. **Worker Pool**:
   - Worker picks up job from Redis queue
   - Creates isolated Docker container
   - Mounts project files
   - Executes LaTeX compiler

5. **Docker Container**:
   - Runs TeX Live compiler
   - No network access (security)
   - Resource limits enforced
   - Generates PDF

6. **Post-Processing**:
   - Worker extracts PDF from container
   - Uploads PDF to MinIO
   - Updates compilation status in MongoDB
   - Logs compilation result

7. **Frontend Updates**:
   - Polls compilation status every 2 seconds
   - Receives PDF URL when complete
   - PDFViewer displays the PDF
   - Shows logs if errors occur

## Architecture Components

### Frontend Components

#### CompilationPanel
- **Location**: `frontend/src/components/CompilationPanel.tsx`
- **Purpose**: User interface for triggering compilation
- **Features**:
  - Compiler selection dropdown
  - Main file input
  - Compile button with status
  - Log viewer
  - Error display

#### PDFViewer
- **Location**: `frontend/src/components/PDFViewer.tsx`
- **Purpose**: Display compiled PDF documents
- **Features**:
  - PDF rendering with react-pdf
  - Zoom in/out controls
  - Page navigation
  - Download button
  - Loading and error states

#### Editor
- **Location**: `frontend/src/pages/Editor.tsx`
- **Purpose**: Main editing environment
- **Layout**:
  ```
  ┌─────────┬──────────┬──────────┬──────────────┐
  │  File   │  Monaco  │   PDF    │ Compilation  │
  │  Tree   │  Editor  │  Viewer  │    Panel     │
  └─────────┴──────────┴──────────┴──────────────┘
  ```

### Backend Services

#### Compilation Service
- **Location**: `services/compilation/`
- **Language**: Go
- **Port**: 8084
- **Responsibilities**:
  - Handle compilation requests
  - Manage job queue
  - Coordinate with Docker
  - Store results

#### Docker Worker
- **File**: `services/compilation/internal/worker/docker_worker.go`
- **Purpose**: Execute LaTeX compilation in isolation
- **Process**:
  1. Pull TeX Live image (if needed)
  2. Create container with security constraints
  3. Mount project files
  4. Run compiler
  5. Extract PDF
  6. Clean up container

### Infrastructure

#### MongoDB
- **Purpose**: Store compilation metadata
- **Collections**:
  - `compilations`: Job status and results
  - `projects`: Project information
  - `files`: File metadata

#### Redis
- **Purpose**: Job queue and caching
- **Streams**:
  - `compilation:queue`: Pending jobs
  - `compilation:processing`: Active jobs
- **Cache**: Compilation results (if enabled)

#### MinIO
- **Purpose**: S3-compatible file storage
- **Buckets**:
  - `texflow`: Project files and PDFs
- **Structure**:
  ```
  texflow/
  ├── projects/
  │   └── {project_id}/
  │       ├── files/
  │       │   └── *.tex
  │       └── output/
  │           └── *.pdf
  ```

#### Docker
- **Purpose**: Isolated LaTeX compilation
- **Image**: `texlive/texlive:latest` (~4GB)
- **Configuration**:
  - Network: none (no network access)
  - Memory: 2GB limit
  - CPU: 2 cores
  - Timeout: 60 seconds

## Security

### Container Isolation
- Each compilation runs in isolated container
- No network access to prevent data exfiltration
- Read-only root filesystem
- Temporary writable directories only

### Resource Limits
- **Memory**: 2GB per compilation
- **CPU**: 2 cores per compilation
- **Time**: 60-second timeout
- **Prevents**: Resource exhaustion attacks

### Authentication
- JWT tokens required for all API calls
- User can only compile their own projects
- Project ownership validated before compilation

### Input Validation
- Compiler type validated (whitelist)
- File paths sanitized
- Project IDs validated
- File sizes checked

## Configuration

### Environment Variables

#### Compilation Service
```bash
# Service Configuration
COMPILATION_SERVICE_PORT=8084
ENVIRONMENT=production

# Database
MONGO_URI=mongodb://admin:password@mongodb:27017
MONGO_DATABASE=texflow

# Cache
REDIS_ADDR=redis:6379
REDIS_PASSWORD=redispassword

# Storage
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=texflow
MINIO_USE_SSL=false

# Compilation
COMPILATION_TIMEOUT=60s
COMPILATION_MEMORY_LIMIT=2147483648  # 2GB
COMPILATION_CPU_LIMIT=2
MAX_COMPILATION_WORKERS=10
TEXLIVE_IMAGE=texlive/texlive:latest

# Authentication
JWT_SECRET=your-secret-key
JWT_PRIVATE_KEY_PATH=./keys/jwt-private.pem
JWT_PUBLIC_KEY_PATH=./keys/jwt-public.pem
```

### Docker Compose
See `deployments/docker/docker-compose.yml` for complete configuration.

## API Reference

### Compile LaTeX Document

**Endpoint**: `POST /api/v1/compilation/compile`

**Headers**:
```
Authorization: Bearer {jwt_token}
Content-Type: application/json
```

**Request Body**:
```json
{
  "project_id": "507f1f77bcf86cd799439011",
  "compiler": "pdflatex",
  "main_file": "main.tex"
}
```

**Response** (202 Accepted):
```json
{
  "id": "507f1f77bcf86cd799439012",
  "project_id": "507f1f77bcf86cd799439011",
  "user_id": "507f1f77bcf86cd799439013",
  "status": "queued",
  "compiler": "pdflatex",
  "main_file": "main.tex",
  "created_at": "2024-12-07T10:52:00Z"
}
```

### Get Compilation Status

**Endpoint**: `GET /api/v1/compilation/:id`

**Headers**:
```
Authorization: Bearer {jwt_token}
```

**Response** (200 OK):
```json
{
  "id": "507f1f77bcf86cd799439012",
  "status": "completed",
  "output_url": "http://minio:9000/texflow/projects/.../output.pdf",
  "duration_ms": 2500,
  "log": "This is pdfTeX, Version 3.141592653...",
  "completed_at": "2024-12-07T10:52:03Z"
}
```

**Status Values**:
- `queued`: Waiting in queue
- `running`: Currently compiling
- `completed`: Success, PDF available
- `failed`: Compilation error
- `timeout`: Exceeded time limit

## Testing

### Automated Verification
```bash
# Run complete verification
./scripts/verify_latex_rendering.sh

# Expected output: All checks pass with ✓
```

### Manual Testing
```bash
# 1. Start services
docker compose up -d

# 2. Create test project
curl -X POST http://localhost:8000/api/v1/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Project",
    "compiler": "pdflatex"
  }'

# 3. Upload LaTeX file
# Use the sample from scripts/sample_template.tex

# 4. Compile
curl -X POST http://localhost:8000/api/v1/compilation/compile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "...",
    "compiler": "pdflatex",
    "main_file": "main.tex"
  }'

# 5. Check status
curl http://localhost:8000/api/v1/compilation/{id} \
  -H "Authorization: Bearer $TOKEN"
```

### Sample LaTeX Document
See `scripts/sample_template.tex` for a simple test document.

## Troubleshooting

### Compilation Fails
1. **Check logs**: View compilation log in UI
2. **Verify syntax**: Ensure LaTeX is valid
3. **Check packages**: Verify packages are in TeX Live
4. **Check timeout**: Large documents may need more time

### PDF Not Displaying
1. **Check URL**: Verify output_url is accessible
2. **Check MinIO**: Ensure MinIO is running
3. **Check CORS**: Verify CORS headers are set
4. **Check network**: Ensure frontend can reach MinIO

### Slow Compilation
1. **Check queue**: View queue status
2. **Check workers**: Ensure workers are running
3. **Check resources**: Monitor CPU/memory usage
4. **Scale workers**: Increase MAX_COMPILATION_WORKERS

### Container Issues
1. **Check Docker**: Ensure Docker daemon is running
2. **Check image**: Verify TeX Live image is pulled
3. **Check permissions**: Ensure service can access Docker socket
4. **Check logs**: View Docker container logs

## Performance Tuning

### Increase Workers
```bash
# In docker-compose.yml or environment
MAX_COMPILATION_WORKERS=20
```

### Enable Caching
```bash
# Enable result caching
ENABLE_CACHE=true
CACHE_TTL=3600  # 1 hour
```

### Optimize Docker
```bash
# Pre-pull TeX Live image
docker pull texlive/texlive:latest

# Use local Docker registry
# Point TEXLIVE_IMAGE to local registry
```

## Monitoring

### Metrics Endpoints
- `/metrics`: Prometheus metrics
- `/health`: Health check
- `/api/v1/compilation/stats`: Compilation statistics
- `/api/v1/compilation/queue`: Queue status

### Key Metrics
- `compilation_total`: Total compilations
- `compilation_duration_seconds`: Compilation time
- `compilation_errors_total`: Failed compilations
- `queue_depth`: Current queue size
- `active_workers`: Workers in use

## Support

### Documentation
- `VERIFICATION_SUMMARY.md`: Quick reference
- `VERIFICATION_RESULTS.md`: Test results
- `LATEX_RENDERING_VERIFICATION.md`: Technical details
- This guide

### Verification Script
```bash
./scripts/verify_latex_rendering.sh
```

### Sample Document
```bash
cat scripts/sample_template.tex
```

## Contributing

When modifying the LaTeX rendering system:

1. Update documentation
2. Run verification script
3. Test with sample document
4. Check security implications
5. Update monitoring/metrics
6. Document breaking changes

## License

See [LICENSE](../LICENSE) file in project root.

---

**Last Updated**: December 7, 2024  
**Status**: Verified and Production Ready ✅
