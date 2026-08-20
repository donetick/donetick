package resource

import (
	"donetick.com/core/config"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
)

type Resource struct {
	Idp                    identityProvider `json:"identity_provider" binding:"omitempty"`
	MinVersion             string           `json:"min_version" binding:"omitempty"`
	APIVersion             string           `json:"api_version" binding:"omitempty"`
	APICommit              string           `json:"api_commit" binding:"omitempty"`
	IsUserCreationDisabled bool             `json:"is_user_creation_disabled"`
	SingleCircleInstance   bool             `json:"single_circle_instance"`
	// DisablePasswordAuth tells the client to hide username/password login and
	// signup, leaving only SSO (#438).
	DisablePasswordAuth bool `json:"disable_password_auth"`
}
type identityProvider struct {
	Auth_url  string   `json:"auth_url" binding:"omitempty"`
	Client_ID string   `json:"client_id" binding:"omitempty"`
	Name      string   `json:"name" binding:"omitempty"`
	Scopes    []string `json:"scopes" binding:"omitempty"`
	PKCE      bool     `json:"pkce" binding:"omitempty"`
}

type Handler struct {
	config config.Config
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		config: *cfg,
	}
}

func (h *Handler) getResource(c *gin.Context) {
	c.JSON(200, &Resource{
		Idp: identityProvider{
			Auth_url:  h.config.OAuth2Config.AuthURL,
			Client_ID: h.config.OAuth2Config.ClientID,
			Name:      h.config.OAuth2Config.Name,
			Scopes:    h.config.OAuth2Config.Scopes,
			PKCE:      h.config.OAuth2Config.PKCE,
		},
		MinVersion:             h.config.MinVersion,
		APIVersion:             h.config.Info.Version,
		APICommit:              h.config.Info.Commit,
		IsUserCreationDisabled: h.config.IsUserCreationDisabled,
		SingleCircleInstance:   h.config.SingleCircleInstance,
		DisablePasswordAuth:    h.config.DisablePasswordAuth,
	})
}

func (h *Handler) getHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "healthy",
		"version": h.config.Info.Version,
		"commit":  h.config.Info.Commit,
	})
}

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {
	// No-auth health check endpoint for container/orchestrator health checks.
	r.GET("/health", h.getHealth)

	resourceRoutes := r.Group("api/v1/resource")

	resourceRoutes.GET("", h.getResource)

}
