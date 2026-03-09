package docs

import (
	"strconv"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	ServeSwagger bool
	ServerPort   int
}

func NewHandler(config *config.Config) *Handler {
	return &Handler{
		ServeSwagger: config.Server.ServeSwagger,
		ServerPort:   config.Server.Port,
	}
}

func Routes(router *gin.Engine, h *Handler) {

	SwaggerInfo.Title = "Donetick Swagger API"
	SwaggerInfo.Description = "Donetick swagger documentation."
	SwaggerInfo.Version = "1.0"
	SwaggerInfo.Host = "localhost" + ":" + strconv.Itoa(h.ServerPort) // TODO include public addr. and proper localhost.
	SwaggerInfo.BasePath = "/api/v1"
	SwaggerInfo.Schemes = []string{"http"}

	if h.ServeSwagger {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

}
