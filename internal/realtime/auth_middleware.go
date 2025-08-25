package realtime

import (
	"context"
	"net/http"
	"strings"

	"donetick.com/core/config"
	cRepo "donetick.com/core/internal/circle/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	ginJWT "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

// AuthMiddleware handles WebSocket authentication
type AuthMiddleware struct {
	userRepo   *uRepo.UserRepository
	circleRepo *cRepo.CircleRepository
	jwtAuth    *ginJWT.GinJWTMiddleware
	config     *config.Config
	logger     *zap.SugaredLogger
}

// NewAuthMiddleware creates a new WebSocket auth middleware
func NewAuthMiddleware(
	userRepo *uRepo.UserRepository,
	circleRepo *cRepo.CircleRepository,
	jwtAuth *ginJWT.GinJWTMiddleware,
	config *config.Config,
) *AuthMiddleware {
	return &AuthMiddleware{
		userRepo:   userRepo,
		circleRepo: circleRepo,
		jwtAuth:    jwtAuth,
		config:     config,
	}
}

// AuthenticateConnection authenticates a WebSocket connection request
func (am *AuthMiddleware) AuthenticateConnection(c *gin.Context) (*uModel.User, int, error) {
	am.logger = logging.FromContext(c.Request.Context())

	// Extract token from query parameter or header
	token := am.extractToken(c)
	if token == "" {
		return nil, 0, ErrMissingToken
	}

	// Validate JWT token
	claims, err := am.validateToken(token)
	if err != nil {
		am.logger.Errorw("Invalid token for WebSocket connection", "error", err)
		return nil, 0, ErrInvalidToken
	}

	// Extract user ID from claims
	userID, err := am.extractUserIDFromClaims(c, claims)
	if err != nil {
		am.logger.Errorw("Invalid user ID in token", "error", err)
		return nil, 0, ErrInvalidToken
	}

	// Get user from database
	user, err := am.userRepo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		am.logger.Errorw("Failed to get user", "userID", userID, "error", err)
		return nil, 0, ErrInvalidToken
	}

	return user, user.CircleID, nil
}

// extractToken extracts the JWT token from the request
func (am *AuthMiddleware) extractToken(c *gin.Context) string {
	// Try Authorization header first (more secure)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	// see the cookies for the token
	if cookie, err := c.Cookie(am.jwtAuth.CookieName); err == nil && cookie != "" {
		return cookie
	}

	// Try WebSocket subprotocol for token (more secure than query params)
	if wsProtocols := c.Request.Header.Get("Sec-WebSocket-Protocol"); wsProtocols != "" {
		protocols := strings.Split(wsProtocols, ",")
		for _, protocol := range protocols {
			protocol = strings.TrimSpace(protocol)
			if strings.HasPrefix(protocol, "access_token.") {
				return strings.TrimPrefix(protocol, "access_token.")
			}
		}
	}

	return ""
}

// validateToken validates the JWT token and returns claims
func (am *AuthMiddleware) validateToken(tokenString string) (jwt.MapClaims, error) {
	// Parse the token using the same secret as the main JWT middleware
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		// Return the secret key used by the main JWT middleware
		return []byte(am.config.Jwt.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// extractUserIDFromClaims extracts the user ID from JWT claims
func (am *AuthMiddleware) extractUserIDFromClaims(c *gin.Context, claims jwt.MapClaims) (int, error) {
	// The JWT token stores the username in the "id" field, not the numeric user ID
	usernameInterface, exists := claims["id"]
	if !exists {
		return 0, ErrInvalidToken
	}

	username, ok := usernameInterface.(string)
	if !ok {
		return 0, ErrInvalidToken
	}

	// Look up the user by username to get the numeric user ID
	// Use request context for proper cancellation and tracing
	user, err := am.userRepo.GetUserByUsername(c.Request.Context(), username)
	if err != nil {
		am.logger.Errorw("Failed to get user by username",
			"username", "[REDACTED]", // Don't log actual username for security
			"error", err,
			"remote_addr", c.ClientIP())
		return 0, ErrInvalidToken
	}

	return user.ID, nil
}

// CheckCircleAccess verifies if a user has access to a specific circle
func (am *AuthMiddleware) CheckCircleAccess(ctx context.Context, userID, circleID int) error {
	user, err := am.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.CircleID != circleID {
		return ErrUnauthorizedCircle
	}

	return nil
}

// WebSocketAuthHandler creates a Gin handler for WebSocket authentication
func (am *AuthMiddleware) WebSocketAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, circleID, err := am.AuthenticateConnection(c)
		if err != nil {
			am.logger.Warnw("WebSocket authentication failed",
				"error", err,
				"remote_addr", c.ClientIP())

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication failed",
				"code":  "AUTH_FAILED",
			})
			c.Abort()
			return
		}
		if !user.IsPlusMember() {
			am.logger.Warnw("WebSocket access denied for non-plus member",
				"user_id", user.ID,
				"remote_addr", c.ClientIP())

			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied for non-plus members",
				"code":  "ACCESS_DENIED",
			})
			c.Abort()
			return
		}

		// Store user and circle ID in context for the WebSocket handler
		c.Set("user", user)
		c.Set("circleId", circleID)
		c.Next()
	}
}
