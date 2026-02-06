package auth

import (
	"net/http"

	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

const apiKeyHeader = "secretkey"
const jwtPayloadKey = "JWT_PAYLOAD"

// MultiAuthMiddleware tries JWT first, then API key authentication
func MultiAuthMiddleware(jwtMiddleware *jwt.GinJWTMiddleware, userRepo *uRepo.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.FromContext(c)
		authenticated := false

		// Attempt 1: Try API key first
		authenticated = authenticateAPIKey(c, userRepo)

		// Attempt 2: If API key failed, try JWT
		if !authenticated {
			authenticated = authenticateJWT(c, jwtMiddleware)
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

// authenticateJWT attempts JWT authentication and returns true if successful
func authenticateJWT(c *gin.Context, jwtMiddleware *jwt.GinJWTMiddleware) bool {
	token, err := jwtMiddleware.ParseToken(c)
	if err != nil || !token.Valid {
		return false
	}

	claims, err := jwtMiddleware.GetClaimsFromJWT(c)
	if err != nil {
		return false
	}

	c.Set(jwtPayloadKey, claims)

	identity := jwtMiddleware.IdentityHandler(c)

	if identity != nil {
		c.Set(jwtMiddleware.IdentityKey, identity)
	}

	return jwtMiddleware.Authorizator(identity, c)
}

// authenticateAPIKey attempts API key authentication and returns true if successful
func authenticateAPIKey(c *gin.Context, userRepo *uRepo.UserRepository) bool {
	logger := logging.FromContext(c)

	apiToken := c.GetHeader(apiKeyHeader)

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
