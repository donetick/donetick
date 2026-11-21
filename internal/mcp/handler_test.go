package mcp

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAuthentication(t *testing.T) {
	engine, db := newServer(t)
	require.NoError(t, db.Create(&uModel.User{ID: 9, Username: "gone", CircleID: 1, Disabled: true}).Error)
	require.NoError(t, db.Create(&uModel.APIToken{UserID: 9, Name: "gone", Token: "token-gone"}).Error)
	ping := legacy("ping", nil)

	for _, test := range []struct {
		name    string
		headers []string
		want    int
	}{
		{"no token", nil, http.StatusUnauthorized},
		{"unknown token", []string{"Authorization", "Bearer nope"}, http.StatusUnauthorized},
		{"disabled account", []string{"Authorization", "Bearer token-gone"}, http.StatusUnauthorized},
		{"not a bearer token", []string{"Authorization", "Basic " + alexToken}, http.StatusUnauthorized},
		{"bearer token", []string{"Authorization", "Bearer " + alexToken}, http.StatusOK},
		{"bearer scheme in lower case", []string{"Authorization", "bearer " + alexToken}, http.StatusOK},
		{"secretkey header", []string{"secretkey", alexToken}, http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := post(t, engine, "", ping, test.headers...)
			require.Equal(t, test.want, r.status, r.body)
			if test.want == http.StatusUnauthorized {
				require.Contains(t, r.header.Get("WWW-Authenticate"), "Bearer")
			}
		})
	}
}

func TestLegacyHandshake(t *testing.T) {
	engine, _ := newServer(t)

	for requested, want := range map[string]string{
		"2025-11-25": "2025-11-25",
		"2025-06-18": "2025-06-18",
		"2025-03-26": "2025-03-26",
		"2024-11-05": "2025-11-25", // too old: offer the newest handshake revision
		"2099-01-01": "2025-11-25",
	} {
		r := post(t, engine, alexToken, legacy("initialize", map[string]any{
			"protocolVersion": requested,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1"},
		}))
		require.Equal(t, http.StatusOK, r.status, r.body)
		require.Equal(t, want, r.result["protocolVersion"], "requested %s", requested)
		require.Equal(t, map[string]any{"tools": map[string]any{}}, r.result["capabilities"])
		require.Equal(t, "donetick", r.result["serverInfo"].(map[string]any)["name"])
		require.NotEmpty(t, r.result["instructions"])
		require.NotContains(t, r.result, "resultType", "handshake clients get the result shape of their revision")
	}

	r := post(t, engine, alexToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	require.Equal(t, http.StatusAccepted, r.status)
	require.Empty(t, r.body)

	// Clients older than 2025-06-18 send no version header after the handshake.
	for _, headers := range [][]string{nil, {headerProtocolVersion, "2025-06-18"}} {
		r = post(t, engine, alexToken, legacy("ping", nil), headers...)
		require.Equal(t, http.StatusOK, r.status, r.body)
		require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{}}`, r.body)

		r = post(t, engine, alexToken, legacy("tools/list", nil), headers...)
		require.Equal(t, http.StatusOK, r.status, r.body)
		require.Len(t, r.result["tools"], len(tools))
		require.NotContains(t, r.result, "ttlMs")
	}

	r = post(t, engine, alexToken, `{"jsonrpc":"2.0","id":"a","method":"ping","params":null}`)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":"a","result":{}}`, r.body, "null params are treated as no params")

	r = post(t, engine, alexToken, legacy("resources/list", nil))
	require.Equal(t, http.StatusOK, r.status, "handshake revisions report JSON-RPC errors in a 200 response")
	require.Equal(t, codeMethodNotFound, r.err.Code)
}

