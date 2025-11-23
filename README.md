# Design Document: Online LaTeX Code Editor and Renderer
## Technology Stack: React, Go, MongoDB, MinIO, Kong, Docker, TeX Live, Redis, Prometheus, Grafana

## 1. Executive Summary

This document outlines the architecture for a real-time collaborative LaTeX editor using a modern, cloud-native stack. The system leverages Go for high-performance backend services, MongoDB for flexible document storage, MinIO for object storage, and Docker for containerized compilation.

## 2. Revised Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Frontend | React + Monaco Editor | Rich editing experience, component reusability |
| API Gateway | Kong | Production-ready, plugin ecosystem, rate limiting |
| Backend Services | Go (Gin/Echo framework) | High performance, excellent concurrency, low latency |
| Database | MongoDB | Flexible schema for documents, native JSON support |
| Cache | Redis | Fast in-memory operations, pub/sub for WebSockets |
| Object Storage | MinIO | S3-compatible, self-hosted option |
| Message Queue | Redis Streams | Built-in with Redis, simpler architecture |
| Compilation | Docker + TeX Live | Isolated execution, resource control |
| Orchestration | Docker Compose/Kubernetes | Container management and scaling |
| Monitoring | Prometheus + Grafana | Metrics collection and visualization |
| Tracing | OpenTelemetry | Distributed tracing for Go services |

## 3. System Architecture

```
                          ┌─────────────┐
                          │     CDN     │
                          └──────┬──────┘
                                 │
                     ┌───────────▼────────────┐
                     │   Kong API Gateway     │
                     │  (Rate Limiting, Auth) │
                     └───────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
     ┌────────▼────────┐ ┌──────▼──────┐  ┌───────▼────────┐
     │   Auth Service  │ │  WebSocket  │  │ Project Service│
     │      (Go)       │ │  Service(Go)│  │      (Go)      │
     └────────┬────────┘ └──────┬──────┘  └───────┬────────┘
              │                  │                  │
              └──────────────────┼──────────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │  Collaboration Service   │
                    │         (Go)             │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │  Compilation Service     │
                    │         (Go)             │
                    └────────────┬─────────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
   ┌──────▼──────┐     ┌────────▼────────┐    ┌───────▼────────┐
   │   MongoDB   │     │  Redis Cluster  │    │     MinIO      │
   │   Cluster   │     │ (Cache + Queue) │    │ Object Storage │
   └─────────────┘     └─────────────────┘    └────────────────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │   Prometheus + Grafana   │
                    │    (Monitoring Stack)    │
                    └──────────────────────────┘
```

## 4. Component Design with Go and MongoDB

### 4.1 Frontend (React)

**Key Libraries**:
- **Monaco Editor**: Code editing with LaTeX syntax highlighting
- **Yjs**: CRDT-based collaborative editing
- **y-websocket**: WebSocket provider for Yjs
- **PDF.js**: PDF rendering
- **React Query**: API state management and caching
- **Zustand/Redux**: Global state management

**Project Structure**:
```
src/
├── components/
│   ├── Editor/          # Monaco editor wrapper
│   ├── PDFViewer/       # PDF preview component
│   ├── FileTree/        # Project file explorer
│   └── Collaboration/   # User presence, cursors
├── hooks/
│   ├── useWebSocket.ts  # WebSocket connection management
│   ├── useCollaboration.ts # Yjs integration
│   └── useCompilation.ts   # Compilation triggers
├── services/
│   ├── api.ts           # HTTP client (axios/fetch)
│   └── websocket.ts     # WebSocket client
└── store/               # Global state management
```

**Collaboration Implementation**:
```javascript
// Using Yjs for CRDT-based collaboration
const ydoc = new Y.Doc()
const ytext = ydoc.getText('monaco')
const websocketProvider = new WebsocketProvider(
  'wss://api.domain.com/collaboration',
  projectId,
  ydoc,
  { WebSocketPolyfill: WebSocket }
)

// Bind to Monaco editor
const binding = new MonacoBinding(
  ytext,
  editor.getModel(),
  new Set([editor]),
  websocketProvider.awareness
)
```

**Key Design Decisions**:
- Use **Yjs CRDT** instead of OT (simpler, no central authority needed)
- Implement **optimistic UI updates** with rollback
- **Debounce compilation requests** (1 second after last keystroke)
- **IndexedDB** for offline document caching
- **Service Worker** for PWA capabilities

### 4.2 Kong API Gateway Configuration

**Key Plugins**:
```yaml
plugins:
  - name: rate-limiting
    config:
      minute: 100
      hour: 1000
      policy: redis
      
  - name: jwt
    config:
      key_claim_name: kid
      secret_is_base64: false
      
  - name: cors
    config:
      origins: ["https://app.domain.com"]
      methods: ["GET", "POST", "PUT", "DELETE"]
      credentials: true
      
  - name: prometheus
    config:
      per_consumer: true
      
  - name: request-transformer
    config:
      add:
        headers: ["X-Request-ID:$(uuid)"]
```

**Service Routes**:
```yaml
services:
  - name: auth-service
    url: http://auth-service:8080
    routes:
      - name: auth-routes
        paths: ["/api/v1/auth"]
        
  - name: project-service
    url: http://project-service:8080
    routes:
      - name: project-routes
        paths: ["/api/v1/projects"]
        
  - name: websocket-service
    url: http://websocket-service:8080
    routes:
      - name: collaboration-ws
        paths: ["/ws/collaboration"]
        protocols: ["http", "https", "ws", "wss"]
```

**Key Design Decisions**:
- Use **Redis** for distributed rate limiting
- **JWT authentication** with RS256 signing
- **Request ID injection** for distributed tracing
- **Circuit breaker** pattern for downstream services
- **Response caching** for read-heavy endpoints

