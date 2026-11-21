package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

// Keep server lifecycle hooks registered but do not start background services.
type corsTestLifecycle struct{}

func (corsTestLifecycle) Append(fx.Hook) {}

func TestMCPBrowserCORS(t *testing.T) {
	const origin = "https://assistant.example.com"
	cfg := &config.Config{}
	cfg.Server.CorsAllowOrigins = []string{origin}
	engine := newServer(corsTestLifecycle{}, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	engine.POST("/mcp", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, test := range []struct {
		name   string
		origin string
		status int
	}{
		{"allowed origin", origin, http.StatusNoContent},
		{"untrusted origin", "https://untrusted.example.com", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", http.MethodPost)
			request.Header.Set("Access-Control-Request-Headers", "authorization,content-type,mcp-protocol-version,mcp-method,mcp-name")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, test.status, recorder.Code)
			if test.status == http.StatusForbidden {
				require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
				return
			}
			require.Equal(t, origin, recorder.Header().Get("Access-Control-Allow-Origin"))
			require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
			allowed := strings.Split(strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers")), ",")
			for _, header := range []string{"authorization", "content-type", "mcp-protocol-version", "mcp-method", "mcp-name"} {
				require.Contains(t, allowed, header)
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	request.Header.Set("Origin", origin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", "2026-07-28")
	request.Header.Set("Mcp-Method", "tools/call")
	request.Header.Set("Mcp-Name", "list_chores")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, origin, recorder.Header().Get("Access-Control-Allow-Origin"))
}
