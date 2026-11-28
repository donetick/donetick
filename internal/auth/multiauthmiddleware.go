package auth

import (
	"errors"
	"net/http"

	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
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

// MultiAuthMiddleware tries API key authentication first, then JWT
func (h *MultiAuthMiddleware) MiddlewareFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(apiKeyHeader) != "" {
			if ok, _ := authenticateAPIKey(c, h.userRepo); !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token"})
				return
			}
			token := c.MustGet(apiTokenContextKey).(*uModel.APIToken)
			user, _ := CurrentUser(c)
			if (token.CircleID != nil && *token.CircleID != user.CircleID) || !tokenHasScope(token.Scopes, requiredTokenScope(c.Request.Method, c.FullPath())) {
				denyTokenScope(c)
				return
			}
			// Usage metadata is best effort and must not change authorization.
			_ = h.userRepo.TouchAPIToken(c.Request.Context(), token.ID)
			c.Next()
			return
		}
		if ok, _ := authenticateJWT(c, h.jwtMiddleware); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		c.Next()
	}
}

// authenticateJWT attempts JWT authentication and returns true if successful
func authenticateJWT(c *gin.Context, jwtMiddleware *jwt.GinJWTMiddleware) (bool, error) {

	if jwtMiddleware == nil {
		return false, errors.New("JWT authentication unavailable")
	}
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

	token, err := userRepo.GetAPIToken(c.Request.Context(), apiToken)
	if err != nil {
		return false, errors.New("Invalid API token")
	}
	c.Set(apiTokenContextKey, token)
	// Set the user in context
	c.Set(identityKey, user)
	return true, nil
}