### 4.3 Go Backend Services Architecture

**Common Service Structure**:
```
service-name/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # Auth, logging, metrics
│   ├── models/              # Domain models
│   ├── repository/          # Database layer (MongoDB)
│   ├── service/             # Business logic
│   └── websocket/           # WebSocket handlers
├── pkg/
│   ├── auth/                # JWT utilities
│   ├── logger/              # Structured logging
│   └── metrics/             # Prometheus metrics
├── Dockerfile
└── go.mod
```

**Key Go Libraries**:
- **Gin** or **Echo**: HTTP framework
- **mongo-go-driver**: MongoDB official driver
- **go-redis/redis**: Redis client with cluster support
- **gorilla/websocket**: WebSocket implementation
- **jwt-go**: JWT token handling
- **prometheus/client_golang**: Metrics
- **uber-go/zap**: Structured logging
- **viper**: Configuration management

### 4.4 Auth Service (Go)

**Responsibilities**:
- User registration and login
- JWT token generation and validation
- Session management in Redis
- Password hashing with bcrypt
- OAuth integration (Google, GitHub)

**MongoDB Schema**:
```javascript
// users collection
{
  _id: ObjectId,
  email: "user@example.com",
  username: "johndoe",
  password_hash: "bcrypt_hash",
  full_name: "John Doe",
  avatar_url: "https://...",
  created_at: ISODate,
  updated_at: ISODate,
  email_verified: true,
  oauth_providers: [
    {
      provider: "google",
      provider_user_id: "12345",
      connected_at: ISODate
    }
  ],
  preferences: {
    default_compiler: "pdflatex",
    theme: "dark"
  }
}
```

**Go Implementation Pattern**:
```go
// internal/handlers/auth_handler.go
type AuthHandler struct {
    authService *service.AuthService
    logger      *zap.Logger
}

func (h *AuthHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    user, token, err := h.authService.Authenticate(
        c.Request.Context(), 
        req.Email, 
        req.Password,
    )
    if err != nil {
        h.logger.Error("Authentication failed", zap.Error(err))
        c.JSON(401, gin.H{"error": "Invalid credentials"})
        return
    }
    
    c.JSON(200, gin.H{
        "user": user,
        "token": token,
        "expires_at": time.Now().Add(15 * time.Minute).Unix(),
    })
}

// internal/service/auth_service.go
type AuthService struct {
    userRepo  *repository.UserRepository
    redisClient *redis.Client
    jwtSecret []byte
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*models.User, string, error) {
    user, err := s.userRepo.FindByEmail(ctx, email)
    if err != nil {
        return nil, "", err
    }
    
    if err := bcrypt.CompareHashAndPassword(
        []byte(user.PasswordHash), 
        []byte(password),
    ); err != nil {
        return nil, "", ErrInvalidCredentials
    }
    
    token, err := s.generateJWT(user)
    if err != nil {
        return nil, "", err
    }
    
    // Store session in Redis
    sessionKey := fmt.Sprintf("session:%s", user.ID.Hex())
    s.redisClient.Set(ctx, sessionKey, token, 24*time.Hour)
    
    return user, token, nil
}
```

**Key Design Decisions**:
- **Bcrypt cost factor**: 12 (balance security and performance)
- **JWT structure**: Access token (15min) + Refresh token (7 days)
- **Redis session storage**: Session key = `session:{user_id}`
- **Token rotation**: Refresh token rotates on each use
- **Middleware**: JWT validation middleware for protected routes

### 4.5 Project Service (Go)

**Responsibilities**:
- Project CRUD operations
- File management within projects
- Project sharing and permissions
- Template management
- Project metadata and settings

**MongoDB Schema**:
```javascript
// projects collection
{
  _id: ObjectId,
  name: "My Research Paper",
  description: "PhD thesis on ML",
  owner_id: ObjectId,  // Reference to users
  collaborators: [
    {
      user_id: ObjectId,
      role: "editor",  // owner, editor, viewer
      invited_at: ISODate,
      accepted_at: ISODate
    }
  ],
  settings: {
    compiler: "pdflatex",
    main_file: "main.tex",
    spell_check: true,
    auto_compile: true
  },
  created_at: ISODate,
  updated_at: ISODate,
  last_compiled_at: ISODate,
  template_id: ObjectId,  // Optional
  file_count: 15,
  total_size_bytes: 524288,
  is_public: false,
  tags: ["research", "machine-learning"]
}

// files collection
{
  _id: ObjectId,
  project_id: ObjectId,  // Reference to projects
  name: "introduction.tex",
  path: "/chapters/introduction.tex",
  content_type: "text/x-tex",
  size_bytes: 4096,
  storage_key: "projects/{project_id}/files/{file_id}",  // MinIO path
  created_at: ISODate,
  updated_at: ISODate,
  created_by: ObjectId,
  version: 5,
  is_binary: false,
  hash: "sha256_hash"  // Content hash for deduplication
}
```

