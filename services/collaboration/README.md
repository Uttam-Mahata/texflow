# Collaboration Service

The Collaboration Service manages Yjs CRDT (Conflict-free Replicated Data Type) updates for real-time collaborative editing in the TexFlow LaTeX editor. It handles storing, retrieving, and synchronizing document states across multiple clients.

## Features

- **Yjs CRDT Storage**: Persistent storage of Yjs updates in MongoDB
- **Document Synchronization**: Efficient state synchronization for new clients
- **Snapshot Management**: Automatic snapshot creation every N updates
- **Version Control**: Monotonically increasing version numbers for updates
- **State Vector Support**: Quick synchronization using Yjs state vectors
- **Metrics & Analytics**: Track document collaboration metrics
- **Automatic Cleanup**: TTL-based cleanup of old updates
- **High Performance**: Thread-safe version counters and efficient queries

## Architecture

```
Client A ──┐
           ├──> WebSocket Service ──> Collaboration Service ──> MongoDB
Client B ──┘                                                      │
                                                                  ├─ yjs_updates
                                                                  └─ yjs_snapshots
```

## Yjs CRDT Workflow

1. **Client Makes Edit**: User types in the editor
2. **Yjs Generates Update**: Yjs library creates a binary update
3. **Send to Server**: Update sent via WebSocket
4. **Store Update**: Collaboration Service stores update with version number
5. **Broadcast**: WebSocket Service broadcasts to other clients
6. **Apply Update**: Other clients apply the update to their local Yjs document

## Data Model

### YjsUpdate

Represents an individual CRDT update:

