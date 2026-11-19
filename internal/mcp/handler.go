// Package mcp serves Donetick's built-in Model Context Protocol endpoint.
//
// AI assistants connect to /mcp with a Donetick API token and get a set of
// tools for working with chores, labels, projects, filters and things. Each
// tool is a thin wrapper over the HTTP API (/api/v1): a call is replayed
// against the API inside the process, as the owner of the token. Access
// checks, validation, webhooks and notifications are therefore the API's own,
// and there is no second copy of that logic to keep in sync.
//
// Nothing that manages users, circles or API tokens is exposed.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
)

const (
	apiTokenHeader = "secretkey"
	tokenKey       = "mcp_api_token"
	userIDKey      = "mcp_user_id"

	// maxRequestBytes bounds a JSON-RPC message. Tool calls carry a handful of
	// short fields, so this is generous.
	maxRequestBytes = 1 << 20

	// cacheTTLMs tells clients how long the tool list stays fresh. It only
	// changes when the server is upgraded.
	cacheTTLMs = 5 * 60 * 1000

	instructions = "Donetick is a task and chore manager shared by a household or team, called a circle. " +
		"Everything is addressed by numeric id: call the matching list tool first to find the id you need. " +
		"Dates and times are RFC 3339 timestamps, for example 2026-01-31T18:00:00Z."
)

type tokenLookup interface {
	GetUserByToken(ctx context.Context, token string) (*uModel.UserDetails, error)
}

// Handler serves the MCP endpoint. It holds the engine the rest of the API is
// registered on, which is where tool calls are sent.
type Handler struct {
	users      tokenLookup
	api        *gin.Engine
	serverInfo map[string]any
}

func NewHandler(cfg *config.Config, userRepo *uRepo.UserRepository, engine *gin.Engine) *Handler {
	return newHandler(cfg, userRepo, engine)
}

func newHandler(cfg *config.Config, users tokenLookup, engine *gin.Engine) *Handler {
	version := cfg.Info.Version
	if version == "" {
		version = "dev"
	}
	return &Handler{
		users: users,
		api:   engine,
		serverInfo: map[string]any{
			"name":    "donetick",
			"title":   "Donetick",
			"version": version,
		},
	}
}