**Go Repository Pattern**:
```go
// internal/repository/project_repository.go
type ProjectRepository struct {
    db *mongo.Database
}

func NewProjectRepository(db *mongo.Database) *ProjectRepository {
    return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *models.Project) error {
    collection := r.db.Collection("projects")
    
    project.ID = primitive.NewObjectID()
    project.CreatedAt = time.Now()
    project.UpdatedAt = time.Now()
    
    _, err := collection.InsertOne(ctx, project)
    return err
}

func (r *ProjectRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Project, error) {
    collection := r.db.Collection("projects")
    
    var project models.Project
    err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&project)
    if err != nil {
        return nil, err
    }
    
    return &project, nil
}

func (r *ProjectRepository) FindByOwner(ctx context.Context, ownerID primitive.ObjectID, page, limit int) ([]*models.Project, error) {
    collection := r.db.Collection("projects")
    
    skip := (page - 1) * limit
    cursor, err := collection.Find(
        ctx,
        bson.M{"owner_id": ownerID},
        options.Find().
            SetSkip(int64(skip)).
            SetLimit(int64(limit)).
            SetSort(bson.D{{"updated_at", -1}}),
    )
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)
    
    var projects []*models.Project
    if err := cursor.All(ctx, &projects); err != nil {
        return nil, err
    }
    
    return projects, nil
}

// Add index for efficient queries
func (r *ProjectRepository) CreateIndexes(ctx context.Context) error {
    collection := r.db.Collection("projects")
    
    indexes := []mongo.IndexModel{
        {
            Keys: bson.D{{"owner_id", 1}, {"updated_at", -1}},
        },
        {
            Keys: bson.D{{"collaborators.user_id", 1}},
        },
        {
            Keys: bson.D{{"tags", 1}},
        },
    }
    
    _, err := collection.Indexes().CreateMany(ctx, indexes)
    return err
}
```

**MinIO Integration**:
```go
// internal/storage/minio_client.go
type MinIOClient struct {
    client *minio.Client
    bucket string
}

func NewMinIOClient(endpoint, accessKey, secretKey, bucket string) (*MinIOClient, error) {
    client, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: true,
    })
    if err != nil {
        return nil, err
    }
    
    return &MinIOClient{
        client: client,
        bucket: bucket,
    }, nil
}

func (m *MinIOClient) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64) error {
    _, err := m.client.PutObject(
        ctx,
        m.bucket,
        objectName,
        reader,
        size,
        minio.PutObjectOptions{
            ContentType: "application/octet-stream",
        },
    )
    return err
}

func (m *MinIOClient) DownloadFile(ctx context.Context, objectName string) (*minio.Object, error) {
    return m.client.GetObject(ctx, m.bucket, objectName, minio.GetObjectOptions{})
}

func (m *MinIOClient) GeneratePresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
    return m.client.PresignedGetObject(ctx, m.bucket, objectName, expiry, nil)
}
```

**Key Design Decisions**:
- **Small files (<100KB)**: Store directly in MongoDB with GridFS
- **Large files (>100KB)**: Store in MinIO with reference in MongoDB
- **MongoDB indexes**: Compound indexes on owner_id + updated_at
- **Pagination**: Cursor-based for large result sets
- **Soft delete**: Add `deleted_at` field instead of hard delete
- **Permissions check**: Middleware validates user access before operations

### 4.6 WebSocket Service (Go)

**Responsibilities**:
- Maintain persistent WebSocket connections
- Route messages to appropriate project rooms
- Broadcast updates to all collaborators
- Handle connection lifecycle (connect, disconnect, reconnect)
- Integrate with Yjs for CRDT synchronization

**Go WebSocket Implementation**:
```go
// internal/websocket/hub.go
type Hub struct {
    rooms      map[string]*Room  // projectID -> Room
    register   chan *Client
    unregister chan *Client
    mu         sync.RWMutex
    redisClient *redis.Client
}

type Room struct {
    ID      string
    clients map[*Client]bool
    mu      sync.RWMutex
}

type Client struct {
    hub        *Hub
    conn       *websocket.Conn
    send       chan []byte
    projectID  string
    userID     string
    userName   string
}

func NewHub(redisClient *redis.Client) *Hub {
    return &Hub{
        rooms:       make(map[string]*Room),
        register:    make(chan *Client),
        unregister:  make(chan *Client),
        redisClient: redisClient,
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.registerClient(client)
            
        case client := <-h.unregister:
            h.unregisterClient(client)
        }
    }
}

func (h *Hub) registerClient(client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    room, exists := h.rooms[client.projectID]
    if !exists {
        room = &Room{
            ID:      client.projectID,
            clients: make(map[*Client]bool),
        }
        h.rooms[client.projectID] = room
        
        // Subscribe to Redis channel for this room
        go h.subscribeToRedis(room)
    }
    
    room.mu.Lock()
    room.clients[client] = true
    room.mu.Unlock()
    
    // Notify others about new user
    h.broadcastToRoom(client.projectID, []byte(fmt.Sprintf(
        `{"type":"user_joined","user_id":"%s","user_name":"%s"}`,
        client.userID, client.userName,
    )))
}

func (h *Hub) broadcastToRoom(projectID string, message []byte) {
    h.mu.RLock()
    room, exists := h.rooms[projectID]
    h.mu.RUnlock()
    
    if !exists {
        return
    }
    
    room.mu.RLock()
    defer room.mu.RUnlock()
    
    for client := range room.clients {
        select {
        case client.send <- message:
        default:
            close(client.send)
            delete(room.clients, client)
        }
    }
    
    // Also publish to Redis for other server instances
    h.redisClient.Publish(
        context.Background(),
        fmt.Sprintf("room:%s", projectID),
        message,
    )
}

// Handle messages from Redis (from other server instances)
func (h *Hub) subscribeToRedis(room *Room) {
    pubsub := h.redisClient.Subscribe(
        context.Background(),
        fmt.Sprintf("room:%s", room.ID),
    )
    defer pubsub.Close()
    
    ch := pubsub.Channel()
    for msg := range ch {
        room.mu.RLock()
        for client := range room.clients {
            select {
            case client.send <- []byte(msg.Payload):
            default:
                close(client.send)
                delete(room.clients, client)
            }
        }
        room.mu.RUnlock()
    }
}

// internal/websocket/client.go
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        
        // Broadcast to room (including Redis for cross-server)
        c.hub.broadcastToRoom(c.projectID, message)
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(54 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    
    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            
            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

**WebSocket Handler**:
```go
// internal/handlers/websocket_handler.go
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
    projectID := c.Param("project_id")
    userID := c.GetString("user_id")  // From JWT middleware
    userName := c.GetString("user_name")
    
    // Verify user has access to this project
    hasAccess, err := h.projectService.VerifyAccess(c.Request.Context(), userID, projectID)
    if err != nil || !hasAccess {
        c.JSON(403, gin.H{"error": "Access denied"})
        return
    }
    
    // Upgrade to WebSocket
    upgrader := websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin: func(r *http.Request) bool {
            return true  // Configure properly in production
        },
    }
    
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    
    client := &Client{
        hub:       h.hub,
        conn:      conn,
        send:      make(chan []byte, 256),
        projectID: projectID,
        userID:    userID,
        userName:  userName,
    }
    
    h.hub.register <- client
    
    go client.writePump()
    go client.readPump()
}
```

**Key Design Decisions**:
- **Goroutines**: One read pump and one write pump per client
- **Buffered channels**: Prevent blocking on slow clients
- **Redis Pub/Sub**: Enable horizontal scaling across multiple servers
- **Heartbeat**: Ping/Pong every 54 seconds (before 60s timeout)
- **Room isolation**: Each project has its own room
- **Graceful shutdown**: Drain connections before server restart

### 4.7 Collaboration Service (Go)

**Responsibilities**:
- Store Yjs document updates
- Provide document state for new connections
- Handle version history snapshots
- Coordinate with WebSocket service

**MongoDB Schema**:
```javascript
// yjs_updates collection
{
  _id: ObjectId,
  project_id: ObjectId,
  document_name: "main.tex",
  update: Binary,  // Yjs update as binary
  clock: NumberLong,  // Yjs clock/version
  created_at: ISODate,
  user_id: ObjectId
}

