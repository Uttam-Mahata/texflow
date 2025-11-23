# TexFlow - Online LaTeX Code Editor and Renderer

A production-ready, real-time collaborative LaTeX editor built with modern cloud-native technologies.

## 🚀 Features

- **Real-time Collaboration**: Multiple users can edit documents simultaneously using CRDTs (Yjs)
- **LaTeX Compilation**: Support for pdflatex, xelatex, and lualatex
- **Cloud Storage**: Secure file storage with MinIO (S3-compatible)
- **User Authentication**: JWT-based authentication with refresh tokens
- **Project Management**: Create, share, and manage LaTeX projects
- **Monitoring**: Built-in Prometheus metrics and Grafana dashboards
- **API Gateway**: Kong gateway with rate limiting and authentication
- **Microservices**: Scalable microservices architecture in Go

## 🏗️ Architecture

```
┌─────────────┐
│   Frontend  │ (React + Monaco Editor)
└──────┬──────┘
       │
┌──────▼──────────┐
│  Kong Gateway   │ (Rate Limiting, Auth, Routing)
└──────┬──────────┘
       │
┌──────┴────────────────────────────────────┐
│                                           │
│  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │   Auth   │  │ Project  │  │ WebSocket│ │
│  │ Service  │  │ Service  │  │ Service  │ │
│  └──────────┘  └──────────┘  └─────────┘ │
│                                           │
│  ┌──────────┐  ┌──────────┐              │
│  │Collabora-│  │Compila-  │              │
│  │tion Svc  │  │tion Svc  │              │
│  └──────────┘  └──────────┘              │
│                                           │
└───────────────────────────────────────────┘
       │
┌──────┴────────────────────────────────────┐
│  ┌────────┐  ┌──────┐  ┌──────┐          │
│  │MongoDB │  │ Redis│  │ MinIO│          │
│  └────────┘  └──────┘  └──────┘          │
└───────────────────────────────────────────┘
```

## 📋 Prerequisites

- **Docker** and **Docker Compose** (20.10+)
- **Go** 1.21+ (for local development)
- **Node.js** 18+ and npm (for frontend development)
- **Make** (optional, for convenience commands)

## 🚀 Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/yourusername/texflow.git
cd texflow
```

### 2. Generate JWT keys

```bash
make generate-keys
```

Or manually:

```bash
mkdir -p keys
openssl genrsa -out keys/jwt-private.pem 4096
openssl rsa -in keys/jwt-private.pem -pubout -out keys/jwt-public.pem
```

### 3. Configure environment variables

```bash
cp .env.example .env
# Edit .env with your configuration
```

### 4. Start all services with Docker Compose

```bash
make docker-up
# OR
docker-compose -f deployments/docker/docker-compose.yml up -d
```

### 5. Verify services are running

```bash
docker-compose -f deployments/docker/docker-compose.yml ps
```

All services should show as "healthy" after a few moments.

### 6. Access the services

- **Auth Service**: http://localhost:8080
- **Project Service**: http://localhost:8081
- **WebSocket Service**: http://localhost:8082
- **Collaboration Service**: http://localhost:8083
- **Compilation Service**: http://localhost:8084
- **Kong API Gateway**: http://localhost:8000
- **Kong Admin API**: http://localhost:8001
- **MinIO Console**: http://localhost:9001
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000

## 🛠️ Development

### Build all services

```bash
make build
```

### Run individual services locally

```bash
make run-auth
make run-project
make run-websocket
make run-collaboration
make run-compilation
```

### Run tests

```bash
make test
```

### View logs

```bash
make docker-logs
```

## 📚 API Documentation

### Authentication Endpoints

#### Register a new user

```bash
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "securepassword123",
  "full_name": "John Doe"
}
```

#### Login

```bash
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

Response:

```json
{
  "user": {
    "id": "...",
    "email": "user@example.com",
    "username": "johndoe",
    "full_name": "John Doe"
  },
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 900
}
```

#### Get current user

```bash
GET /api/v1/auth/me
Authorization: Bearer <access_token>
```

### Project Endpoints

#### Create a project

```bash
POST /api/v1/projects
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "My Research Paper",
  "description": "PhD thesis on Machine Learning",
  "compiler": "pdflatex",
  "is_public": false,
  "tags": ["research", "ml"]
}
```

#### List user projects

```bash
GET /api/v1/projects?page=1&limit=10
Authorization: Bearer <access_token>
```

#### Get a project

```bash
GET /api/v1/projects/:id
Authorization: Bearer <access_token>
```

## 🏗️ Project Structure

```
texflow/
├── services/
│   ├── auth/                 # Authentication service
│   ├── project/              # Project management service
│   ├── websocket/            # WebSocket service for real-time features
│   ├── collaboration/        # Collaboration service (Yjs)
│   └── compilation/          # LaTeX compilation service
├── frontend/                 # React frontend
├── infrastructure/
│   ├── kong/                 # Kong API Gateway configuration
│   ├── prometheus/           # Prometheus configuration
│   └── grafana/              # Grafana dashboards
├── deployments/
│   ├── docker/               # Docker Compose files
│   └── kubernetes/           # Kubernetes manifests
├── docs/                     # Documentation
├── Makefile                  # Build and deployment commands
└── README.md
```

## 🔧 Configuration

### Environment Variables

All services can be configured via environment variables. See `.env.example` for a complete list.

Key variables:

- `MONGO_URI`: MongoDB connection string
- `REDIS_ADDR`: Redis address
- `MINIO_ENDPOINT`: MinIO endpoint
- `JWT_SECRET`: Secret for JWT signing (use RSA keys in production)
- `COMPILATION_TIMEOUT`: Maximum compilation time (default: 30s)

## 📊 Monitoring

### Prometheus Metrics

Each service exposes metrics at `/metrics`. Key metrics include:

- HTTP request rates and latencies
- Database operation metrics
- Active WebSocket connections
- Compilation queue depth and duration
- Cache hit rates

### Grafana Dashboards

Access Grafana at http://localhost:3000 (default credentials: admin/admin)

Pre-configured dashboards:

1. **Service Health Dashboard**: Overview of all services
2. **Compilation Metrics**: Compilation performance and queue status
3. **Database Performance**: MongoDB and Redis metrics
4. **Infrastructure Overview**: CPU, memory, and network usage

## 🔒 Security

- JWT authentication with RS256 signing
- Password hashing with bcrypt (cost factor: 12)
- Docker container isolation for compilation
- No network access for compilation containers
- Rate limiting via Kong API Gateway
- CORS configuration
- Input validation and sanitization

## 🚢 Deployment

### Docker Compose (Development/Testing)

```bash
docker-compose -f deployments/docker/docker-compose.yml up -d
```

### Kubernetes (Production)

```bash
kubectl apply -f deployments/kubernetes/
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [TeX Live](https://www.tug.org/texlive/) for LaTeX compilation
- [Yjs](https://yjs.dev/) for CRDT-based collaboration
- [Monaco Editor](https://microsoft.github.io/monaco-editor/) for code editing
- [Kong](https://konghq.com/) for API Gateway
- [MinIO](https://min.io/) for object storage

## 📧 Contact

For questions or support, please open an issue on GitHub.

---

**Built with ❤️ using Go, React, MongoDB, and Docker**
