package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	realTimeService *RealTimeService
	authMiddleware  *AuthMiddleware
	config          *config.Config
	logger          *zap.SugaredLogger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	rts *RealTimeService,
	authMiddleware *AuthMiddleware,
	config *config.Config,
) *WebSocketHandler {
	return &WebSocketHandler{
		realTimeService: rts,
		authMiddleware:  authMiddleware,
		config:          config,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection management
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	h.logger = logging.FromContext(c.Request.Context())

	// Check if WebSocket is enabled
	if !h.config.RealTimeConfig.Enabled || !h.config.RealTimeConfig.WebSocketEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "WebSocket service is not available",
			"code":  "SERVICE_UNAVAILABLE",
		})
		return
	}

	// Get authenticated user and circle ID from middleware
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	circleIDInterface, exists := c.Get("circleId")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Circle ID required",
			"code":  "CIRCLE_ID_REQUIRED",
		})
		return
	}

	user := userInterface.(*uModel.User)
	circleID := circleIDInterface.(int)

	// Upgrade HTTP connection to WebSocket
	conn, err := WebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorw("Failed to upgrade WebSocket connection",
			"error", err,
			"userId", user.ID,
			"circleId", circleID)
		return
	}

	// Generate unique connection ID
	connectionID := h.generateConnectionID()

	// Create connection object
	wsConn := NewConnection(connectionID, circleID, user.ID, user, conn, h.logger)

	// Add connection to the real-time service
	if err := h.realTimeService.AddConnection(wsConn); err != nil {
		h.logger.Errorw("Failed to add connection to service",
			"error", err,
			"connectionId", connectionID,
			"userId", user.ID,
			"circleId", circleID)

		conn.WriteJSON(NewErrorEvent(circleID, "CONNECTION_FAILED", err.Error()))
		conn.Close()
		return
	}

	h.logger.Infow("WebSocket connection established",
		"connectionId", connectionID,
		"userId", user.ID,
		"username", user.Username,
		"circleId", circleID)

	// Send connection established event
	establishedEvent := NewConnectionEstablishedEvent(connectionID, circleID, user.ID)
	wsConn.SendEvent(establishedEvent)

	// Get connection pool for this circle
	pool := h.realTimeService.GetConnectionPool(circleID)

	// Start read and write pumps in separate goroutines
	go wsConn.StartWritePump()
	go wsConn.StartReadPump(pool) // This will block until connection closes
}

// generateConnectionID generates a unique connection identifier
func (h *WebSocketHandler) generateConnectionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// HandleHealthCheck provides health check endpoint for the real-time service
func (h *WebSocketHandler) HandleHealthCheck(c *gin.Context) {
	stats := h.realTimeService.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": gin.H{
			"enabled":   h.config.RealTimeConfig.Enabled,
			"websocket": h.config.RealTimeConfig.WebSocketEnabled,
		},
		"stats": gin.H{
			"activeConnections": stats.ActiveConnections,
			"eventsPublished":   stats.EventsPublished,
			"eventsDelivered":   stats.EventsDelivered,
			"circlesActive":     stats.CirclesActive,
		},
	})
}

// HandleConnectionStats provides connection statistics for a specific circle
func (h *WebSocketHandler) HandleConnectionStats(c *gin.Context) {
	circleIDStr := c.Param("circleId")
	if circleIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Circle ID required",
			"code":  "CIRCLE_ID_REQUIRED",
		})
		return
	}

	// Parse circle ID
	var circleID int
	if _, err := fmt.Sscanf(circleIDStr, "%d", &circleID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid circle ID",
			"code":  "INVALID_CIRCLE_ID",
		})
		return
	}

	// Get authenticated user from middleware
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	user := userInterface.(*uModel.User)

	// Verify user has access to this circle
	if user.CircleID != circleID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied to circle",
			"code":  "ACCESS_DENIED",
		})
		return
	}

	// Get connection pool stats
	pool := h.realTimeService.GetConnectionPool(circleID)
	poolStats := pool.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"circleId": circleID,
		"connections": gin.H{
			"active":        poolStats.ActiveConnections,
			"totalMessages": poolStats.TotalMessages,
			"uniqueUsers":   pool.GetUserCount(),
		},
	})
}