// yjs_snapshots collection
{
  _id: ObjectId,
  project_id: ObjectId,
  document_name: "main.tex",
  state_vector: Binary,  // Yjs state vector
  snapshot: Binary,  // Full document snapshot
  version: 100,  // Snapshot taken at version 100
  created_at: ISODate,
  size_bytes: 65536
}
```

**Go Implementation**:
```go
// internal/service/collaboration_service.go
type CollaborationService struct {
    db          *mongo.Database
    redisClient *redis.Client
}

func (s *CollaborationService) StoreUpdate(ctx context.Context, projectID, docName string, update []byte, userID string) error {
    collection := s.db.Collection("yjs_updates")
    
    doc := bson.M{
        "project_id":    projectID,
        "document_name": docName,
        "update":        update,
        "clock":         time.Now().UnixNano(),
        "created_at":    time.Now(),
        "user_id":       userID,
    }
    
    _, err := collection.InsertOne(ctx, doc)
    if err != nil {
        return err
    }
    
    // Check if we need a snapshot
    count, _ := collection.CountDocuments(ctx, bson.M{
        "project_id":    projectID,
        "document_name": docName,
    })
    
    if count%100 == 0 {
        go s.createSnapshot(projectID, docName)
    }
    
    return nil
}

func (s *CollaborationService) GetUpdates(ctx context.Context, projectID, docName string, since int64) ([][]byte, error) {
    collection := s.db.Collection("yjs_updates")
    
    cursor, err := collection.Find(ctx, bson.M{
        "project_id":    projectID,
        "document_name": docName,
        "clock":         bson.M{"$gt": since},
    }, options.Find().SetSort(bson.D{{"clock", 1}}))
    
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)
    
    var updates [][]byte
    for cursor.Next(ctx) {
        var doc struct {
            Update []byte `bson:"update"`
        }
        if err := cursor.Decode(&doc); err != nil {
            continue
        }
        updates = append(updates, doc.Update)
    }
    
    return updates, nil
}

func (s *CollaborationService) createSnapshot(projectID, docName string) error {
    // Implementation to create periodic snapshots
    // This reduces the number of updates to replay for new clients
    return nil
}
```

**Key Design Decisions**:
- **Yjs binary format**: Store updates as binary for efficiency
- **Snapshot strategy**: Create snapshot every 100 updates
- **Update retention**: Keep updates for 30 days, then archive
- **MongoDB TTL index**: Automatic cleanup of old updates
- **Caching**: Cache recent document state in Redis

### 4.8 Compilation Service (Go)

**Responsibilities**:
- Receive compilation requests from Redis queue
- Spin up Docker containers with TeX Live
- Execute LaTeX compilation with timeout
- Upload results to MinIO
- Send notifications via WebSocket

**MongoDB Schema**:
```javascript
// compilations collection
{
  _id: ObjectId,
  project_id: ObjectId,
  user_id: ObjectId,
  status: "queued",  // queued, running, completed, failed
  compiler: "pdflatex",
  main_file: "main.tex",
  
  // Results
  output_file_key: "compilations/{compilation_id}/output.pdf",  // MinIO
  log_file_key: "compilations/{compilation_id}/output.log",
  
  // Metrics
  started_at: ISODate,
  completed_at: ISODate,
  duration_ms: 3542,
  
  // Error handling
  error_message: null,
  exit_code: 0,
  
  // Caching
  input_hash: "sha256_of_all_inputs",  // For cache lookup
  
  created_at: ISODate
}
```

**Redis Streams for Job Queue**:
```go
// internal/queue/compilation_queue.go
type CompilationQueue struct {
    redisClient *redis.Client
    streamName  string
}

