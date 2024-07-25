package frontend

import (
	"embed"
	"io/fs"
	"net/http"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
)

//go:embed dist
var embeddedFiles embed.FS

type Handler struct {
	ServeFrontend bool
}

func NewHandler(config *config.Config) *Handler {
	return &Handler{
		ServeFrontend: config.Server.ServeFrontend,
	}
}

func Routes(router *gin.Engine, h *Handler) {
	// this whole logic is walkaround for serving frontend files
	// TODO: figure out better way to improve it. main issue i run into is failing over to index.html when file does not exist

	if h.ServeFrontend {
		// if file exists in dist folder, serve it
		router.Use(staticMiddleware("dist"))
		// if file does not exist in dist folder fallback to index.html
		router.NoRoute(staticMiddlewareNoRoute("dist"))

	}

}

func staticMiddleware(root string) gin.HandlerFunc {
	fileServer := http.FileServer(getFileSystem(root))

	return func(c *gin.Context) {
		_, err := fs.Stat(embeddedFiles, "dist"+c.Request.URL.Path)
		if err != nil {
			c.Next()
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)

	}
}
func staticMiddlewareNoRoute(root string) gin.HandlerFunc {
	fileServer := http.FileServer(getFileSystem(root))

	// always serve index.html for any route does not match:
	return func(c *gin.Context) {
		// Rewrite all requests to serve index.html
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)

	}
}

func getFileSystem(path string) http.FileSystem {
	fs, err := fs.Sub(embeddedFiles, path)
	if err != nil {
		panic(err)
	}
	return http.FS(fs)
}
