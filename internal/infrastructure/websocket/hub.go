package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/prepmyapp/notification/internal/domain"
)

// Client represents a connected WebSocket client.
type Client struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Conn   *websocket.Conn
	Send   chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to clients.
type Hub struct {
	// Registered clients grouped by user ID
	clients map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast channel for notifications
	broadcast chan *BroadcastMessage

	// Mutex for thread-safe client map access
	mu sync.RWMutex
}

// BroadcastMessage represents a message to broadcast to specific users.
type BroadcastMessage struct {
	UserID       uuid.UUID
	Notification *domain.Notification
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToUser(message)
		}
	}
}

// registerClient adds a client to the hub.
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	log.Printf("Client %s registered for user %s (total connections: %d)",
		client.ID, client.UserID, len(h.clients[client.UserID]))
}

// unregisterClient removes a client from the hub.
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.UserID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.Send)

			// Clean up empty user entries
			if len(clients) == 0 {
				delete(h.clients, client.UserID)
			}

			log.Printf("Client %s unregistered for user %s", client.ID, client.UserID)
		}
	}
}

// broadcastToUser sends a notification to all of a user's connected clients.
func (h *Hub) broadcastToUser(message *BroadcastMessage) {
	h.mu.RLock()
	clients, ok := h.clients[message.UserID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	data, err := json.Marshal(message.Notification)
	if err != nil {
		log.Printf("Failed to marshal notification: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			// Client buffer full, close connection
			close(client.Send)
			delete(clients, client)
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Notify sends a notification to a user via WebSocket.
// Implements the service.InAppNotifier interface.
func (h *Hub) Notify(ctx context.Context, userID uuid.UUID, notification *domain.Notification) error {
	h.broadcast <- &BroadcastMessage{
		UserID:       userID,
		Notification: notification,
	}
	return nil
}

// GetConnectedUsers returns the number of unique users with active connections.
func (h *Hub) GetConnectedUsers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetTotalConnections returns the total number of active connections.
func (h *Hub) GetTotalConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, clients := range h.clients {
		total += len(clients)
	}
	return total
}

// IsUserConnected checks if a user has any active connections.
func (h *Hub) IsUserConnected(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[userID]
	return ok && len(clients) > 0
}
