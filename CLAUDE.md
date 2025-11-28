# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TexFlow is an online collaborative LaTeX editor built with a microservices architecture. It enables real-time collaborative editing with live PDF preview using Yjs CRDT for conflict-free synchronization.

## Build and Development Commands

### Primary Commands (Makefile)
```bash
make build              # Build all Go services into bin/
make test               # Run all tests (go test ./... for each service)
make lint               # Run golangci-lint on all services
make docker-up          # Start entire stack with Docker Compose
make docker-down        # Stop all services
make docker-logs        # View logs from all services
make generate-keys      # Generate JWT RSA key pairs
make copy-keys          # Copy JWT keys to service directories
```

### Run Individual Services
```bash
make run-auth           # Auth service (port 8080)
make run-project        # Project service (port 8081)
make run-websocket      # WebSocket service (port 8082)
make run-collaboration  # Collaboration service (port 8083)
make run-compilation    # Compilation service (port 8084)
```

### Frontend Commands
```bash
cd frontend
npm install             # Install dependencies
npm run dev             # Vite dev server (port 3000)
npm run build           # Production build
npm run lint            # ESLint + TypeScript validation
```

## Architecture

```
┌─────────────────┐
│   React Client  │
└────────┬────────┘
         │
┌────────▼────────┐
│  Kong Gateway   │  (port 8000 - routing, rate limiting, CORS)
└────────┬────────┘
         │
    ┌────┴────┬────────────┬──────────────┬────────────┐
    ▼         ▼            ▼              ▼            ▼
┌───────┐ ┌───────┐ ┌───────────┐ ┌────────────┐ ┌───────────┐
│ Auth  │ │Project│ │ WebSocket │ │Collaboration│ │Compilation│
│ 8080  │ │ 8081  │ │   8082    │ │    8083    │ │   8084    │
└───┬───┘ └───┬───┘ └─────┬─────┘ └─────┬──────┘ └─────┬─────┘
    └─────────┴───────────┴─────────────┴──────────────┘
                          │
         ┌────────────────┼────────────────┐
         ▼                ▼                ▼
    ┌────────┐       ┌────────┐       ┌────────┐
    │MongoDB │       │ Redis  │       │ MinIO  │
    │  7.0   │       │  7.0   │       │(S3-API)│
    └────────┘       └────────┘       └────────┘
```

### Service Responsibilities
- **Auth**: User registration, JWT authentication (RS256), token refresh
- **Project**: Project/file CRUD, file storage via MinIO, sharing/permissions
- **WebSocket**: Real-time connections, Yjs update relay, presence/cursors
- **Collaboration**: Yjs CRDT state persistence, snapshots, version tracking
- **Compilation**: LaTeX→PDF via Docker sandbox, job queue (Redis Streams)

### Communication Patterns
- **REST API**: Frontend → Kong → Services (JSON, JWT in Authorization header)
- **WebSocket**: Frontend ↔ WebSocket Service (Yjs binary updates, presence)
- **Pub/Sub**: Redis for WebSocket horizontal scaling and event broadcasting
- **Job Queue**: Redis Streams for compilation job processing

## Tech Stack

### Backend (Go 1.25)
- Gin web framework
- MongoDB driver (`go.mongodb.org/mongo-driver`)
- Redis client (`github.com/redis/go-redis/v9`)
- JWT handling (`github.com/golang-jwt/jwt/v5`)
- Gorilla WebSocket
- Docker API (compilation service)

### Frontend (React 18 + TypeScript)
- Vite build tool
- Monaco Editor (code editing)
- Yjs + y-websocket + y-monaco (real-time collaboration)
- Tailwind CSS
- React Router DOM v6
- React PDF (PDF preview)

## Code Patterns

### Go Service Structure
Each service follows this pattern:
```
services/{service}/
├── cmd/main.go           # Entry point, server setup
├── internal/
│   ├── handlers/         # HTTP handlers (Gin)
│   ├── service/          # Business logic
│   ├── repository/       # MongoDB data access
│   └── middleware/       # Auth, logging middleware
├── config/               # Configuration loading
└── go.mod
```

### JWT Authentication
Services use RS256 JWT tokens. Protected routes require:
```go
authGroup := router.Group("/api/v1")
authGroup.Use(middleware.AuthMiddleware(jwtService))
```

### MongoDB Repository Pattern
```go
type ProjectRepository struct {
    collection *mongo.Collection
}

func (r *ProjectRepository) Create(ctx context.Context, project *models.Project) error {
    _, err := r.collection.InsertOne(ctx, project)
    return err
}
```

## API Routes

All services expose `/health` and `/metrics` endpoints. Main routes through Kong (port 8000):

- `POST /api/v1/auth/register|login|refresh|logout`
- `GET/POST/PUT/DELETE /api/v1/projects[/:id]`
- `WS /ws/:project_id?token=JWT`
- `POST /api/v1/compilation/compile`

## Configuration

Environment variables (see `.env.example`):
- `MONGO_URI`, `MONGO_DATABASE`
- `REDIS_ADDR`, `REDIS_PASSWORD`
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`
- `JWT_SECRET`, `JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`
- `COMPILATION_TIMEOUT=30s`, `COMPILATION_MEMORY_LIMIT=2147483648`

## Docker Compose Services

Infrastructure: MongoDB (27018), Redis (6380), MinIO (9000/9001), PostgreSQL (Kong DB), Prometheus (9095), Grafana (3000)

## Key Implementation Details

- **Compilation Isolation**: Each compilation runs in a Docker container with no network, 2GB RAM limit, 2 CPUs, 30s timeout
- **CRDT Persistence**: Yjs updates stored in MongoDB with snapshots every 100 updates
- **Token Security**: 15-minute access tokens, 7-day refresh tokens, bcrypt (cost 12) for passwords
- **Rate Limiting**: 1000 requests/minute per service via Kong
