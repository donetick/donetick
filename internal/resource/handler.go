package resource

import (
	"donetick.com/core/config"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
)

type Resource struct {
	Idp        identityProvider `json:"identity_provider" binding:"omitempty"`
	MinVersion string           `json:"min_version" binding:"omitempty"`
	APIVersion string           `json:"api_version" binding:"omitempty"`
	APICommit  string           `json:"api_commit" binding:"omitempty"`
}
type identityProvider struct {
	Auth_url  string `json:"auth_url" binding:"omitempty"`
	Client_ID string `json:"client_id" binding:"omitempty"`
	Name      string `json:"name" binding:"omitempty"`
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
		},
		MinVersion: h.config.MinVersion,
		APIVersion: h.config.Info.Version,
		APICommit:  h.config.Info.Commit,
	})
}

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {
	resourceRoutes := r.Group("api/v1/resource")

	resourceRoutes.GET("", h.getResource)

}
