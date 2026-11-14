package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/chore"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/events"
	lModel "donetick.com/core/internal/label/model"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	pModel "donetick.com/core/internal/project/model"
	pjRepo "donetick.com/core/internal/project/repo"
	storageModel "donetick.com/core/internal/storage/model"
	stModel "donetick.com/core/internal/subtask/model"
	stRepo "donetick.com/core/internal/subtask/repo"
	syncModel "donetick.com/core/internal/sync/model"
	"donetick.com/core/internal/thing"
	tModel "donetick.com/core/internal/thing/model"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	alexToken = "token-alex" // admin of circle 1
	samToken  = "token-sam"  // member of circle 1
	patToken  = "token-pat"  // admin of circle 2
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.MCP.Enabled = true
	cfg.Server.RateLimit = 1000
	cfg.Server.RatePeriod = time.Minute
	cfg.Server.WriteTimeout = 5 * time.Second
	cfg.WebhookConfig.QueueSize = 10
	return cfg
}

// newServer returns an engine serving /mcp in front of the real external API
// routes and an in-memory database, so the tool tests also catch a tool that
// has drifted from the route it wraps.
func newServer(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(
		&uModel.User{}, &uModel.APIToken{}, &uModel.UserNotificationTarget{},
		&cModel.Circle{}, &cModel.UserCircle{}, &chModel.Chore{}, &chModel.ChoreAssignees{},
		&chModel.ChoreHistory{}, &chModel.TimeSession{}, &chModel.ChoreLabels{},
		&stModel.SubTask{}, &lModel.Label{}, &pModel.Project{}, &pModel.ProjectMember{},
		&tModel.Thing{}, &tModel.ThingChore{}, &tModel.ThingHistory{},
		&nModel.Notification{}, &storageModel.StorageFile{}, &syncModel.SyncCursor{}, &syncModel.Tombstone{},
	))

	cfg := testConfig()
	users := uRepo.NewUserRepository(db, cfg)
	circles := cRepo.NewCircleRepository(db)
	chores := chRepo.NewChoreRepository(db, cfg)
	things := tRepo.NewThingRepository(db, cfg)
	notifications := nRepo.NewNotificationRepository(db)
	planner := nps.NewNotificationPlanner(notifications, circles, users, dRepo.NewDeviceRepository(db, cfg))
	producer := events.NewEventsProducer(cfg)
	producer.Start(context.Background())
	limiter := utils.NewRateLimiter(cfg)

	engine := gin.New()
	chore.APIs(cfg, chore.NewAPI(chores, users, circles, planner, notifications, producer,
		stRepo.NewSubTasksRepository(db), pjRepo.NewProjectRepository(db, cfg, chores)), engine, nil, limiter, users)
	thing.APIs(cfg, thing.NewAPI(chores, circles, things, users, things, producer), engine, nil, users)
	Routes(cfg, engine, newHandler(cfg, users, engine), limiter)

	for _, u := range []struct {
		id, circle  int
		name, token string
		role        cModel.UserRole
	}{
		{1, 1, "alex", alexToken, cModel.UserRoleAdmin},
		{2, 1, "sam", samToken, cModel.UserRoleMember},
		{3, 2, "pat", patToken, cModel.UserRoleAdmin},
	} {
		require.NoError(t, db.Create(&uModel.User{ID: u.id, Username: u.name, DisplayName: u.name, CircleID: u.circle}).Error)
		require.NoError(t, db.Create(&uModel.APIToken{UserID: u.id, Name: u.name, Token: u.token}).Error)
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: u.id, CircleID: u.circle, Role: u.role, IsActive: true}).Error)
	}
	require.NoError(t, db.Create(&tModel.Thing{ID: 1, UserID: 1, Name: "Dishwasher loads", Type: "number", State: "3"}).Error)
	require.NoError(t, db.Create(&tModel.Thing{ID: 2, UserID: 3, Name: "Vacation mode", Type: "boolean", State: "false"}).Error)
	return engine, db
}

// reply is the HTTP response to a request sent to /mcp.
type reply struct {
	status int
	header http.Header
	body   string
	result map[string]any
	err    *rpcError
}

func (r reply) errorData() map[string]any {
	data, _ := r.err.Data.(map[string]any)
	return data
}

