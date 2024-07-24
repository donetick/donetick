package frontend

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed dist
var embeddedFiles embed.FS

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

func Routes(router *gin.Engine, h *Handler) {

	router.Use(staticMiddleware("dist"))
	router.Static("/assets", "dist/assets")

	// Gzip compression middleware
	router.Group("/assets").Use(func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=31536000, immutable")
		c.Next()
	})

}

func staticMiddleware(root string) gin.HandlerFunc {
	fileServer := http.FileServer(getFileSystem(root))

	return func(c *gin.Context) {
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
