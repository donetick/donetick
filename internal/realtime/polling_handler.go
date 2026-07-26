package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PollingHandler handles Server-Sent Events (SSE) for real-time updates
type PollingHandler struct {
	realTimeService *RealTimeService
	authMiddleware  *AuthMiddleware
	config          *config.Config
	logger          *zap.SugaredLogger
	// Rate limiting for SSE connections
	AllowedOrigins map[string]bool
	sseConnections map[string]time.Time // userID:IP -> last SSE connection time
	sseTickets     map[string]time.Time // userID:IP -> last ticket mint time
	sseMutex       sync.RWMutex
}

// NewPollingHandler creates a new polling handler for SSE
func NewPollingHandler(
	rts *RealTimeService,
	authMiddleware *AuthMiddleware,
	config *config.Config,
) *PollingHandler {
	h := &PollingHandler{
		realTimeService: rts,
		authMiddleware:  authMiddleware,
		config:          config,
		logger:          logging.DefaultLogger(),
		sseConnections:  make(map[string]time.Time),
		sseTickets:      make(map[string]time.Time),
		AllowedOrigins:  make(map[string]bool),
	}

	for _, origin := range config.Server.CorsAllowOrigins {
		h.AllowedOrigins[origin] = true
	}

	// Start periodic cleanup of stale connections
	go func() {
		ticker := time.NewTicker(config.RealTimeConfig.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			h.cleanupStaleConnections()
		}
	}()

	return h
}

// HandleSSETicket issues a short-lived, single-use ticket that native
// EventSource clients exchange for an authenticated SSE connection. This
// endpoint is protected by the standard JWT middleware, so the token is sent in
// the Authorization header and never exposed in a URL.
func (h *PollingHandler) HandleSSETicket(c *gin.Context) {
	logger := logging.FromContext(c.Request.Context())

	user, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	if !user.IsPlusMember() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied for non-plus members",
			"code":  "ACCESS_DENIED",
		})
		return
	}

	// Throttle ticket minting per user+IP using a dedicated counter that is
	// separate from the SSE-connection counter. This ensures that minting a
	// ticket doesn't block the immediate follow-up /sse request, while still
	// preventing a client from flooding /sse/ticket and exhausting TicketStore.
	if !h.checkTicketRateLimit(user.ID, c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Rate limit exceeded",
			"code":  "RATE_LIMIT_EXCEEDED",
		})
		return
	}

	ticket, err := h.authMiddleware.ticketStore.Issue(user.ID)
	if err != nil {
		logger.Errorw("Failed to issue SSE ticket", "userID", user.ID, "error", err)
		if errors.Is(err, ErrTicketStoreFull) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Real-time service is temporarily busy, please retry",
				"code":  "TICKET_STORE_FULL",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to issue ticket",
			"code":  "TICKET_ISSUE_FAILED",
		})
		return
	}

	h.recordTicketMint(user.ID, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"ticket":    ticket,
		"expiresIn": int(ticketTTL.Seconds()),
	})
}