// post sends body to /mcp with the API token as a bearer token. Extra headers
// are given as name, value pairs.
func post(t *testing.T, engine *gin.Engine, token, body string, headers ...string) reply {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	r := reply{status: recorder.Code, header: recorder.Header(), body: recorder.Body.String()}
	var decoded struct {
		Result map[string]any `json:"result"`
		Error  *rpcError      `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &decoded) == nil {
		r.result, r.err = decoded.Result, decoded.Error
	}
	return r
}

// legacy builds a request the way a client does after an initialize handshake.
func legacy(method string, params map[string]any) string {
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		request["params"] = params
	}
	body, _ := json.Marshal(request)
	return string(body)
}

// modern builds a stateless (2026-07-28) request and the headers that must
// mirror its body.
func modern(method string, params map[string]any) (body string, headers []string) {
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		metaProtocolVersion:    modernVersion,
		metaClientCapabilities: map[string]any{},
	}
	headers = []string{headerProtocolVersion, modernVersion, headerMethod, method}
	if name, ok := params["name"].(string); ok {
		headers = append(headers, headerName, name)
	}
	return legacy(method, params), headers
}

// callTool calls a tool as a legacy client and returns the text of its result.
func callTool(t *testing.T, engine *gin.Engine, token, name string, arguments map[string]any) (text string, isError bool) {
	t.Helper()
	r := post(t, engine, token, legacy("tools/call", map[string]any{"name": name, "arguments": arguments}),
		headerProtocolVersion, legacyVersions[0])
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Nil(t, r.err, r.body)
	content := r.result["content"].([]any)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	require.Equal(t, "text", block["type"])
	return block["text"].(string), r.result["isError"].(bool)
}

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

	body, headers = modern("tools/call", map[string]any{"name": "get_thing", "arguments": map[string]any{"id": 1}})
	r = post(t, engine, alexToken, body, headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Equal(t, "complete", r.result["resultType"])
	require.Equal(t, false, r.result["isError"])
	require.Contains(t, r.body, "Dishwasher loads")

	// A tool name may arrive Base64-wrapped in its header.
	headers[len(headers)-1] = "=?base64?" + base64.StdEncoding.EncodeToString([]byte("get_thing")) + "?="
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
	getThing := legacy("tools/call", map[string]any{"name": "get_thing", "arguments": map[string]any{"id": 1}, "_meta": meta(modernVersion)})

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
		{"name header missing", getThing,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call"}, http.StatusBadRequest, codeHeaderMismatch},
		{"name header disagrees with the body", getThing,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call", headerName, "delete_chore"}, http.StatusBadRequest, codeHeaderMismatch},
		{"name header is not valid Base64", getThing,
			[]string{headerProtocolVersion, modernVersion, headerMethod, "tools/call", headerName, "=?base64?%%%?="}, http.StatusBadRequest, codeHeaderMismatch},
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
	cfg := testConfig()
	cfg.MCP.Enabled = false
	engine := gin.New()
	Routes(cfg, engine, newHandler(cfg, tokens{}, engine), utils.NewRateLimiter(cfg))

	r := post(t, engine, alexToken, legacy("ping", nil))
	require.Equal(t, http.StatusNotFound, r.status)
}

func TestToolDefinitions(t *testing.T) {
	validName := regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	seen := map[string]bool{}
	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			require.Regexp(t, validName, tool.Name)
			require.False(t, seen[tool.Name], "duplicate tool name")
			seen[tool.Name] = true
			require.NotEmpty(t, tool.Title)
			require.NotEmpty(t, tool.Description)
			require.NotNil(t, tool.build)

			require.Equal(t, "object", tool.InputSchema["type"])
			properties, _ := tool.InputSchema["properties"].(schema)
			required, _ := tool.InputSchema["required"].([]string)
			for _, name := range required {
				require.Contains(t, properties, name, "required argument is not declared")
			}
			for name, property := range properties {
				require.NotEmpty(t, property.(schema)["description"], "argument %s has no description", name)
			}

			// A call that leaves out a required argument must be turned down
			// before it reaches the API.
			if len(required) > 0 {
				_, err := tool.build(json.RawMessage(`{}`))
				require.Error(t, err)
			}
			_, err := tool.build(json.RawMessage(`{"noSuchArgument":true}`))
			require.ErrorContains(t, err, "noSuchArgument")
		})
	}
}

func TestChoreTools(t *testing.T) {
	engine, db := newServer(t)

	text, isError := callTool(t, engine, alexToken, "create_chore", map[string]any{
		"name": "Water the plants", "description": "Ferns too", "dueDate": "2030-01-15",
	})
	require.False(t, isError, text)
	var created chModel.Chore
	require.NoError(t, json.Unmarshal([]byte(text), &created))
	require.Equal(t, "Water the plants", created.Name)
	require.Equal(t, "Ferns too", *created.Description)
	require.Equal(t, "2030-01-15", created.NextDueDate.Format(time.DateOnly))
	require.Equal(t, 1, created.CreatedBy)
	id := created.ID

	text, isError = callTool(t, engine, alexToken, "list_chores", nil)
	require.False(t, isError, text)
	var listed []chModel.Chore
	require.NoError(t, json.Unmarshal([]byte(text), &listed))
	require.Len(t, listed, 1)
	require.Equal(t, id, listed[0].ID)

	t.Run("chores stay inside their circle", func(t *testing.T) {
		text, isError := callTool(t, engine, patToken, "list_chores", nil)
		require.False(t, isError, text)
		require.JSONEq(t, `[]`, text)

		text, isError = callTool(t, engine, patToken, "update_chore", map[string]any{"id": id, "name": "Mine now"})
		require.True(t, isError)
		require.Contains(t, text, "Chore not found")
	})

	t.Run("the API's permission checks apply", func(t *testing.T) {
		text, isError := callTool(t, engine, samToken, "delete_chore", map[string]any{"id": id})
		require.True(t, isError)
		require.Contains(t, text, "HTTP 403")
		require.Contains(t, text, "Only the chore creator or a circle admin/manager can delete this chore")
	})

	text, isError = callTool(t, engine, alexToken, "update_chore", map[string]any{"id": id, "name": "Water the ferns", "dueDate": "2030-02-01T09:30:00Z"})
	require.False(t, isError, text)
	var updated chModel.Chore
	require.NoError(t, json.Unmarshal([]byte(text), &updated))
	require.Equal(t, "Water the ferns", updated.Name)
	require.Equal(t, "Ferns too", *updated.Description, "fields that are left out keep their value")
	require.Equal(t, "2030-02-01T09:30:00Z", updated.NextDueDate.UTC().Format(time.RFC3339))

	// The chore is assigned to alex, so the API will not let it be completed in sam's name.
	text, isError = callTool(t, engine, alexToken, "complete_chore", map[string]any{"id": id, "completedBy": 2})
	require.True(t, isError)
	require.Contains(t, text, "User is not assigned to chore")

	text, isError = callTool(t, engine, alexToken, "complete_chore", map[string]any{"id": id})
	require.False(t, isError, text)
	var history chModel.ChoreHistory
	require.NoError(t, db.Where("chore_id = ?", id).First(&history).Error)
	require.Equal(t, 1, history.CompletedBy)

	text, isError = callTool(t, engine, alexToken, "delete_chore", map[string]any{"id": id})
	require.False(t, isError, text)
	require.ErrorIs(t, db.First(&chModel.Chore{}, id).Error, gorm.ErrRecordNotFound)

	text, isError = callTool(t, engine, alexToken, "list_circle_members", nil)
	require.False(t, isError, text)
	require.Contains(t, text, `"username":"alex"`)
	require.Contains(t, text, `"username":"sam"`)
	require.NotContains(t, text, `"username":"pat"`)
}

func TestThingTools(t *testing.T) {
	engine, db := newServer(t)
	state := func(id int) string {
		var thing tModel.Thing
		require.NoError(t, db.First(&thing, id).Error)
		return thing.State
	}

	text, isError := callTool(t, engine, alexToken, "list_things", nil)
	require.False(t, isError, text)
	require.Contains(t, text, "Dishwasher loads")
	require.NotContains(t, text, "Vacation mode", "things belong to their owner")

	text, isError = callTool(t, engine, alexToken, "get_thing", map[string]any{"id": 1})
	require.False(t, isError, text)
	require.Contains(t, text, `"state":"3"`)

	text, isError = callTool(t, engine, alexToken, "increment_thing", map[string]any{"id": 1})
	require.False(t, isError, text)
	require.JSONEq(t, `{"state":"4"}`, text)

	text, isError = callTool(t, engine, alexToken, "increment_thing", map[string]any{"id": 1, "amount": -3})
	require.False(t, isError, text)
	require.JSONEq(t, `{"state":"1"}`, text)

	text, isError = callTool(t, engine, alexToken, "set_thing_state", map[string]any{"id": 1, "state": 42})
	require.False(t, isError, text)
	require.Equal(t, "Done.", text)
	require.Equal(t, "42", state(1))

	text, isError = callTool(t, engine, patToken, "set_thing_state", map[string]any{"id": 2, "state": true})
	require.False(t, isError, text)
	require.Equal(t, "true", state(2))

	t.Run("the API validates the new state", func(t *testing.T) {
		text, isError := callTool(t, engine, alexToken, "set_thing_state", map[string]any{"id": 1, "state": "many"})
		require.True(t, isError)
		require.Contains(t, text, "Invalid state for thing")
		require.Equal(t, "42", state(1))
	})

	t.Run("another user's thing is out of reach", func(t *testing.T) {
		text, isError := callTool(t, engine, alexToken, "set_thing_state", map[string]any{"id": 2, "state": false})
		require.True(t, isError)
		require.Contains(t, text, "Unauthorized")
		require.Equal(t, "true", state(2))
	})
}

func TestToolCallErrors(t *testing.T) {
	engine, _ := newServer(t)

	// Mistakes in the arguments come back as a result the model can read and
	// act on, not as a protocol error.
	for _, test := range []struct {
		name      string
		tool      string
		arguments map[string]any
		want      string
	}{
		{"required argument missing", "delete_chore", nil, "id is required"},
		{"blank name", "create_chore", map[string]any{"name": "  "}, "name is required"},
		{"unknown argument", "list_chores", map[string]any{"archived": true}, `unknown field "archived"`},
		{"text for a number", "get_thing", map[string]any{"id": "one"}, "id must be a whole number, not a string"},
		{"fraction for a number", "get_thing", map[string]any{"id": 1.5}, "id must be a whole number, not a number 1.5"},
		{"number for text", "create_chore", map[string]any{"name": 7}, "name must be a string, not a number"},
		{"text for a flag", "list_chores", map[string]any{"includeSubtasks": "yes"}, "includeSubtasks must be true or false, not a string"},
		{"nothing to update", "update_chore", map[string]any{"id": 1}, "nothing to update"},
		{"state missing", "set_thing_state", map[string]any{"id": 1}, "state is required"},
		{"state of the wrong type", "set_thing_state", map[string]any{"id": 1, "state": []int{1}}, "state must be a string, a number or a boolean"},
		{"bad due date", "create_chore", map[string]any{"name": "Dust", "dueDate": "tomorrow"}, "Invalid due date format"},
	} {
		t.Run(test.name, func(t *testing.T) {
			text, isError := callTool(t, engine, alexToken, test.tool, test.arguments)
			require.True(t, isError, text)
			require.Contains(t, text, test.want)
		})
	}

	t.Run("arguments are not an object", func(t *testing.T) {
		r := post(t, engine, alexToken, legacy("tools/call", map[string]any{"name": "list_chores", "arguments": []int{1}}))
		require.Equal(t, true, r.result["isError"], r.body)
		require.Contains(t, r.body, "arguments must be an object")
	})

	for name, params := range map[string]map[string]any{
		"unknown tool": {"name": "format_disk"},
		"no tool name": {"arguments": map[string]any{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := post(t, engine, alexToken, legacy("tools/call", params))
			require.Equal(t, http.StatusOK, r.status, r.body)
			require.Equal(t, codeInvalidParams, r.err.Code, r.body)
		})
	}
}

func TestResultsAreNotHTMLEscaped(t *testing.T) {
	for _, test := range []struct{ name, body, want string }{
		{"ampersand", `{"name":"Wash \u0026 fold"}`, `{"name":"Wash & fold"}`},
		{"angle brackets", `"\u003cb\u003e"`, `"<b>"`},
		{"other escapes are kept", `"line\nbreak \u00e9 \u2028 \"quoted\""`, `"line\nbreak \u00e9 \u2028 \"quoted\""`},
		{"an escaped backslash is not an escape", `"C:\\u0026"`, `"C:\\u0026"`},
		{"an escaped backslash before an escape", `"\\\u0026"`, `"\\&"`},
		{"cut short", `"\u00`, `"\u00`},
		{"trailing backslash", `\u0026\`, `&\`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, string(unescapeHTML([]byte(test.body))))
		})
	}

	engine, _ := newServer(t)
	const name = `Wash & fold <delicates> at "C:\u0026"`
	text, isError := callTool(t, engine, alexToken, "create_chore", map[string]any{"name": name})
	require.False(t, isError, text)
	require.Contains(t, text, `"name":"Wash & fold <delicates> at \"C:\\u0026\""`)
	var created chModel.Chore
	require.NoError(t, json.Unmarshal([]byte(text), &created))
	require.Equal(t, name, created.Name, "the result is still the same JSON")
}

// tokens is a user lookup backed by a fixed set of API tokens.
type tokens map[string]int

func (known tokens) GetUserByToken(_ context.Context, token string) (*uModel.UserDetails, error) {
	id, ok := known[token]
	if !ok {
		return nil, errors.New("record not found")
	}
	return &uModel.UserDetails{User: uModel.User{ID: id}}, nil
}

func TestAPIRequestsCarryTheCallersIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	engine := gin.New()
	var seenToken, seenIP string
	engine.GET("/eapi/v1/things/", func(c *gin.Context) {
		seenToken, seenIP = c.GetHeader(apiTokenHeader), c.ClientIP()
		c.JSON(http.StatusOK, []string{})
	})
	engine.GET("/eapi/v1/circle/members", func(c *gin.Context) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only plus members can access this endpoint"})
	})
	engine.GET("/eapi/v1/chore", func(c *gin.Context) {
		panic("boom")
	})
	Routes(cfg, engine, newHandler(cfg, tokens{alexToken: 1}, engine), utils.NewRateLimiter(cfg))

	// The API rate limits by client address, so a request made on a client's
	// behalf has to appear to come from that client.
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
			seenToken, seenIP = "", ""
			_, isError := callToolWith(t, engine, "list_things", test.headers...)
			require.False(t, isError)
			require.Equal(t, alexToken, seenToken)
			require.Equal(t, test.wantIP, seenIP)
		})
	}

	t.Run("plan limits of the API apply", func(t *testing.T) {
		text, isError := callToolWith(t, engine, "list_circle_members")
		require.True(t, isError)
		require.Equal(t, "Donetick refused the request (HTTP 403): Only plus members can access this endpoint", text)
	})

	t.Run("a crash in the API is an internal error", func(t *testing.T) {
		r := post(t, engine, alexToken, legacy("tools/call", map[string]any{"name": "list_chores"}))
		require.Equal(t, http.StatusOK, r.status, r.body)
		require.Equal(t, codeInternalError, r.err.Code)
		require.Equal(t, "Internal error", r.err.Message, "details stay in the server log")
	})
}

func callToolWith(t *testing.T, engine *gin.Engine, name string, headers ...string) (text string, isError bool) {
	t.Helper()
	r := post(t, engine, alexToken, legacy("tools/call", map[string]any{"name": name}), headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Nil(t, r.err, r.body)
	block := r.result["content"].([]any)[0].(map[string]any)
	return block["text"].(string), r.result["isError"].(bool)
}

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	cfg.Server.RateLimit = 3
	engine := gin.New()
	Routes(cfg, engine, newHandler(cfg, tokens{alexToken: 1}, engine), utils.NewRateLimiter(cfg))

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, post(t, engine, alexToken, legacy("ping", nil)).status)
	}
	require.Equal(t, http.StatusTooManyRequests, post(t, engine, alexToken, legacy("ping", nil)).status)
	// Guessing tokens is limited too: the limiter runs before authentication.
	require.Equal(t, http.StatusTooManyRequests, post(t, engine, "guess", legacy("ping", nil)).status)
	// Other clients are unaffected.
	require.Equal(t, http.StatusOK, post(t, engine, alexToken, legacy("ping", nil), "X-Forwarded-For", "203.0.113.7").status)
}