func (q *CompilationQueue) EnqueueCompilation(ctx context.Context, job *CompilationJob) error {
    jobData, err := json.Marshal(job)
    if err != nil {
        return err
    }
    
    _, err = q.redisClient.XAdd(ctx, &redis.XAddArgs{
        Stream: q.streamName,
        Values: map[string]interface{}{
            "job": jobData,
        },
    }).Result()
    
    return err
}

func (q *CompilationQueue) ConsumeCompilations(ctx context.Context, workerID string, handler func(*CompilationJob) error) {
    // Create consumer group if not exists
    q.redisClient.XGroupCreateMkStream(ctx, q.streamName, "workers", "0")
    
    for {
        streams, err := q.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
            Group:    "workers",
            Consumer: workerID,
            Streams:  []string{q.streamName, ">"},
            Count:    1,
            Block:    0,  // Block indefinitely
        }).Result()
        
        if err != nil {
            time.Sleep(1 * time.Second)
            continue
        }
        
        for _, stream := range streams {
            for _, message := range stream.Messages {
                var job CompilationJob
                jobData := message.Values["job"].(string)
                json.Unmarshal([]byte(jobData), &job)
                
                if err := handler(&job); err != nil {
                    // Retry logic here
                    continue
                }
                
                // Acknowledge message
                q.redisClient.XAck(ctx, q.streamName, "workers", message.ID)
            }
        }
    }
}
```

**Docker Compilation Worker**:
```go
// internal/worker/compilation_worker.go
type CompilationWorker struct {
    dockerClient *client.Client
    minioClient  *MinIOClient
    repo         *repository.CompilationRepository
}

func (w *CompilationWorker) ProcessCompilation(ctx context.Context, job *CompilationJob) error {
    // Update status to running
    w.repo.UpdateStatus(ctx, job.CompilationID, "running")
    
    // Check cache first
    cached, err := w.checkCache(ctx, job.InputHash)
    if err == nil && cached != nil {
        return w.useCachedResult(ctx, job, cached)
    }
    
    // Download project files from MinIO
    projectDir, err := w.downloadProjectFiles(ctx, job.ProjectID)
    if err != nil {
        return err
    }
    defer os.RemoveAll(projectDir)
    
    // Create Docker container
    containerID, err := w.createContainer(ctx, projectDir, job.Compiler)
    if err != nil {
        return err
    }
    defer w.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
    
    // Start compilation with timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    if err := w.dockerClient.ContainerStart(timeoutCtx, containerID, container.StartOptions{}); err != nil {
        return err
    }
    
    // Wait for completion
    statusCh, errCh := w.dockerClient.ContainerWait(timeoutCtx, containerID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        if err != nil {
            return err
        }
    case <-statusCh:
    case <-timeoutCtx.Done():
        return fmt.Errorf("compilation timeout")
    }
    
    // Get exit code
    inspect, err := w.dockerClient.ContainerInspect(ctx, containerID)
    if err != nil {
        return err
    }
    
    // Copy output files
    outputPath := filepath.Join(projectDir, "output.pdf")
    logPath := filepath.Join(projectDir, "output.log")
    
    // Upload to MinIO
    pdfKey := fmt.Sprintf("compilations/%s/output.pdf", job.CompilationID)
    logKey := fmt.Sprintf("compilations/%s/output.log", job.CompilationID)
    
    if err := w.uploadFile(ctx, pdfKey, outputPath); err != nil {
        return err
    }
    if err := w.uploadFile(ctx, logKey, logPath); err != nil {
        return err
    }
    
    // Update compilation record
    w.repo.UpdateCompleted(ctx, job.CompilationID, &CompilationResult{
        OutputFileKey: pdfKey,
        LogFileKey:    logKey,
        ExitCode:      inspect.State.ExitCode,
        DurationMs:    time.Since(job.StartTime).Milliseconds(),
    })
    
    // Cache result
    w.cacheResult(ctx, job.InputHash, pdfKey)
    
    return nil
}