// HandleSSE handles Server-Sent Events connections
func (h *PollingHandler) HandleSSE(c *gin.Context) {
	// Check if SSE is enabled
	if !h.config.RealTimeConfig.Enabled || !h.config.RealTimeConfig.SSEEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server-Sent Events service is not available",
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

	user := userInterface.(*uModel.User)
	circleID := user.CircleID

	// Validate the request parameters
	if err := h.validateSSERequest(c, user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Reconnect throttle: reject if the user reconnected too recently.
	if !h.checkRateLimit(user.ID, c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Rate limit exceeded",
			"code":  "RATE_LIMIT_EXCEEDED",
		})
		return
	}

	// Generate a unique connection ID for tracking (early for potential logging)
	connectionID := h.generateConnectionID()

	// Set SSE headers first
	h.setSSEHeaders(c)

	// Write HTTP 200 OK status explicitly
	c.Status(http.StatusOK)

	// IMPORTANT! This disable write timeout for this specific SSE connection seems requires Go 1.20+ which i have anyway
	// THIS WAS VERY IMPORTANT I SPEND HOURS DEBUGGING SSE not working and it was due to
	// the write deadline being set by default in gin's ResponseWriter.
	// I want to keep it for other handlers, but for SSE we need to disable it.
	if rc := http.NewResponseController(c.Writer); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			h.logger.Errorw("Failed to disable write deadline for SSE connection",
				"error", err,
				"connectionId", connectionID,
			)
			// Depending on policy, this might be a fatal error for the connection.
			// For now, logging and continuing.
		}
	} else {
		// This case should ideally not happen with a standard http.ResponseWriter.
		h.logger.Warnw("Could not get ResponseController, unable to disable write deadline for SSE. This might lead to premature connection drops if a server write timeout is configured.",
			"connectionId", connectionID,
		)
	}

	// Create SSE connection object - we use nil for websocket conn as this is SSE
	sseConn := NewConnection(connectionID, circleID, user.ID, user, nil, h.logger)

	// Add connection to the real-time service pool so it can receive broadcast events
	if err := h.realTimeService.AddConnection(sseConn); err != nil {
		h.logger.Errorw("Failed to add SSE connection to service",
			"error", err,
			"connectionId", connectionID,
			"userId", user.ID,
			"circleId", circleID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to establish connection",
			"code":  "CONNECTION_FAILED",
		})
		return
	}

	// Record connection time now so that even a very short-lived connection
	// counts against the reconnect throttle.
	h.updateConnectionTime(user.ID, c.ClientIP())

	h.logger.Infow("SSE connection established",
		"connectionId", connectionID,
		"userId", user.ID,
		"username", "[REDACTED]", // Don't log username for security
		"circleId", circleID)

	// Send initial connection event
	initialEvent := NewConnectionEstablishedEvent(connectionID, circleID, user.ID)
	if !h.sendSSEEvent(c, initialEvent) {
		h.logger.Errorw("Failed to send initial SSE event", "connectionId", connectionID)
		h.realTimeService.RemoveConnection(sseConn)
		return
	}

	h.logger.Infow("SSE initial event sent successfully", "connectionId", connectionID)

	// Create context for managing the connection lifecycle
	// Important: Create a background context to avoid HTTP request timeout issues
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor the original request context for client disconnects
	go func() {
		<-c.Request.Context().Done()
		h.logger.Infow("Client request context cancelled", "connectionId", connectionID)
		cancel()
	}()

	// Use a done channel to properly handle cleanup
	done := make(chan struct{})
	defer func() {
		close(done)
		h.logger.Infow("SSE connection cleanup started", "connectionId", connectionID)
		// Remove connection from pool
		h.realTimeService.RemoveConnection(sseConn)
		sseConn.Close()
		// Update the last connection time for the user
		h.updateConnectionTime(user.ID, c.ClientIP())
		h.logger.Infow("SSE connection cleanup completed", "connectionId", connectionID)
	}()

	// Start the main SSE event loop
	h.logger.Infow("Starting SSE event loop", "connectionId", connectionID)
	h.sseEventLoop(ctx, c, sseConn, circleID)
}

// setSSEHeaders sets the necessary headers for Server-Sent Events
func (h *PollingHandler) setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// CORS headers for cross-origin requests from frontend
	origin := c.GetHeader("Origin")
	if h.AllowedOrigins[origin] {
		c.Header("Access-Control-Allow-Origin", origin)
	}

	c.Header("Access-Control-Allow-Headers", "Authorization, Cache-Control, Content-Type, Accept, Sec-Ch-Ua, Sec-Ch-Ua-Mobile, Sec-Ch-Ua-Platform")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Expose-Headers", "Content-Type")

	// Ensure response is flushed immediately
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// sendSSEEvent sends an event over SSE
func (h *PollingHandler) sendSSEEvent(c *gin.Context, event *Event) bool {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.logger.Errorw("Failed to marshal SSE event", "error", err)
		return false
	}

	// Write SSE format
	_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
	if err != nil {
		h.logger.Debugw("Failed to write SSE event (client likely disconnected)", "error", err)
		return false
	}

	// Flush the data to the client
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	} else {
		h.logger.Warnw("Response writer does not support flushing")
		return false
	}

	return true
}

// heartbeatLoop sends periodic heartbeat events
func (h *PollingHandler) heartbeatLoop(ctx context.Context, c *gin.Context, circleID int) {
	ticker := time.NewTicker(h.config.RealTimeConfig.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeatEvent := NewHeartbeatEvent(circleID)
			if !h.sendSSEEvent(c, heartbeatEvent) {
				return
			}
		}
	}
}

