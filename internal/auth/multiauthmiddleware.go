package auth

import (
	"errors"
	"net/http"

	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

const apiKeyHeader = "secretkey"
const jwtPayloadKey = "JWT_PAYLOAD"

type MultiAuthMiddleware struct {
	jwtMiddleware *jwt.GinJWTMiddleware
	userRepo      *uRepo.UserRepository
}

func NewMultiAuthMiddleware(jwtMiddleware *jwt.GinJWTMiddleware, userRepo *uRepo.UserRepository) *MultiAuthMiddleware {
	return &MultiAuthMiddleware{
		jwtMiddleware: jwtMiddleware,
		userRepo:      userRepo,
	}
}

// MultiAuthMiddleware tries JWT first, then API key authentication
func (h *MultiAuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.FromContext(c)

		// Authenticate using API key first, it's just's a header check. if fails, try JWT next
		var authenticated, err = authenticateAPIKey(c, h.userRepo)
		if authenticated {
			c.Next()
			return
		}

		if err.Error() == "API token is invalid." {
			authenticated, err = authenticateJWT(c, h.jwtMiddleware)
			if authenticated {
				c.Next()
				return
			}
		}

		// If neither succeeded, return unauthorized
		if !authenticated {
			logger.Error("Authentication failed:", err)
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
func authenticateJWT(c *gin.Context, jwtMiddleware *jwt.GinJWTMiddleware) (bool, error) {

	token, err := jwtMiddleware.ParseToken(c)
	if err != nil || !token.Valid {
		return false, errors.New("JWT is invalid.")
	}

	claims, err := jwtMiddleware.GetClaimsFromJWT(c)
	if err != nil {
		return false, errors.New("Unable to get claims from JWT.")
	}

	c.Set(jwtPayloadKey, claims)

	identity := jwtMiddleware.IdentityHandler(c)

	if identity != nil {
		c.Set(jwtMiddleware.IdentityKey, identity)
	}

	return jwtMiddleware.Authorizator(identity, c), nil
}

// authenticateAPIKey attempts API key authentication and returns true if successful
func authenticateAPIKey(c *gin.Context, userRepo *uRepo.UserRepository) (bool, error) {
	apiToken := c.GetHeader(apiKeyHeader)

	if apiToken == "" {
		return false, errors.New("API token is invalid.")
	}

	user, err := userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		return false, errors.New("Unable to find user associated to API token.")
	}

	if user.Disabled {
		return false, errors.New("User account is disabled")
	}

	// Set the user in context
	c.Set(identityKey, user)
	return true, nil
}
