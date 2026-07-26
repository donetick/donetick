package realtime

import (
	ginJWT "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// Routes sets up the real-time service routes. The auth middleware and polling
// handler are provided by fx (see main.go) so their lifecycle and shared
// dependencies (e.g. the ticket store) are managed in one place.
func Routes(
	router *gin.Engine,
	rtAuthMiddleware *AuthMiddleware,
	pollingHandler *PollingHandler,
	jwtAuth *ginJWT.GinJWTMiddleware,
) {
	// Real-time API group
	rtGroup := router.Group("/api/v1/realtime")

	// SSE endpoint (with auth)
	rtGroup.GET("/sse", rtAuthMiddleware.WebSocketAuthHandler(), pollingHandler.HandleSSE)

	// SSE ticket endpoint (header-authenticated). Native EventSource clients that
	// cannot send custom headers call this first, then open /sse?ticket=...
	rtGroup.GET("/sse/ticket", jwtAuth.MiddlewareFunc(), pollingHandler.HandleSSETicket)

	// Admin endpoints (require authentication)
	adminGroup := rtGroup.Group("/admin")
	adminGroup.Use(jwtAuth.MiddlewareFunc())
	{
		// adminGroup.GET("/stats", wsHandler.HandleHealthCheck)
		// adminGroup.GET("/connections/:circleId", wsHandler.HandleConnectionStats)
	}
}
