package realtime

import (
	"donetick.com/core/config"
	cRepo "donetick.com/core/internal/circle/repo"
	uRepo "donetick.com/core/internal/user/repo"
	ginJWT "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// Routes sets up the real-time service routes
func Routes(
	router *gin.Engine,
	rts *RealTimeService,
	authMiddleware *ginJWT.GinJWTMiddleware,
	userRepo *uRepo.UserRepository,
	circleRepo *cRepo.CircleRepository,
	config *config.Config,
) {
	// Create real-time auth middleware
	rtAuthMiddleware := NewAuthMiddleware(userRepo, circleRepo, authMiddleware, config)

	// Create WebSocket handler
	wsHandler := NewWebSocketHandler(rts, rtAuthMiddleware, config)

	// Create SSE polling handler
	pollingHandler := NewPollingHandler(rts, rtAuthMiddleware, config)

	// Real-time API group
	rtGroup := router.Group("/api/v1/realtime")

	// Health check endpoint (no auth required)
	rtGroup.GET("/health", wsHandler.HandleHealthCheck)

	// I never endup releasing the ws as i found sse is more suitable for donetick
	// i can remove this code in future release
	// // WebSocket endpoint (with auth)
	// rtGroup.GET("/ws", rtAuthMiddleware.WebSocketAuthHandler(), wsHandler.HandleWebSocket)
	// SSE endpoint (with auth)
	rtGroup.GET("/sse", rtAuthMiddleware.WebSocketAuthHandler(), pollingHandler.HandleSSE)

	// Connection stats endpoint (with auth)
	rtGroup.GET("/stats/:circleId", authMiddleware.MiddlewareFunc(), rtAuthMiddleware.WebSocketAuthHandler(), wsHandler.HandleConnectionStats)

	// Admin endpoints (require authentication)
	adminGroup := rtGroup.Group("/admin")
	adminGroup.Use(authMiddleware.MiddlewareFunc())
	{
		adminGroup.GET("/stats", wsHandler.HandleHealthCheck)
		adminGroup.GET("/connections/:circleId", wsHandler.HandleConnectionStats)
	}
}
