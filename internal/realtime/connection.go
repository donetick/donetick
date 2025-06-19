package realtime

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	uModel "donetick.com/core/internal/user/model"
)

// Connection represents a WebSocket or SSE connection with metadata
type Connection struct {
	ID           string
	CircleID     int
	UserID       int
	User         *uModel.User
	Conn         *websocket.Conn // nil for SSE connections
	Send         chan *Event
	LastActivity time.Time
	mu           sync.RWMutex
	closed       bool
	logger       *zap.SugaredLogger
}

// NewConnection creates a new WebSocket or SSE connection
func NewConnection(id string, circleID, userID int, user *uModel.User, conn *websocket.Conn, logger *zap.SugaredLogger) *Connection {
	return &Connection{
		ID:           id,
		CircleID:     circleID,
		UserID:       userID,
		User:         user,
		Conn:         conn,
		Send:         make(chan *Event, 256), // Buffered channel for events
		LastActivity: time.Now(),
		closed:       false,
		logger:       logger,
	}
}

// IsStale checks if the connection is stale based on the threshold
func (c *Connection) IsStale(threshold time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.LastActivity) > threshold
}

// IsClosed returns true if the connection is closed
func (c *Connection) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Close closes the WebSocket connection
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.Send)

	// Only close websocket connection if it exists (SSE connections have nil Conn)
	if c.Conn != nil {
		c.Conn.Close()
	}

	c.logger.Debugw("Connection closed",
		"connectionId", c.ID,
		"userId", c.UserID,
		"circleId", c.CircleID,
		"connectionType", c.getConnectionType())
}

// getConnectionType returns the connection type for logging
func (c *Connection) getConnectionType() string {
	if c.Conn == nil {
		return "SSE"
	}
	return "WebSocket"
}

// UpdateActivity updates the last activity timestamp
func (c *Connection) UpdateActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastActivity = time.Now()
}

// SendEvent sends an event to the connection
func (c *Connection) SendEvent(event *Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return false
	}

	select {
	case c.Send <- event:
		return true
	default:
		// Channel is full, log warning and mark connection for closure.
		// The actual channel closing and connection cleanup is handled by Connection.Close().
		c.logger.Warnw("Connection send buffer full. Marking connection for closure.",
			"connectionId", c.ID,
			"userId", c.UserID,
			"circleId", c.CircleID,
			"bufferSize", cap(c.Send),
			"eventTypeAttempted", event.Type)

		c.closed = true // Mark as closed. Connection.Close() will handle actual closing.
		// NOTE: Removed close(c.Send) from here to prevent double-close panics.
		// NOTE: Removed WebSocket specific closing from here; Connection.Close() handles it.
		return false // Indicate that the event was not sent
	}
}

// StartReadPump starts the read pump for handling incoming messages
func (c *Connection) StartReadPump(pool *ConnectionPool) {
	defer func() {
		pool.RemoveConnection(c)
		c.Close()
	}()

	// Set read deadline and pong handler for heartbeat
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.UpdateActivity()
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Errorw("WebSocket read error", "error", err, "connectionId", c.ID)
			}
			break
		}

		c.UpdateActivity()

		// Handle incoming message (e.g., ping/pong, client events)
		c.handleIncomingMessage(message)
	}
}

// StartWritePump starts the write pump for sending outgoing messages
func (c *Connection) StartWritePump() {
	ticker := time.NewTicker(54 * time.Second) // Send ping every 54 seconds
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case event, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send the event as JSON
			eventJSON, err := event.ToJSON()
			if err != nil {
				c.logger.Errorw("Failed to marshal event", "error", err, "connectionId", c.ID)
				continue
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
				c.logger.Errorw("Failed to write message", "error", err, "connectionId", c.ID)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Errorw("Failed to write ping", "error", err, "connectionId", c.ID)
				return
			}
		}
	}
}

// handleIncomingMessage processes incoming WebSocket messages
func (c *Connection) handleIncomingMessage(message []byte) {
	// For now, we mainly handle ping/pong for keepalive
	// In the future, we could handle client-side events here
	c.logger.Debugw("Received message from client",
		"connectionId", c.ID,
		"message", string(message))
}

// WebSocketUpgrader configures the WebSocket upgrader
var WebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// For now, allow all origins - in production, implement proper origin checking
		return true
	},
}