func (w *CompilationWorker) createContainer(ctx context.Context, projectDir, compiler string) (string, error) {
    resp, err := w.dockerClient.ContainerCreate(ctx,
        &container.Config{
            Image: "texlive/texlive:latest",
            Cmd: []string{
                compiler,
                "-interaction=nonstopmode",
                "-output-directory=/workspace",
                "/workspace/main.tex",
            },
            WorkingDir: "/workspace",
        },
        &container.HostConfig{
            Binds: []string{
                fmt.Sprintf("%s:/workspace", projectDir),
            },
            Resources: container.Resources{
                Memory:   2 * 1024 * 1024 * 1024,  // 2GB
                NanoCPUs: 2 * 1e9,                  // 2 CPUs
            },
            NetworkMode: "none",  // No network access
        },
        nil, nil, "",
    )
    
    if err != nil {
        return "", err
    }
    
    return resp.ID, nil
}
```

**Key Design Decisions**:
- **Redis Streams**: Reliable message queue with consumer groups
- **Docker isolation**: Each compilation in separate container
- **Resource limits**: 2GB RAM, 2 CPU cores, 30s timeout
- **No network access**: Security measure for sandboxing
- **Caching**: SHA256 hash of inputs → PDF output mapping
- **Worker pool**: Multiple goroutines consuming from queue
- **Dead letter queue**: Failed jobs after 3 retries
- **Cleanup**: Remove containers and temp files after completion

### 4.9 MongoDB Design Patterns

**Indexing Strategy**:
```go
func SetupIndexes(db *mongo.Database) error {
    // Projects collection
    projectsCollection := db.Collection("projects")
    _, err := projectsCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {Keys: bson.D{{"owner_id", 1}, {"updated_at", -1}}},
        {Keys: bson.D{{"collaborators.user_id", 1}}},
        {Keys: bson.D{{"tags", 1}}},
    })
    
    // Files collection
    filesCollection := db.Collection("files")
    _, err = filesCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {Keys: bson.D{{"project_id", 1}, {"path", 1}}, Options: options.Index().SetUnique(true)},
        {Keys: bson.D{{"hash", 1}}},  // For deduplication
    })
    
    // Yjs updates collection with TTL
    yjsCollection := db.Collection("yjs_updates")
    _, err = yjsCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {Keys: bson.D{{"project_id", 1}, {"document_name", 1}, {"clock", 1}}},
        {Keys: bson.D{{"created_at", 1}}, Options: options.Index().SetExpireAfterSeconds(2592000)},  // 30 days
    })
    
    // Compilations collection
    compilationsCollection := db.Collection("compilations")
    _, err = compilationsCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
        {Keys: bson.D{{"project_id", 1}, {"created_at", -1}}},
        {Keys: bson.D{{"input_hash", 1}}},  // For cache lookup
        {Keys: bson.D{{"status", 1}}},
    })
    
    return err
}
```

**Connection Pooling**:
```go
func NewMongoClient(uri string) (*mongo.Client, error) {
    clientOptions := options.Client().
        ApplyURI(uri).
        SetMaxPoolSize(100).
        SetMinPoolSize(10).
        SetMaxConnIdleTime(30 * time.Second).
        SetServerSelectionTimeout(5 * time.Second)
    
    client, err := mongo.Connect(context.Background(), clientOptions)
    if err != nil {
        return nil, err
    }
    
    // Ping to verify connection
    if err := client.Ping(context.Background(), nil); err != nil {
        return nil, err
    }
    
    return client, nil
}
```

**Transactions for Complex Operations**:
```go
func (s *ProjectService) ShareProject(ctx context.Context, projectID, userID, role string) error {
    session, err := s.db.Client().StartSession()
    if err != nil {
        return err
    }
    defer session.EndSession(ctx)
    
    _, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
        // Update project with new collaborator
        projectCollection := s.db.Collection("projects")
        _, err := projectCollection.UpdateOne(sessCtx,
            bson.M{"_id": projectID},
            bson.M{
                "$push": bson.M{
                    "collaborators": bson.M{
                        "user_id":     userID,
                        "role":        role,
                        "invited_at":  time.Now(),
                        "accepted_at": time.Now(),
                    },
                },
            },
        )
        if err != nil {
            return nil, err
        }
        
        // Log audit event
        auditCollection := s.db.Collection("audit_logs")
        _, err = auditCollection.InsertOne(sessCtx, bson.M{
            "action":     "project_shared",
            "project_id": projectID,
            "user_id":    userID,
            "role":       role,
            "timestamp":  time.Now(),
        })
        
        return nil, err
    })
    
    return err
}
```

### 4.10 Redis Architecture

**Use Cases and Key Patterns**:

```go
// 1. Session Storage
func (s *AuthService) StoreSession(ctx context.Context, userID, token string) error {
    key := fmt.Sprintf("session:%s", userID)
    return s.redisClient.Set(ctx, key, token, 24*time.Hour).Err()
}

// 2. Rate Limiting (Token Bucket)
func (m *RateLimitMiddleware) CheckRateLimit(ctx context.Context, userID string) (bool, error) {
    key := fmt.Sprintf("ratelimit:%s", userID)
    
    // Use Lua script for atomic operations
    script := redis.NewScript(`
        local tokens = redis.call('get', KEYS[1])
        if tokens == false then
            redis.call('set', KEYS[1], ARGV[1] - 1)
            redis.call('expire', KEYS[1], ARGV[2])
            return 1
        end
        if tonumber(tokens) > 0 then
            redis.call('decr', KEYS[1])
            return 1
        end
        return 0
    `)
    
    result, err := script.Run(ctx, m.redisClient, []string{key}, 100, 3600).Int()
    return result == 1, err
}

// 3. Compilation Result Caching
func (w *CompilationWorker) cacheResult(ctx context.Context, inputHash, pdfKey string) error {
    key := fmt.Sprintf("compilation_cache:%s", inputHash)
    return w.redisClient.Set(ctx, key, pdfKey, 1*time.Hour).Err()
}

// 4. WebSocket Pub/Sub
func (h *Hub) PublishMessage(ctx context.Context, roomID string, message []byte) error {
    channel := fmt.Sprintf("room:%s", roomID)
    return h.redisClient.Publish(ctx, channel, message).Err()
}

// 5. Distributed Locking (for critical sections)
func (s *ProjectService) AcquireLock(ctx context.Context, projectID string) (bool, error) {
    key := fmt.Sprintf("lock:project:%s", projectID)
    return s.redisClient.SetNX(ctx, key, "locked", 10*time.Second).Result()
}

// 6. Leaderboard (Sorted Sets)
func (s *AnalyticsService) IncrementCompilations(ctx context.Context, userID string) error {
    return s.redisClient.ZIncrBy(ctx, "compilations_leaderboard", 1, userID).Err()
}
```

**Redis Cluster Configuration**:
```go
func NewRedisClusterClient(addrs []string) *redis.ClusterClient {
    return redis.NewClusterClient(&redis.ClusterOptions{
        Addrs:        addrs,
        PoolSize:     100,
        MinIdleConns: 10,
        MaxRetries:   3,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
    })
}
```

## 5. Monitoring and Observability

### 5.1 Prometheus Metrics in Go

**Custom Metrics**:
```go
// internal/metrics/metrics.go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // HTTP metrics
    HttpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    HttpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )
    
    // WebSocket metrics
    WebSocketConnections = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "websocket_connections_active",
            Help: "Number of active WebSocket connections",
        },
    )
    
    // Compilation metrics
    CompilationsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "compilations_total",
            Help: "Total number of compilations",
        },
        []string{"status"},  // success, failed, timeout
    )
    
    CompilationDuration = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "compilation_duration_seconds",
            Help:    "Compilation duration in seconds",
            Buckets: []float64{1, 3, 5, 10, 20, 30},
        },
    )
    
    // Queue metrics
    QueueDepth = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "compilation_queue_depth",
            Help: "Number of jobs in compilation queue",
        },
    )
    
    // Database metrics
    MongoOperations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mongo_operations_total",
            Help: "Total MongoDB operations",
        },
        []string{"operation", "collection", "status"},
    )
)

