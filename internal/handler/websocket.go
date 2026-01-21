package handler

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	ws "github.com/prepmyapp/notification/internal/infrastructure/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	hub       *ws.Hub
	jwtSecret string
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(hub *ws.Hub, jwtSecret string) *WebSocketHandler {
	return &WebSocketHandler{
		hub:       hub,
		jwtSecret: jwtSecret,
	}
}

// HandleConnection handles incoming WebSocket connection requests.
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	// Get token from query param
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	// Validate token and extract user ID
	userID, err := h.validateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Create new client
	client := &ws.Client{
		ID:     uuid.New(),
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	// Register client with hub
	h.hub.Register(client)

	// Start goroutines for reading and writing
	go h.writePump(client)
	go h.readPump(client)
}

// validateToken validates a JWT token and returns the user ID.
func (h *WebSocketHandler) validateToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, errors.New("invalid claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, errors.New("missing sub claim")
	}

	return uuid.Parse(sub)
}

// readPump pumps messages from the WebSocket connection to the hub.
func (h *WebSocketHandler) readPump(client *ws.Client) {
	defer func() {
		h.hub.Unregister(client)
		if err := client.Conn.Close(); err != nil {
			log.Printf("failed to close websocket connection: %v", err)
		}
	}()

	client.Conn.SetReadLimit(maxMessageSize)
	if err := client.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("failed to set read deadline: %v", err)
	}
	client.Conn.SetPongHandler(func(string) error {
		return client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// We don't process incoming messages for now
		// This could be extended to handle client-side actions
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (h *WebSocketHandler) writePump(client *ws.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := client.Conn.Close(); err != nil {
			log.Printf("failed to close websocket connection: %v", err)
		}
	}()

	for {
		select {
		case message, ok := <-client.Send:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("failed to set write deadline: %v", err)
				return
			}
			if !ok {
				// Hub closed the channel
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("failed to write close message: %v", err)
				}
				return
			}

			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				log.Printf("failed to write message: %v", err)
			}

			// Drain queued messages
			n := len(client.Send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					log.Printf("failed to write newline: %v", err)
				}
				if _, err := w.Write(<-client.Send); err != nil {
					log.Printf("failed to write queued message: %v", err)
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("failed to set write deadline: %v", err)
				return
			}
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// RegisterRoutes registers WebSocket routes.
func (h *WebSocketHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/ws/notifications", h.HandleConnection)
}