```json
{
  "id": "ObjectId",
  "project_id": "ObjectId",
  "document_name": "main.tex",
  "update": "base64_encoded_binary_data",
  "clock": 1705320000000000,
  "version": 42,
  "user_id": "ObjectId",
  "client_id": "client_abc123",
  "size_bytes": 1024,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### YjsSnapshot

Represents a document snapshot at a specific version:

```json
{
  "id": "ObjectId",
  "project_id": "ObjectId",
  "document_name": "main.tex",
  "state_vector": "binary_state_vector",
  "snapshot": "binary_snapshot_data",
  "version": 100,
  "update_count": 100,
  "size_bytes": 102400,
  "created_at": "2024-01-15T10:30:00Z"
}
```

## API Endpoints

### Store Update

```bash
POST /api/v1/collaboration/updates
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": "abc123",
  "document_name": "main.tex",
  "update": "base64_encoded_yjs_update",
  "client_id": "client_xyz"
}
```

Response:
```json
{
  "id": "...",
  "version": 42,
  "update_base64": "...",
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Get Document State

Get the current state of a document (snapshot + missing updates):

```bash
GET /api/v1/collaboration/state/:project_id/:document_name?since_version=0
Authorization: Bearer <token>
```

Response:
```json
{
  "state_vector": "base64_encoded",
  "snapshot": "base64_encoded",
  "updates": [
    {
      "version": 101,
      "update_base64": "...",
      "created_at": "..."
    }
  ],
  "version": 150,
  "update_count": 150
}
```

**Synchronization Strategy**:

- If `since_version = 0`: Client wants full state
  - Return: Latest snapshot + all updates after snapshot

- If `since_version < snapshot_version`: Client is too far behind
  - Return: Snapshot + updates after snapshot

- If `since_version >= snapshot_version`: Client is recent
  - Return: Only missing updates

### Get Updates

Get updates since a specific version:

```bash
GET /api/v1/collaboration/updates/:project_id/:document_name?since_version=42&limit=100
Authorization: Bearer <token>
```

Response:
```json
[
  {
    "id": "...",
    "version": 43,
    "update_base64": "...",
    "user_id": "...",
    "created_at": "..."
  },
  {
    "id": "...",
    "version": 44,
    "update_base64": "...",
    "created_at": "..."
  }
]
```

### Get Document Metrics

```bash
GET /api/v1/collaboration/metrics/:project_id/:document_name
Authorization: Bearer <token>
```

Response:
```json
{
  "project_id": "...",
  "document_name": "main.tex",
  "total_updates": 523,
  "total_snapshots": 5,
  "current_version": 523,
  "last_updated": "2024-01-15T10:30:00Z",
  "contributors": ["user_id_1", "user_id_2"],
  "size_bytes": 524288
}
```

## Configuration

Environment variables:

```bash
# Service
COLLABORATION_SERVICE_PORT=8083
ENVIRONMENT=production

# MongoDB
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=texflow

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# JWT
JWT_SECRET=your-secret-key
JWT_PUBLIC_KEY_PATH=./keys/jwt-public.pem

# Collaboration Settings
SNAPSHOT_INTERVAL=100              # Create snapshot every N updates
MAX_UPDATES_PER_FETCH=1000        # Maximum updates per request
UPDATE_RETENTION_DAYS=30          # Days to keep updates
MAX_DOCUMENT_SIZE_BYTES=10485760  # 10MB max document size

# Logging
LOG_LEVEL=info
```

## Snapshot Strategy

Snapshots are created automatically to optimize synchronization:

1. **Trigger**: Every `SNAPSHOT_INTERVAL` updates (default: 100)
2. **Process**:
   - Merge all updates into a single Yjs document
   - Encode document state as binary
   - Generate state vector
   - Store snapshot in MongoDB
3. **Benefit**: New clients can sync from snapshot instead of replaying all updates
4. **Storage**: Old snapshots are retained for rollback capability

## Database Indexes

### yjs_updates Collection

```javascript
// Unique index for version uniqueness
{ project_id: 1, document_name: 1, version: 1 } (unique)

// Query index for fetching updates
{ project_id: 1, document_name: 1, created_at: -1 }

// TTL index for automatic cleanup (30 days)
{ created_at: 1 } (TTL: 2592000 seconds)
```

### yjs_snapshots Collection

```javascript
// Query index for latest snapshot
{ project_id: 1, document_name: 1, version: -1 }

// Cleanup index
{ created_at: -1 }
```

## Version Management

Version numbers are:
- **Monotonically increasing**: Each update gets next sequential number
- **Thread-safe**: In-memory counters with mutex protection
- **Persistent**: Initialized from database on startup
- **Per-document**: Each document has independent version counter

## Performance Optimizations

1. **Version Counters**: In-memory counters to avoid DB queries for each update
2. **Batch Fetching**: Retrieve multiple updates in single query
3. **Indexes**: Optimized indexes for common queries
4. **TTL Cleanup**: Automatic removal of old updates
5. **Snapshot Compression**: Periodic snapshots reduce sync time
6. **Connection Pooling**: MongoDB connection pool (10-100 connections)

## Integration with WebSocket Service

The Collaboration Service works with the WebSocket Service:

```javascript
// Client side - Yjs integration
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';

const ydoc = new Y.Doc();
const wsProvider = new WebsocketProvider(
  'ws://localhost:8082/ws/project123?token=...',
  'main.tex',
  ydoc
);

// WebSocket receives Yjs updates
wsProvider.on('sync', (isSynced) => {
  console.log('Synced:', isSynced);
});

// Collaboration Service stores updates for persistence
```

## Cleanup & Maintenance

### Automatic Cleanup

Runs daily to remove old data:
- Updates older than `UPDATE_RETENTION_DAYS` are deleted
- Snapshots are retained longer for rollback capability
- MongoDB TTL index provides backup cleanup

### Manual Cleanup

```bash
# Trigger cleanup via API (admin endpoint)
POST /api/v1/admin/cleanup
Authorization: Bearer <admin_token>
```

## Running Locally

### With Go

```bash
cd services/collaboration
go run cmd/main.go
```

### With Docker

```bash
cd services/collaboration
docker build -t texflow-collaboration .
docker run -p 8083:8083 \
  -e MONGO_URI=mongodb://mongo:27017 \
  -e REDIS_ADDR=redis:6379 \
  -e JWT_SECRET=your-secret \
  texflow-collaboration
```

## Monitoring

Prometheus metrics at `/metrics`:

- `collaboration_updates_total` - Total updates stored
- `collaboration_snapshots_total` - Total snapshots created
- `collaboration_documents_active` - Active documents
- `collaboration_storage_bytes` - Storage used
- `collaboration_sync_duration_seconds` - State sync duration

## Error Handling

Common errors and solutions:

### "Invalid update encoding"
- Update must be base64 encoded
- Ensure binary Yjs update is properly encoded

### "Version conflict"
- Concurrent updates may cause version conflicts
- Service handles this automatically with retries

### "Document too large"
- Document exceeds `MAX_DOCUMENT_SIZE_BYTES`
- Consider splitting into multiple files

## Testing

### Unit Tests

```bash
go test ./...
```

### Integration Test

```javascript
// Store an update
const update = Y.encodeStateAsUpdate(ydoc);
const base64Update = btoa(String.fromCharCode(...update));

const response = await fetch('/api/v1/collaboration/updates', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    project_id: 'abc123',
    document_name: 'main.tex',
    update: base64Update,
    client_id: 'client_xyz'
  })
});

const result = await response.json();
console.log('Stored version:', result.version);
```

## License

MIT License - see LICENSE file for details