// Middleware for automatic HTTP metrics
func MetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start).Seconds()
        status := c.Writer.Status()
        
        HttpRequestsTotal.WithLabelValues(
            c.Request.Method,
            c.FullPath(),
            fmt.Sprintf("%d", status),
        ).Inc()
        
        HttpRequestDuration.WithLabelValues(
            c.Request.Method,
            c.FullPath(),
        ).Observe(duration)
    }
}
```

**Expose Metrics Endpoint**:
```go
func main() {
    r := gin.Default()
    
    // Metrics endpoint
    r.GET("/metrics", gin.WrapH(promhttp.Handler()))
    
    // Apply metrics middleware
    r.Use(MetricsMiddleware())
    
    // ... rest of routes
}
```

### 5.2 Grafana Dashboard Configuration

**Key Dashboards**:

1. **Service Health Dashboard**:
   - Request rate (requests/sec)
   - Error rate (% of 5xx responses)
   - Response time (p50, p95, p99)
   - Active WebSocket connections
   - Service uptime

2. **Compilation Dashboard**:
   - Queue depth over time
   - Compilation success/failure rate
   - Average compilation time
   - Worker utilization
   - Cache hit rate

3. **Database Dashboard**:
   - MongoDB query latency
   - Connection pool usage
   - Index efficiency
   - Document count per collection
   - Slow queries

4. **Infrastructure Dashboard**:
   - CPU and memory usage per service
   - Network I/O
   - Disk usage (MongoDB, MinIO)
   - Container health status

### 5.3 Structured Logging

```go
import "go.uber.org/zap"

func InitLogger() *zap.Logger {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout", "/var/log/app.log"}
    
    logger, _ := config.Build()
    return logger
}

// Usage
logger.Info("Compilation started",
    zap.String("project_id", projectID),
    zap.String("user_id", userID),
    zap.String("compiler", "pdflatex"),
)

logger.Error("Compilation failed",
    zap.String("project_id", projectID),
    zap.Error(err),
    zap.Int("exit_code", exitCode),
)
```

### 5.4 Distributed Tracing with OpenTelemetry

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func (s *CompilationService) Compile(ctx context.Context, job *CompilationJob) error {
    tracer := otel.Tracer("compilation-service")
    ctx, span := tracer.Start(ctx, "CompileProject")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("project_id", job.ProjectID),
        attribute.String("compiler", job.Compiler),
    )
    
    // Download files (child span)
    ctx, downloadSpan := tracer.Start(ctx, "DownloadProjectFiles")
    files, err := s.downloadFiles(ctx, job.ProjectID)
    downloadSpan.End()
    if err != nil {
        span.RecordError(err)
        return err
    }
    
    // Execute compilation (child span)
    ctx, compileSpan := tracer.Start(ctx, "ExecuteCompilation")
    result, err := s.runCompiler(ctx, files, job.Compiler)
    compileSpan.End()
    
    return err
}
```

## 6. Deployment Architecture

### 6.1 Docker Compose for Development

```yaml
version: '3.8'

services:
  # Kong API Gateway
  kong:
    image: kong:3.4
    environment:
      KONG_DATABASE: postgres
      KONG_PG_HOST: kong-database
      KONG_PG_DATABASE: kong
      KONG_PROXY_ACCESS_LOG: /dev/stdout
      KONG_ADMIN_ACCESS_LOG: /dev/stdout
    ports:
      - "8000:8000"
      - "8001:8001"
    depends_on:
      - kong-database
  
  kong-database:
    image: postgres:15
    environment:
      POSTGRES_DB: kong
      POSTGRES_USER: kong
      POSTGRES_PASSWORD: kong
  
  # MongoDB
  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: password
    volumes:
      - mongo-data:/data/db
  
  # Redis Cluster (simplified single node for dev)
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
  
  # MinIO
  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio-data:/data
  
  # Auth Service
  auth-service:
    build: ./services/auth
    ports:
      - "8080:8080"
    environment:
      MONGO_URI: mongodb://admin:password@mongodb:27017
      REDIS_ADDR: redis:6379
      JWT_SECRET: your-secret-key
    depends_on:
      - mongodb
      - redis
  
  # Project Service
  project-service:
    build: ./services/project
    ports:
      - "8081:8080"
    environment:
      MONGO_URI: mongodb://admin:password@mongodb:27017
      REDIS_ADDR: redis:6379
      MINIO_ENDPOINT: minio:9000
    depends_on:
      - mongodb
      - redis
      - minio
  
  # WebSocket Service
  websocket-service:
    build: ./services/websocket
    ports:
      - "8082:8080"
    environment:
      REDIS_ADDR: redis:6379
      MONGO_URI: mongodb://admin:password@mongodb:27017
    depends_on:
      - redis
      - mongodb
  
  # Compilation Service
  compilation-service:
    build: ./services/compilation
    ports:
      - "8083:8080"
    environment:
      MONGO_URI: mongodb://admin:password@mongodb:27017
      REDIS_ADDR: redis:6379
      MINIO_ENDPOINT: minio:9000
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - mongodb
      - redis
      - minio
  
  # Prometheus
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
  
  # Grafana
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - grafana-data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  mongo-data:
  redis-data:
  minio-data:
  prometheus-data:
  grafana-data:
```