// authenticate accepts a Donetick API token in the secretkey header used by
// the rest of the API, or as a bearer token, which is what most MCP clients
// know how to send.
func (h *Handler) authenticate(c *gin.Context) {
	token := c.GetHeader(apiTokenHeader)
	if token == "" {
		if scheme, value, ok := strings.Cut(c.GetHeader("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
			token = strings.TrimSpace(value)
		}
	}
	if token == "" {
		unauthorized(c, "API token required")
		return
	}
	user, err := h.users.GetUserByToken(c.Request.Context(), token)
	if err != nil {
		unauthorized(c, "Invalid API token")
		return
	}
	if user.Disabled {
		unauthorized(c, "User account is disabled")
		return
	}
	c.Set(tokenKey, token)
	c.Set(userIDKey, user.ID)
	c.Next()
}

func unauthorized(c *gin.Context, reason string) {
	c.Header("WWW-Authenticate", `Bearer realm="donetick"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": reason})
}

func (h *Handler) handlePost(c *gin.Context) {
	if !strings.EqualFold(c.ContentType(), "application/json") {
		writeFailure(c, nil, &failure{
			status:  http.StatusUnsupportedMediaType,
			code:    codeInvalidRequest,
			message: "Invalid request: Content-Type must be application/json",
		})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes))
	if err != nil {
		fail := badRequest(codeInvalidRequest, "Invalid request: the request body could not be read")
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail.status = http.StatusRequestEntityTooLarge
		}
		writeFailure(c, nil, fail)
		return
	}

	msg, fail := decodeMessage(body)
	if fail != nil {
		var id json.RawMessage
		if msg != nil {
			id = msg.id
		}
		writeFailure(c, id, fail)
		return
	}
	if msg.isNotification() {
		// Nothing a client can notify this server of needs acting on.
		c.Status(http.StatusAccepted)
		return
	}

	modern, fail := negotiate(c.Request.Header, msg)
	if fail != nil {
		writeFailure(c, msg.id, fail)
		return
	}
	var result map[string]any
	if modern {
		result, fail = h.serveModern(c, msg)
	} else {
		result, fail = h.serveLegacy(c, msg)
	}
	if fail != nil {
		writeFailure(c, msg.id, fail)
		return
	}
	c.JSON(http.StatusOK, response{JSONRPC: "2.0", ID: msg.id, Result: result})
}

func writeFailure(c *gin.Context, id json.RawMessage, fail *failure) {
	c.JSON(fail.status, response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: fail.code, Message: fail.message, Data: fail.data},
	})
}

func methodNotAllowed(c *gin.Context) {
	// The server never opens a stream of its own and keeps no session a
	// client could end, so POST is the only method.
	c.Header("Allow", http.MethodPost)
	c.Status(http.StatusMethodNotAllowed)
}

func capabilities() map[string]any {
	return map[string]any{"tools": map[string]any{}}
}

// serveLegacy answers clients that open with an initialize handshake
// (protocol revisions up to 2025-11-25).
func (h *Handler) serveLegacy(c *gin.Context, msg *message) (map[string]any, *failure) {
	switch msg.method {
	case "initialize":
		version := legacyVersions[0]
		var requested string
		if json.Unmarshal(msg.params["protocolVersion"], &requested) == nil && slices.Contains(legacyVersions, requested) {
			version = requested
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    capabilities(),
			"serverInfo":      h.serverInfo,
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		return h.callTool(c, msg)
	}
	return nil, &failure{status: http.StatusOK, code: codeMethodNotFound, message: "Method not found: " + msg.method}
}

// serveModern answers stateless requests (protocol revision 2026-07-28).
func (h *Handler) serveModern(c *gin.Context, msg *message) (map[string]any, *failure) {
	var result map[string]any
	switch msg.method {
	case "server/discover":
		result = map[string]any{
			"supportedVersions": supportedVersions,
			"capabilities":      capabilities(),
			"instructions":      instructions,
			"ttlMs":             cacheTTLMs,
			"cacheScope":        "public",
		}
	case "tools/list":
		result = map[string]any{"tools": tools, "ttlMs": cacheTTLMs, "cacheScope": "public"}
	case "tools/call":
		var fail *failure
		if result, fail = h.callTool(c, msg); fail != nil {
			return nil, fail
		}
	default:
		return nil, &failure{status: http.StatusNotFound, code: codeMethodNotFound, message: "Method not found: " + msg.method}
	}
	result["resultType"] = "complete"
	result["_meta"] = map[string]any{metaServerInfo: h.serverInfo}
	return result, nil
}

// callTool runs a tool. A request that names no known tool is a protocol
// error; anything that goes wrong while running one is reported in the result
// so the model can read it and correct the call.
func (h *Handler) callTool(c *gin.Context, msg *message) (map[string]any, *failure) {
	var name string
	if err := json.Unmarshal(msg.params["name"], &name); err != nil || name == "" {
		return nil, &failure{status: http.StatusOK, code: codeInvalidParams, message: "Invalid params: tools/call requires the name of a tool"}
	}
	t, ok := findTool(name)
	if !ok {
		return nil, &failure{status: http.StatusOK, code: codeInvalidParams, message: "Unknown tool: " + name}
	}

	response, err := t.run(&session{
		userID: c.GetInt(userIDKey),
		call:   func(call apiCall) (apiResponse, error) { return h.callAPI(c, call) },
	}, msg.params["arguments"])
	var refused *refusal
	var internal *internalError
	switch {
	case errors.As(err, &internal):
		logging.FromContext(c).Errorw("mcp: tool call failed", "tool", name, "error", err)
		return nil, &failure{status: http.StatusOK, code: codeInternalError, message: "Internal error"}
	case errors.As(err, &refused):
		response = refused.response
	case err != nil:
		return textResult(fmt.Sprintf("Invalid arguments for %s: %s", name, err), true), nil
	}
	logging.FromContext(c).Debugw("mcp: tool called", "tool", name, "status", response.status)

	if !response.ok() {
		return textResult(apiErrorText(response), true), nil
	}
	text := strings.TrimSpace(string(unescapeHTML(response.body)))
	if text == "" || text == "{}" {
		text = "Done."
	}
	return textResult(text, false), nil
}

func textResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

// unescapeHTML undoes the \u0026, \u003c and \u003e escapes the API's JSON
// encoder writes for &, < and >. They only matter when JSON is embedded in
// HTML, and a model reads "Wash & fold" more reliably than "Wash \u0026 fold".
func unescapeHTML(body []byte) []byte {
	if !bytes.Contains(body, []byte(`\u00`)) {
		return body
	}
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 == len(body) {
			out = append(out, body[i])
			continue
		}
		if body[i+1] == 'u' && i+5 < len(body) {
			if char, ok := htmlEscapes[string(body[i+2:i+6])]; ok {
				out = append(out, char)
				i += 5
				continue
			}
		}
		// Copy any other escape whole, so the second character of "\\" is
		// never taken for the start of the next one.
		out = append(out, body[i], body[i+1])
		i++
	}
	return out
}

var htmlEscapes = map[string]byte{"0026": '&', "003c": '<', "003e": '>'}

// apiErrorText turns an error response of the API into a sentence for the
// model, keeping the API's own explanation when it gave one.
func apiErrorText(response apiResponse) string {
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Details string `json:"details"`
	}
	_ = json.Unmarshal(response.body, &payload)
	reason := payload.Error
	if reason == "" {
		reason = payload.Message
	}
	if reason == "" {
		reason = http.StatusText(response.status)
	}
	if payload.Details != "" {
		reason += " (" + payload.Details + ")"
	}
	return fmt.Sprintf("Donetick refused the request (HTTP %d): %s", response.status, reason)
}

// callAPI replays a tool call against the HTTP API without leaving the
// process. The request carries the caller's own token and network address,
// and nothing else of the original request: the API authorizes it exactly as
// if the client had called the route directly.
func (h *Handler) callAPI(c *gin.Context, call apiCall) (response apiResponse, err error) {
	var payload io.Reader
	if call.body != nil {
		encoded, err := json.Marshal(call.body)
		if err != nil {
			return apiResponse{}, &internalError{err}
		}
		payload = bytes.NewReader(encoded)
	}
	target := call.path
	if len(call.query) > 0 {
		target += "?" + call.query.Encode()
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), call.method, target, payload)
	if err != nil {
		return apiResponse{}, &internalError{err}
	}
	request.Header.Set(apiTokenHeader, c.GetString(tokenKey))
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	// Whatever in the API looks at the client's address, such as a rate
	// limit, should see the real client and not a shared, empty one.
	request.RemoteAddr = c.Request.RemoteAddr
	for _, name := range append([]string{h.api.TrustedPlatform}, h.api.RemoteIPHeaders...) {
		if name == "" {
			continue
		}
		if values := c.Request.Header.Values(name); len(values) > 0 {
			request.Header[http.CanonicalHeaderKey(name)] = values
		}
	}

	recorder := &responseRecorder{header: http.Header{}, status: http.StatusOK}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &internalError{fmt.Errorf("panic in %s %s: %v", call.method, call.path, recovered)}
		}
	}()
	h.api.ServeHTTP(recorder, request)
	return apiResponse{status: recorder.status, body: recorder.body.Bytes()}, nil
}

// responseRecorder collects the response of an in-process API request.
type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) Header() http.Header         { return r.header }
func (r *responseRecorder) WriteHeader(status int)      { r.status = status }
func (r *responseRecorder) Write(p []byte) (int, error) { return r.body.Write(p) }

// Routes registers the MCP endpoint when it is enabled in the configuration.
func Routes(cfg *config.Config, r *gin.Engine, h *Handler, limiter *limiter.Limiter) {
	if !cfg.MCP.Enabled {
		return
	}
	endpoint := r.Group("mcp")
	endpoint.Use(utils.RateLimitMiddleware(limiter), h.authenticate)
	{
		endpoint.POST("", h.handlePost)
		endpoint.GET("", methodNotAllowed)
		endpoint.DELETE("", methodNotAllowed)
	}
}
