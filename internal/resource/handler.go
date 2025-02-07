package resource

import (
	"donetick.com/core/config"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
)

type Resource struct {
	Idp identityProvider `json:"identity_provider" binding:"omitempty"`
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
	})
}

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {
	resourceRoutes := r.Group("api/v1/resource")

	// skip resource endpoint for donetick.com
	if h.config.IsDoneTickDotCom {
		resourceRoutes.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{})
		})
		return
	}
	resourceRoutes.GET("", h.getResource)
}