func TestModernRequests(t *testing.T) {
	engine, _ := newServer(t)
	serverInfo := map[string]any{"name": "donetick", "title": "Donetick", "version": "dev"}
	mustCall[map[string]any](t, engine, alexToken, "create_label", map[string]any{"name": "Kitchen"})

	body, headers := modern("server/discover", nil)
	r := post(t, engine, alexToken, body, headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Equal(t, "complete", r.result["resultType"])
	require.Equal(t, []any{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}, r.result["supportedVersions"])
	require.Equal(t, map[string]any{"tools": map[string]any{}}, r.result["capabilities"])
	require.Equal(t, map[string]any{metaServerInfo: serverInfo}, r.result["_meta"])
	require.NotEmpty(t, r.result["instructions"])
	require.EqualValues(t, cacheTTLMs, r.result["ttlMs"])
	require.Equal(t, "public", r.result["cacheScope"])

	body, headers = modern("tools/list", nil)
	r = post(t, engine, alexToken, body, headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Equal(t, "complete", r.result["resultType"])
	require.Len(t, r.result["tools"], len(tools))
	require.EqualValues(t, cacheTTLMs, r.result["ttlMs"])
	require.Equal(t, "public", r.result["cacheScope"])
	require.Equal(t, map[string]any{metaServerInfo: serverInfo}, r.result["_meta"])

	body, headers = modern("tools/call", map[string]any{"name": "list_labels", "arguments": map[string]any{}})
	r = post(t, engine, alexToken, body, headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Equal(t, "complete", r.result["resultType"])
	require.Equal(t, false, r.result["isError"])
	require.Contains(t, r.body, "Kitchen")

	// A tool name may arrive Base64-wrapped in its header.
	headers[len(headers)-1] = "=?base64?" + base64.StdEncoding.EncodeToString([]byte("list_labels")) + "?="
	r = post(t, engine, alexToken, body, headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Equal(t, false, r.result["isError"])
}

func TestModernRequestValidation(t *testing.T) {
	engine, _ := newServer(t)
	meta := func(version string) map[string]any {
		return map[string]any{metaProtocolVersion: version, metaClientCapabilities: map[string]any{}}
	}
	listTools := func(m map[string]any) string { return legacy("tools/list", map[string]any{"_meta": m}) }
	listLabels := legacy("tools/call", map[string]any{"name": "list_labels", "_meta": meta(modernVersion)})

	for _, test := range []struct {
		name       string
		body       string
		headers    []string
		wantStatus int
		wantCode   int
	}{
		{"version header missing", listTools(meta(modernVersion)),
			[]string{headerMethod, "tools/list"}, http.StatusBadRequest, codeHeaderMismatch},
		{"version header disagrees with the body", listTools(meta(modernVersion)),
			[]string{headerProtocolVersion, "2025-11-25", headerMethod, "tools/list"}, http.StatusBadRequest, codeHeaderMismatch},
		{"method header missing", listTools(meta(modernVersion)),
			[]string{headerProtocolVersion, modernVersion}, http.StatusBadRequest, codeHeaderMismatch},
		{"method header disagrees with the body", listTools(meta(modernVersion)),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call"}, http.StatusBadRequest, codeHeaderMismatch},
		{"name header missing", listLabels,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call"}, http.StatusBadRequest, codeHeaderMismatch},
		{"name header disagrees with the body", listLabels,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call", headerName, "delete_chore"}, http.StatusBadRequest, codeHeaderMismatch},
		{"name header is not valid Base64", listLabels,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call", headerName, "=?base64?%%%?="}, http.StatusBadRequest, codeHeaderMismatch},
		// Only names may be Base64-wrapped; a proxy must be able to read the method as it is.
		{"method header is Base64-wrapped", listTools(meta(modernVersion)),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "=?base64?dG9vbHMvbGlzdA==?="}, http.StatusBadRequest, codeHeaderMismatch},
		{"unsupported version", listTools(meta("2099-01-01")),
			[]string{headerProtocolVersion, "2099-01-01", headerMethod, "tools/list"}, http.StatusBadRequest, codeUnsupportedProtocolVersion},
		{"unsupported version in the header alone", legacy("tools/list", nil),
			[]string{headerProtocolVersion, "2099-01-01"}, http.StatusBadRequest, codeUnsupportedProtocolVersion},
		{"modern header without request metadata", legacy("tools/list", nil),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/list"}, http.StatusBadRequest, codeInvalidParams},
		{"client capabilities missing", listTools(map[string]any{metaProtocolVersion: modernVersion}),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/list"}, http.StatusBadRequest, codeInvalidParams},
		{"version is not a string", listTools(map[string]any{metaProtocolVersion: 20260728, metaClientCapabilities: map[string]any{}}),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/list"}, http.StatusBadRequest, codeInvalidParams},
		{"unknown method", legacy("ping", map[string]any{"_meta": meta(modernVersion)}),
			[]string{headerProtocolVersion, modernVersion, headerMethod, "ping"}, http.StatusNotFound, codeMethodNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := post(t, engine, alexToken, test.body, test.headers...)
			require.Equal(t, test.wantStatus, r.status, r.body)
			require.NotNil(t, r.err, r.body)
			require.Equal(t, test.wantCode, r.err.Code, r.body)
			if test.wantCode == codeUnsupportedProtocolVersion {
				require.Equal(t, "2099-01-01", r.errorData()["requested"])
				require.Equal(t, []any{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}, r.errorData()["supported"])
			}
		})
	}
}

func TestMalformedRequests(t *testing.T) {
	engine, _ := newServer(t)

	for _, test := range []struct {
		name       string
		body       string
		wantStatus int
		wantCode   int
	}{
		{"not JSON", `{"jsonrpc":`, http.StatusBadRequest, codeParseError},
		{"batch", `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, http.StatusBadRequest, codeInvalidRequest},
		{"not an object", `"ping"`, http.StatusBadRequest, codeInvalidRequest},
		{"wrong jsonrpc version", `{"jsonrpc":"1.0","id":1,"method":"ping"}`, http.StatusBadRequest, codeInvalidRequest},
		{"no method", `{"jsonrpc":"2.0","id":1}`, http.StatusBadRequest, codeInvalidRequest},
		{"null id", `{"jsonrpc":"2.0","id":null,"method":"ping"}`, http.StatusBadRequest, codeInvalidRequest},
		{"params is not an object", `{"jsonrpc":"2.0","id":1,"method":"ping","params":[1]}`, http.StatusBadRequest, codeInvalidRequest},
		{"_meta is not an object", `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"_meta":1}}`, http.StatusBadRequest, codeInvalidParams},
		// Member names are case-sensitive: "Method" is not "method".
		{"member name in the wrong case", `{"jsonrpc":"2.0","id":1,"Method":"ping"}`, http.StatusBadRequest, codeInvalidRequest},
		{"too large", `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"` + strings.Repeat("x", maxRequestBytes) + `"}}`,
			http.StatusRequestEntityTooLarge, codeInvalidRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := post(t, engine, alexToken, test.body)
			require.Equal(t, test.wantStatus, r.status, r.body)
			require.NotNil(t, r.err, r.body)
			require.Equal(t, test.wantCode, r.err.Code, r.body)
		})
	}

	t.Run("the id is echoed when it could be read", func(t *testing.T) {
		r := post(t, engine, alexToken, `{"jsonrpc":"2.0","id":"abc","method":7}`)
		require.JSONEq(t, `{"jsonrpc":"2.0","id":"abc","error":{"code":-32600,"message":"Invalid request: method is required"}}`, r.body)
	})

	t.Run("wrong content type", func(t *testing.T) {
		r := post(t, engine, alexToken, legacy("ping", nil), "Content-Type", "text/plain")
		require.Equal(t, http.StatusUnsupportedMediaType, r.status)
	})
}

func TestOnlyPostIsAllowed(t *testing.T) {
	engine, _ := newServer(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "/mcp", nil)
		request.Header.Set("Authorization", "Bearer "+alexToken)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusMethodNotAllowed, recorder.Code, method)
		require.Equal(t, http.MethodPost, recorder.Header().Get("Allow"))
	}
}

func TestEndpointIsOffUnlessEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig(t)
	cfg.MCP.Enabled = false
	engine := gin.New()
	Routes(cfg, engine, newHandler(cfg, tokens{}, engine), utils.NewRateLimiter(cfg))

	r := post(t, engine, alexToken, legacy("ping", nil))
	require.Equal(t, http.StatusNotFound, r.status)
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig(t)
	cfg.Server.RateLimit = 3
	engine := gin.New()
	Routes(cfg, engine, newHandler(cfg, tokens{alexToken: alex}, engine), utils.NewRateLimiter(cfg))

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, post(t, engine, alexToken, legacy("ping", nil)).status)
	}
	require.Equal(t, http.StatusTooManyRequests, post(t, engine, alexToken, legacy("ping", nil)).status)
	// The limiter runs before authentication, so guesses at a token count too.
	require.Equal(t, http.StatusTooManyRequests, post(t, engine, "guess", legacy("ping", nil)).status)
	// Other clients are unaffected.
	require.Equal(t, http.StatusOK, post(t, engine, alexToken, legacy("ping", nil), "X-Forwarded-For", "203.0.113.7").status)
}

// The API authorizes and rate limits by who is calling and from where, so a
// request made on a client's behalf has to carry that client's token and
// address, and nothing else it sent.
func TestAPIRequestsCarryTheCallersIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig(t)
	engine := gin.New()
	var seen *http.Request
	var seenIP string
	engine.GET("/api/v1/things", func(c *gin.Context) {
		seen, seenIP = c.Request.Clone(c.Request.Context()), c.ClientIP()
		c.JSON(http.StatusOK, gin.H{"res": []string{}})
	})
	engine.GET("/api/v1/labels", func(c *gin.Context) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not for you", "details": "ask an admin"})
	})
	engine.GET("/api/v1/projects", func(c *gin.Context) {
		panic("boom")
	})
	Routes(cfg, engine, newHandler(cfg, tokens{alexToken: alex}, engine), utils.NewRateLimiter(cfg))

	for _, test := range []struct {
		name    string
		headers []string
		wantIP  string
	}{
		{"direct connection", nil, "192.0.2.1"},
		{"behind a reverse proxy", []string{"X-Forwarded-For", "203.0.113.7"}, "203.0.113.7"},
		{"secretkey header", []string{"Authorization", "", "secretkey", alexToken}, "192.0.2.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, isError := callTool(t, engine, alexToken, "list_things", nil, test.headers...)
			require.False(t, isError)
			require.Equal(t, alexToken, seen.Header.Get(apiTokenHeader))
			require.Equal(t, test.wantIP, seenIP)
		})
	}

	t.Run("no other header reaches the API", func(t *testing.T) {
		// Impersonation is an API feature for the web app; a model must not
		// be able to switch users by having its client send the header.
		seen = nil
		_, isError := callTool(t, engine, alexToken, "list_things", nil, "X-Impersonate-User-ID", "2", "Cookie", "session=1", "Origin", "https://example.com")
		require.False(t, isError)
		require.NotNil(t, seen, "the request did not reach the API")
		require.Equal(t, http.Header{"Secretkey": {alexToken}}, seen.Header)
	})

	t.Run("an error from the API is passed on", func(t *testing.T) {
		text := mustFail(t, engine, alexToken, "list_labels", nil)
		require.Equal(t, "Donetick refused the request (HTTP 403): Not for you (ask an admin)", text)
	})

	t.Run("a crash in the API is an internal error", func(t *testing.T) {
		r := post(t, engine, alexToken, legacy("tools/call", map[string]any{"name": "list_projects"}))
		require.Equal(t, http.StatusOK, r.status, r.body)
		require.Equal(t, codeInternalError, r.err.Code)
		require.Equal(t, "Internal error", r.err.Message, "details stay in the server log")
	})
}
