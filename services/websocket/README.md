# WebSocket Service

The WebSocket Service provides real-time bidirectional communication for the TexFlow collaborative LaTeX editor. It enables multiple users to collaborate on documents in real-time, with features like cursor tracking, presence awareness, and live document updates.

## Features

- **Real-time Communication**: Persistent WebSocket connections for instant message delivery
- **Room Management**: Automatic room creation and cleanup for project-based collaboration
- **Horizontal Scaling**: Redis Pub/Sub enables multiple WebSocket server instances
- **User Presence**: Track which users are currently active in a project
- **Cursor Tracking**: Share cursor positions between collaborators
- **Heartbeat Mechanism**: Automatic connection health monitoring with ping/pong
- **Message Types**: Support for various message types (Yjs updates, cursor updates, presence, etc.)
- **Authentication**: JWT-based authentication for secure connections
- **Metrics**: Prometheus metrics for monitoring connection stats

## Architecture

```
Client (Browser)
    ↓ WebSocket Upgrade
WebSocket Handler
    ↓ Register Client
Hub (Room Manager)
    ├─→ Room 1 (Project A)
    │   ├─→ Client 1
    │   ├─→ Client 2
    │   └─→ Client 3
    ├─→ Room 2 (Project B)
    │   ├─→ Client 4
    │   └─→ Client 5
    └─→ Redis Pub/Sub
        ↓ (for horizontal scaling)
    Other WebSocket Server Instances
```

## Message Types

### User Presence
- `user_joined` - User joined the room
- `user_left` - User left the room
- `user_typing` - User is typing

### Document Events
- `document_update` - Document content changed
- `cursor_update` - Cursor position changed
- `selection` - Text selection changed

### Collaboration (Yjs)
- `yjs_update` - Yjs CRDT update
- `yjs_awareness` - Yjs awareness update

### Compilation
- `compilation_started` - Compilation started
- `compilation_completed` - Compilation completed
- `compilation_failed` - Compilation failed

### System
- `ping` - Heartbeat ping
- `pong` - Heartbeat pong response
- `error` - Error message

## WebSocket Connection

### Endpoint

```
WS /ws/:project_id?token=<jwt_token>
```

### Authentication

Provide JWT token via:
1. Query parameter: `?token=<jwt_token>`
2. Authorization header: `Authorization: Bearer <jwt_token>`

### Example Connection (JavaScript)

```javascript
const projectId = 'abc123';
const token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...';
const ws = new WebSocket(`ws://localhost:8082/ws/${projectId}?token=${token}`);

ws.onopen = () => {
  console.log('Connected to WebSocket');
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message);

  switch (message.type) {
    case 'user_joined':
      console.log(`${message.payload.username} joined`);
      break;
    case 'yjs_update':
      // Handle Yjs update
      break;
    case 'cursor_update':
      // Update cursor position
      break;
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from WebSocket');
};
```

### Sending Messages

```javascript
// Send cursor position
ws.send(JSON.stringify({
  type: 'cursor_update',
  payload: {
    line: 10,
    column: 25
  }
}));

// Send Yjs update
ws.send(JSON.stringify({
  type: 'yjs_update',
  payload: {
    update: new Uint8Array([...]),
    origin: 'user-abc'
  }
}));
```

## Message Format

All messages follow this structure:

```json
{
  "type": "message_type",
  "payload": {},
  "timestamp": "2024-01-15T10:30:00Z",
  "user_id": "user123",
  "username": "johndoe"
}
```

## API Endpoints

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "service": "websocket-service"
}
```

### WebSocket Statistics

```bash
GET /stats
Authorization: Bearer <token>
```

Response:
```json
{
  "total_rooms": 5,
  "total_clients": 15,
  "rooms": [
    {
      "room_id": "project-abc",
      "client_count": 3,
      "created_at": "2024-01-15T10:00:00Z",
      "last_activity": "2024-01-15T10:30:00Z"
    }
  ]
}
```

## Configuration

Environment variables:

```bash
# Service
WEBSOCKET_SERVICE_PORT=8082
ENVIRONMENT=production

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# JWT
JWT_SECRET=your-secret-key
JWT_PUBLIC_KEY_PATH=./keys/jwt-public.pem

# WebSocket Settings
WS_PING_INTERVAL=54s
WS_PONG_WAIT=60s
WS_WRITE_WAIT=10s

# Logging
LOG_LEVEL=info
```

## Running Locally

### With Go

```bash
cd services/websocket
go run cmd/main.go
```

### With Docker

```bash
cd services/websocket
docker build -t texflow-websocket .
docker run -p 8082:8082 \
  -e REDIS_ADDR=redis:6379 \
  -e JWT_SECRET=your-secret \
  texflow-websocket
```

## Horizontal Scaling

The service supports horizontal scaling through Redis Pub/Sub:

1. Multiple WebSocket server instances can run simultaneously
2. Each instance maintains its own connections
3. Messages are broadcast via Redis to all instances
4. Clients connected to different instances can communicate seamlessly

```
Client A → Server 1 → Redis → Server 2 → Client B
```

## Performance

- **Concurrent Connections**: Supports thousands of concurrent connections per instance
- **Message Latency**: Sub-millisecond message routing
- **Memory**: ~1MB per 100 concurrent connections
- **CPU**: Minimal CPU usage with goroutine-based concurrency

## Monitoring

Prometheus metrics exposed at `/metrics`:

- `websocket_active_connections` - Current active connections
- `websocket_total_rooms` - Total active rooms
- `websocket_messages_sent_total` - Total messages sent
- `websocket_messages_received_total` - Total messages received
- `websocket_connection_duration_seconds` - Connection duration histogram

## Security

- **JWT Authentication**: All connections require valid JWT token
- **Rate Limiting**: Configurable max connections per IP
- **Message Size Limit**: 512KB maximum message size
- **Origin Validation**: CORS headers for cross-origin requests
- **Connection Timeout**: Automatic cleanup of stale connections

## Troubleshooting

### Connection Refused

Check if the service is running:
```bash
curl http://localhost:8082/health
```

### Authentication Failed

Verify JWT token is valid:
```bash
# Decode JWT token to check expiry
echo "eyJ..." | base64 -d
```

### Messages Not Broadcasting

Check Redis connection:
```bash
redis-cli ping
```

## Development

### Running Tests

```bash
go test ./...
```

### Code Structure

```
services/websocket/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/              # Configuration
│   ├── handlers/            # HTTP/WebSocket handlers
│   ├── middleware/          # Authentication, CORS
│   ├── models/              # Message models
│   └── websocket/           # WebSocket logic
│       ├── client.go        # Client connection handler
│       └── hub.go           # Hub/room manager
├── pkg/
│   ├── auth/                # JWT validation
│   ├── logger/              # Structured logging
│   └── metrics/             # Prometheus metrics
└── Dockerfile
```

## License

MIT License - see LICENSE file for details
