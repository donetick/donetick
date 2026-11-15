package frontend

import (
	"bytes"
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"donetick.com/core/config"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

//go:embed dist
var embeddedFiles embed.FS

type Handler struct {
	ServeFrontend  bool
	BasePath       string
	BasePathHeader string

	indexOnce sync.Once
	indexHTML []byte
	indexErr  error
}

func NewHandler(config *config.Config) *Handler {
	return &Handler{
		ServeFrontend:  config.Server.ServeFrontend,
		BasePath:       config.Server.BasePath,
		BasePathHeader: config.Server.BasePathHeader,
	}
}

func Routes(router *gin.Engine, h *Handler) {
	// this whole logic is walkaround for serving frontend files
	// TODO: figure out better way to improve it. main issue i run into is failing over to index.html when file does not exist

	if h.ServeFrontend {
		// if file exists in dist folder, serve it
		router.Use(h.staticMiddleware("dist"))
		// if file does not exist in dist folder fallback to index.html
		router.NoRoute(h.serveIndex)

	}

}

// basePathFor returns the prefix to use for this request based on the following, in order of precedence:
//   - the value sent in BasePathHeader if configured and valid, an invalid header value is logged as a warning and ignored.
//   - the configured BasePath
func (h *Handler) basePathFor(c *gin.Context) string {
	if h.BasePathHeader != "" {
		if v := c.GetHeader(h.BasePathHeader); v != "" {
			if p, ok := config.SanitizeBasePath(v); ok {
				return p
			}
			logging.FromContext(c).Warnw("ignoring invalid base path header value; using server.base_path instead",
				"header", h.BasePathHeader,
				"value", v,
				"fallback", h.BasePath,
			)
		}
	}
	return h.BasePath
}

func (h *Handler) loadIndex() ([]byte, error) {
	h.indexOnce.Do(func() {
		h.indexHTML, h.indexErr = fs.ReadFile(embeddedFiles, "dist/index.html")
	})
	return h.indexHTML, h.indexErr
}

func (h *Handler) serveIndex(c *gin.Context) {
	index, err := h.loadIndex()
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	// index.html depends on the request when a prefix is in play and must never be cached, so app updates are picked up immediately.
	c.Header("Cache-Control", "no-cache")
	if h.BasePathHeader != "" {
		c.Header("Vary", h.BasePathHeader)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderIndex(index, h.basePathFor(c)))
}

// baseHrefTag is the placeholder emitted by the frontend build in index.html.
var baseHrefTag = []byte(`<base href="/" />`)

// renderIndex takes the original index.html and a basePath that has already passed config.SanitizeBasePath.
// It returns index.html set up to run under that path prefix.
// When basePath is "" (served at the root), it returns index unchanged.
func renderIndex(index []byte, basePath string) []byte {
	// return index unchanged when basePath is "" this ensures the common deployment case (served at the root) is not modified.
	if basePath == "" {
		return index
	}

	// basePath is validated by the caller, not here.
	// Escaping it for the HTML attribute and the JS string below is a second layer of protection.

	// The browser resolves relative URLs (the build emits assets as "./assets/...") against <base href> instead of the page URL, so they load from the prefix on every route.
	// The trailing "/" is required, or the browser drops the last segment when resolving.
	baseTag := `<base href="` + html.EscapeString(basePath) + `/" />`

	// <base> doesn't affect absolute paths the app builds in code (API calls, router routes, OAuth redirects).
	// The frontend reads this global and adds the prefix to those itself.
	jsValue, _ := json.Marshal(basePath)
	script := `<script>window.__DONETICK_BASE_PATH__=` + string(jsValue) + `</script>`

	inject := []byte(baseTag + script)

	// Current frontend builds ship a <base href="/" /> placeholder;
	// replace it so the page ends up with exactly one <base> tag.
	if bytes.Contains(index, baseHrefTag) {
		return bytes.Replace(index, baseHrefTag, inject, 1)
	}
	// Older frontend builds have no placeholder;
	// add the <base> and <script> right after <head> so they are present before any other tags that might use them.
	if i := bytes.Index(index, []byte("<head>")); i >= 0 {
		i += len("<head>")
		out := make([]byte, 0, len(index)+len(inject))
		out = append(out, index[:i]...)
		out = append(out, inject...)
		return append(out, index[i:]...)
	}
	// No <head> means this isn't a page we know how to change;
	// serve it as is.
	return index
}

func (h *Handler) staticMiddleware(root string) gin.HandlerFunc {
	fileServer := http.FileServer(getFileSystem(root))

	return func(c *gin.Context) {
		// "/" and "/index.html" go through serveIndex so the base path can be injected;
		// http.FileServer would serve the raw file.
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			if p := c.Request.URL.Path; p == "/" || p == "/index.html" {
				h.serveIndex(c)
				c.Abort()
				return
			}
		}
		_, err := fs.Stat(embeddedFiles, "dist"+c.Request.URL.Path)
		if err != nil {
			c.Next()
			return
		}
		// Cache-busted assets (filenames contain content hashes) can be
		// cached indefinitely. Other files like index.html must not be
		// cached so that updates are picked up immediately.
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
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
