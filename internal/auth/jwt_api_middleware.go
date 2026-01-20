package auth

import (
	"net/http"

	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// JWTOrAPIKeyMiddleware tries JWT first, then API key authentication
func JWTOrAPIKeyMiddleware(jwtMiddleware *jwt.GinJWTMiddleware, userRepo *uRepo.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.FromContext(c)
		authenticated := false

		// Attempt 1: Try JWT authentication
		if tryJWTAuth(c, jwtMiddleware) {
			authenticated = true
		}

		// Attempt 2: If JWT failed, try API key
		if !authenticated {
			if tryAPIKeyAuth(c, userRepo) {
				authenticated = true
			}
		}

		// If neither succeeded, return unauthorized
		if !authenticated {
			logger.Error("Authentication failed: neither JWT nor API key provided/valid")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required. Provide either a valid JWT token or API key.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// tryJWTAuth attempts JWT authentication and returns true if successful
func tryJWTAuth(c *gin.Context, jwtMiddleware *jwt.GinJWTMiddleware) bool {
	token, err := jwtMiddleware.ParseToken(c)
	if err != nil || !token.Valid {
		return false
	}

	claims, err := jwtMiddleware.GetClaimsFromJWT(c)
	if err != nil {
		return false
	}

	c.Set("JWT_PAYLOAD", claims)
	identity := jwtMiddleware.IdentityHandler(c)

	if identity != nil {
		c.Set(jwtMiddleware.IdentityKey, identity)
	}

	return jwtMiddleware.Authorizator(identity, c)
}

// tryAPIKeyAuth attempts API key authentication and returns true if successful
func tryAPIKeyAuth(c *gin.Context, userRepo *uRepo.UserRepository) bool {
	logger := logging.FromContext(c)
	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		return false
	}

	user, err := userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		logger.Debugw("API token validation failed", "error", err)
		return false
	}

	if user.Disabled {
		logger.Debugw("User account is disabled", "userID", user.ID)
		return false
	}

	// Set the user in context
	c.Set(identityKey, user)
	return true
}