// sseEventLoop manages the main event loop for SSE connections
func (h *PollingHandler) sseEventLoop(ctx context.Context, c *gin.Context, conn *Connection, circleID int) {
	ticker := time.NewTicker(h.config.RealTimeConfig.HeartbeatInterval)
	defer ticker.Stop()

	h.logger.Debugw("SSE event loop started", "connectionId", conn.ID, "heartbeatInterval", h.config.RealTimeConfig.HeartbeatInterval)

	for {
		select {
		case <-ctx.Done():
			h.logger.Debugw("SSE context cancelled, closing event loop.", "connectionId", conn.ID, "error", ctx.Err())
			return

		case event, ok := <-conn.Send:
			if !ok {
				// Channel closed, connection is being shut down
				h.logger.Debugw("SSE send channel closed, closing event loop.", "connectionId", conn.ID)
				return
			}

			h.logger.Debugw("Sending SSE event", "connectionId", conn.ID, "eventType", event.Type)

			if !h.sendSSEEvent(c, event) {
				h.logger.Warnw("Failed to send SSE event, client likely disconnected. Closing event loop.",
					"connectionId", conn.ID, "eventType", event.Type)
				return // Exit loop if sending fails
			}

			conn.UpdateActivity()
			h.logger.Debugw("SSE event sent successfully", "connectionId", conn.ID, "eventType", event.Type)

		case <-ticker.C:
			if conn.IsClosed() { // Check if connection was marked as closed (e.g. buffer full)
				h.logger.Infow("SSE connection marked as closed, stopping heartbeat and event loop.", "connectionId", conn.ID)
				return
			}
			h.logger.Debugw("Sending SSE heartbeat", "connectionId", conn.ID)

			heartbeatEvent := NewHeartbeatEvent(circleID)
			if !h.sendSSEEvent(c, heartbeatEvent) {
				h.logger.Warnw("Failed to send SSE heartbeat, client likely disconnected. Closing event loop.",
					"connectionId", conn.ID)
				return // Exit loop if sending fails
			}

			conn.UpdateActivity()
			h.logger.Debugw("SSE heartbeat sent successfully", "connectionId", conn.ID)
		}
	}
}

// generateConnectionID generates a unique connection identifier for SSE
func (h *PollingHandler) generateConnectionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("sse_%s", hex.EncodeToString(bytes))
}

// validateSSERequest validates the SSE request parameters
func (h *PollingHandler) validateSSERequest(c *gin.Context, user *uModel.User) error {
	// Validate user ID
	if user.ID <= 0 {
		h.logger.Warnw("Invalid user ID in SSE request", "userId", user.ID, "clientIP", c.ClientIP())
		return fmt.Errorf("invalid user ID: %d", user.ID)
	}

	// Validate circle ID
	if user.CircleID <= 0 {
		h.logger.Warnw("Invalid circle ID in SSE request", "circleId", user.CircleID, "userId", user.ID)
		return fmt.Errorf("invalid circle ID: %d", user.CircleID)
	}

	// Check if user is disabled
	if user.Disabled || user.Expiration.Before(time.Now().UTC()) {
		h.logger.Warnw("Disabled user attempted SSE connection", "userId", user.ID, "clientIP", c.ClientIP())
		return fmt.Errorf("user account is disabled")
	}

	return nil
}

// checkRateLimit checks if the user has exceeded the rate limit for SSE connections.
func (h *PollingHandler) checkRateLimit(userID int, clientIP string) bool {
	h.sseMutex.RLock()
	key := fmt.Sprintf("%d:%s", userID, clientIP)
	lastConnTime, exists := h.sseConnections[key]
	h.sseMutex.RUnlock()

	if exists {
		if time.Since(lastConnTime) < h.config.RealTimeConfig.MinConnectionInterval {
			return false
		}
	}

	return true
}

// checkTicketRateLimit checks if the user has exceeded the rate limit for ticket minting.
// It uses a separate counter from the SSE-connection rate limit so that minting
// a ticket does not prevent the subsequent /sse connection.
func (h *PollingHandler) checkTicketRateLimit(userID int, clientIP string) bool {
	h.sseMutex.RLock()
	key := fmt.Sprintf("%d:%s", userID, clientIP)
	lastMintTime, exists := h.sseTickets[key]
	h.sseMutex.RUnlock()

	if exists {
		if time.Since(lastMintTime) < h.config.RealTimeConfig.MinConnectionInterval {
			return false
		}
	}

	return true
}

// recordTicketMint records the current time as the last ticket-mint time for rate limiting.
func (h *PollingHandler) recordTicketMint(userID int, clientIP string) {
	h.sseMutex.Lock()
	key := fmt.Sprintf("%d:%s", userID, clientIP)
	h.sseTickets[key] = time.Now().UTC()
	h.sseMutex.Unlock()
}

// updateConnectionTime updates the last connection time for rate limiting.
func (h *PollingHandler) updateConnectionTime(userID int, clientIP string) {
	h.sseMutex.Lock()
	key := fmt.Sprintf("%d:%s", userID, clientIP)
	h.sseConnections[key] = time.Now().UTC()
	h.sseMutex.Unlock()
}

// cleanupStaleConnections removes stale entries from both rate-limiting maps.
func (h *PollingHandler) cleanupStaleConnections() {
	h.sseMutex.Lock()
	defer h.sseMutex.Unlock()

	cutoff := time.Now().UTC().Add(-h.config.RealTimeConfig.StaleThreshold)
	for key, lastTime := range h.sseConnections {
		if lastTime.Before(cutoff) {
			delete(h.sseConnections, key)
		}
	}
	for key, lastTime := range h.sseTickets {
		if lastTime.Before(cutoff) {
			delete(h.sseTickets, key)
		}
	}
}