### 6.2 Kubernetes Production Deployment

**Sample Service Deployment**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: project-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: project-service
  template:
    metadata:
      labels:
        app: project-service
    spec:
      containers:
      - name: project-service
        image: your-registry/project-service:v1.0
        ports:
        - containerPort: 8080
        env:
        - name: MONGO_URI
          valueFrom:
            secretKeyRef:
              name: mongo-credentials
              key: uri
        - name: REDIS_ADDR
          value: redis-service:6379
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: project-service
spec:
  selector:
    app: project-service
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

## 7. Security Architecture

### 7.1 JWT Authentication Flow

```go
// Generate JWT
func (s *AuthService) GenerateJWT(user *models.User) (string, error) {
    claims := jwt.MapClaims{
        "user_id":  user.ID.Hex(),
        "username": user.Username,
        "email":    user.Email,
        "exp":      time.Now().Add(15 * time.Minute).Unix(),
        "iat":      time.Now().Unix(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(s.privateKey)
}

// Validate JWT Middleware
func JWTMiddleware(publicKey *rsa.PublicKey) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "No authorization header"})
            c.Abort()
            return
        }
        
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                return nil, fmt.Errorf("unexpected signing method")
            }
            return publicKey, nil
        })
        
        if err != nil || !token.Valid {
            c.JSON(401, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }
        
        claims := token.Claims.(jwt.MapClaims)
        c.Set("user_id", claims["user_id"])
        c.Set("username", claims["username"])
        c.Next()
    }
}
```

### 7.2 Input Validation

```go
// Request validation
type CreateProjectRequest struct {
    Name        string `json:"name" binding:"required,min=1,max=100"`
    Description string `json:"description" binding:"max=500"`
    Compiler    string `json:"compiler" binding:"required,oneof=pdflatex xelatex lualatex"`
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
    var req CreateProjectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Sanitize inputs
    req.Name = html.EscapeString(req.Name)
    req.Description = html.EscapeString(req.Description)
    
    // Continue with business logic
}
```

### 7.3 Compilation Sandboxing

**Docker Security**:
- Read-only root filesystem
- No network access
- Limited CPU and memory
- User namespace isolation
- AppArmor/SELinux profiles

```go
func (w *CompilationWorker) createSecureContainer(ctx context.Context, projectDir string) (string, error) {
    resp, err := w.dockerClient.ContainerCreate(ctx,
        &container.Config{
            Image: "texlive/texlive:latest",
            Cmd:   []string{"pdflatex", "-interaction=nonstopmode", "main.tex"},
            User:  "65534:65534",  // nobody user
        },
        &container.HostConfig{
            Binds: []string{
                fmt.Sprintf("%s:/workspace:ro", projectDir),  // Read-only
            },
            Resources: container.Resources{
                Memory:     2 * 1024 * 1024 * 1024,
                NanoCPUs:   2 * 1e9,
                PidsLimit:  ptrInt64(256),
            },
            NetworkMode: "none",
            ReadonlyRootfs: true,
            Tmpfs: map[string]string{
                "/tmp": "rw,noexec,nosuid,size=512m",
            },
            SecurityOpt: []string{
                "no-new-privileges",
                "apparmor=docker-default",
            },
        },
        nil, nil, "",
    )
    
    return resp.ID, err
}
```

## 8. Performance Optimization Strategies

### 8.1 Caching Layers

```
Browser Cache (Static Assets)
       ↓
CDN Cache (Compiled PDFs, Assets)
       ↓
Redis Cache (Sessions, Metadata, Compilations)
       ↓
MongoDB (Persistent Data)
       ↓
MinIO (File Storage)
```

### 8.2 Database Optimization

- **Connection pooling**: 100 max connections, 10 min idle
- **Indexes**: Compound indexes on frequently queried fields
- **Read preference**: Secondary reads for analytics
- **Write concern**: Majority for critical operations
- **Projection**: Fetch only required fields
- **Aggregation pipeline**: Server-side data processing

### 8.3 Go Performance Best Practices

- **Goroutine pooling**: Limit concurrent goroutines
- **Context propagation**: Proper cancellation and timeouts
- **Memory pooling**: Reuse buffers with `sync.Pool`
- **Profiling**: Use pprof for CPU and memory profiling
- **Benchmarking**: Regular benchmark tests

```go
// Goroutine pool example
type WorkerPool struct {
    tasks   chan Task
    workers int
}

func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        go func() {
            for task := range p.tasks {
                task.Execute()
            }
        }()
    }
}
```

## 9. Scalability Roadmap

### Phase 1: Initial Launch (0-10K users)
- Single region deployment
- 3 replicas per service
- MongoDB replica set (1 primary, 2 secondaries)
- Redis single instance

### Phase 2: Growth (10K-100K users)
- Multi-region deployment with CDN
- Horizontal scaling (10+ replicas)
- MongoDB sharding by user_id
- Redis cluster (3+ nodes)
- Dedicated compilation worker fleet

### Phase 3: Scale (100K+ users)
- Multi-cloud deployment
- Kubernetes auto-scaling
- MongoDB sharded cluster (multiple shards)
- Redis cluster with sentinel
- Regional compilation worker pools
- Advanced caching strategies

This architecture provides a robust foundation for building a scalable, performant online LaTeX editor using Go, MongoDB, and modern cloud-native technologies. 
Start The Implementation at production Level with Complete Implementation and verification
