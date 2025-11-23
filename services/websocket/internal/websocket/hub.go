package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/texflow/services/websocket/internal/models"
	"go.uber.org/zap"
)

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	// Registered clients per room
	rooms map[string]*Room

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to all clients in a room
	broadcast chan *BroadcastMessage

	// Redis client for pub/sub
	redisClient *redis.Client

	// Logger
	logger *zap.Logger

	// Mutex for thread-safe access to rooms
	mu sync.RWMutex
}

// Room represents a collaboration room (project)
type Room struct {
	ID      string
	clients map[*Client]bool
	mu      sync.RWMutex

	// Room metadata
	createdAt    time.Time
	lastActivity time.Time
}

// BroadcastMessage represents a message to be broadcast to a room
type BroadcastMessage struct {
	roomID  string
	message *models.Message
	exclude *Client // Optional: exclude this client from receiving the message
}

// NewHub creates a new Hub instance
func NewHub(redisClient *redis.Client, logger *zap.Logger) *Hub {
	return &Hub{
		rooms:       make(map[string]*Room),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan *BroadcastMessage, 256),
		redisClient: redisClient,
		logger:      logger,
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case broadcastMsg := <-h.broadcast:
			h.broadcastToRoom(broadcastMsg)
		}
	}
}

// RegisterClient registers a client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// registerClient adds a client to a room
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.rooms[client.projectID]
	if !exists {
		room = &Room{
			ID:           client.projectID,
			clients:      make(map[*Client]bool),
			createdAt:    time.Now(),
			lastActivity: time.Now(),
		}
		h.rooms[client.projectID] = room

		// Subscribe to Redis channel for this room
		go h.subscribeToRedis(room)

		h.logger.Info("Created new room",
			zap.String("room_id", room.ID),
		)
	}

	room.mu.Lock()
	room.clients[client] = true
	room.lastActivity = time.Now()
	clientCount := len(room.clients)
	room.mu.Unlock()

	h.logger.Info("Client joined room",
		zap.String("user_id", client.userID),
		zap.String("username", client.username),
		zap.String("room_id", client.projectID),
		zap.Int("total_clients", clientCount),
	)

	// Notify all clients in the room about the new user
	presence := models.UserPresence{
		UserID:   client.userID,
		Username: client.username,
		Color:    client.color,
		JoinedAt: time.Now(),
	}

	joinMsg, err := models.NewMessage(
		models.MessageTypeUserJoined,
		presence,
		client.userID,
		client.username,
	)
	if err != nil {
		h.logger.Error("Failed to create join message", zap.Error(err))
		return
	}

	h.broadcast <- &BroadcastMessage{
		roomID:  client.projectID,
		message: joinMsg,
		exclude: nil, // Notify everyone including the new user
	}

	// Send current room members to the new client
	h.sendRoomMembers(client, room)
}

// unregisterClient removes a client from a room
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.rooms[client.projectID]
	if !exists {
		return
	}

	room.mu.Lock()
	if _, ok := room.clients[client]; ok {
		delete(room.clients, client)
		close(client.send)
		room.lastActivity = time.Now()
		clientCount := len(room.clients)
		room.mu.Unlock()

		h.logger.Info("Client left room",
			zap.String("user_id", client.userID),
			zap.String("username", client.username),
			zap.String("room_id", client.projectID),
			zap.Int("remaining_clients", clientCount),
		)

		// Notify other clients about the user leaving
		leaveMsg, err := models.NewMessage(
			models.MessageTypeUserLeft,
			models.UserPresence{
				UserID:   client.userID,
				Username: client.username,
			},
			client.userID,
			client.username,
		)
		if err == nil {
			h.broadcast <- &BroadcastMessage{
				roomID:  client.projectID,
				message: leaveMsg,
				exclude: client,
			}
		}

		// Clean up empty room
		if clientCount == 0 {
			delete(h.rooms, client.projectID)
			h.logger.Info("Removed empty room", zap.String("room_id", client.projectID))
		}
	} else {
		room.mu.Unlock()
	}
}

// broadcastToRoom sends a message to all clients in a room
func (h *Hub) broadcastToRoom(broadcastMsg *BroadcastMessage) {
	h.mu.RLock()
	room, exists := h.rooms[broadcastMsg.roomID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	// Marshal message once
	messageData, err := json.Marshal(broadcastMsg.message)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message", zap.Error(err))
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	// Send to all clients in the room
	for client := range room.clients {
		// Skip excluded client
		if broadcastMsg.exclude != nil && client == broadcastMsg.exclude {
			continue
		}

		select {
		case client.send <- messageData:
		default:
			// Client's send buffer is full, close the connection
			close(client.send)
			delete(room.clients, client)
		}
	}

	// Also publish to Redis for other server instances
	h.publishToRedis(broadcastMsg.roomID, messageData)
}

// subscribeToRedis subscribes to Redis pub/sub for a room
func (h *Hub) subscribeToRedis(room *Room) {
	ctx := context.Background()
	channel := fmt.Sprintf("room:%s", room.ID)

	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	h.logger.Info("Subscribed to Redis channel", zap.String("channel", channel))

	ch := pubsub.Channel()
	for msg := range ch {
		// Parse the message
		var wsMsg models.Message
		if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
			h.logger.Error("Failed to unmarshal Redis message", zap.Error(err))
			continue
		}

		// Broadcast to local clients
		messageData := []byte(msg.Payload)

		room.mu.RLock()
		for client := range room.clients {
			select {
			case client.send <- messageData:
			default:
				close(client.send)
				delete(room.clients, client)
			}
		}
		room.mu.RUnlock()
	}
}

// publishToRedis publishes a message to Redis
func (h *Hub) publishToRedis(roomID string, message []byte) {
	ctx := context.Background()
	channel := fmt.Sprintf("room:%s", roomID)

	if err := h.redisClient.Publish(ctx, channel, message).Err(); err != nil {
		h.logger.Error("Failed to publish to Redis",
			zap.String("channel", channel),
			zap.Error(err),
		)
	}
}

// sendRoomMembers sends the list of current room members to a client
func (h *Hub) sendRoomMembers(client *Client, room *Room) {
	room.mu.RLock()
	defer room.mu.RUnlock()

	members := make([]models.UserPresence, 0, len(room.clients))
	for c := range room.clients {
		if c != client {
			members = append(members, models.UserPresence{
				UserID:   c.userID,
				Username: c.username,
				Color:    c.color,
				JoinedAt: room.createdAt,
			})
		}
	}

	if len(members) > 0 {
		// Send list of existing members
		for _, member := range members {
			msg, err := models.NewMessage(
				models.MessageTypeUserJoined,
				member,
				member.UserID,
				member.Username,
			)
			if err != nil {
				continue
			}
			client.sendMessage(msg)
		}
	}
}

// GetRoomStats returns statistics about active rooms
func (h *Hub) GetRoomStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	roomStats := make([]map[string]interface{}, 0, len(h.rooms))

	for roomID, room := range h.rooms {
		room.mu.RLock()
		clientCount := len(room.clients)
		room.mu.RUnlock()

		totalClients += clientCount

		roomStats = append(roomStats, map[string]interface{}{
			"room_id":       roomID,
			"client_count":  clientCount,
			"created_at":    room.createdAt,
			"last_activity": room.lastActivity,
		})
	}

	return map[string]interface{}{
		"total_rooms":   len(h.rooms),
		"total_clients": totalClients,
		"rooms":         roomStats,
	}
}
