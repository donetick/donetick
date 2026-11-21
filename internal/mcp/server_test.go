package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/events"
	"donetick.com/core/internal/filter"
	fRepo "donetick.com/core/internal/filter/repo"
	"donetick.com/core/internal/label"
	lRepo "donetick.com/core/internal/label/repo"
	"donetick.com/core/internal/notifier"
	"donetick.com/core/internal/notifier/dispatch"
	"donetick.com/core/internal/notifier/push"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/project"
	pjRepo "donetick.com/core/internal/project/repo"
	"donetick.com/core/internal/storage"
	storageRepo "donetick.com/core/internal/storage/repo"
	stRepo "donetick.com/core/internal/subtask/repo"
	"donetick.com/core/internal/thing"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The users every test server starts with.
const (
	alex, alexToken = 1, "token-alex" // admin of circle 1
	sam, samToken   = 2, "token-sam"  // member of circle 1
	pat, patToken   = 3, "token-pat"  // admin of circle 2
)

func testConfig(t *testing.T) *config.Config {
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.MCP.Enabled = true
	cfg.Server.RateLimit = 1000
	cfg.Server.RatePeriod = time.Minute
	cfg.Server.WriteTimeout = 5 * time.Second
	cfg.WebhookConfig.QueueSize = 10
	cfg.Storage.BasePath = t.TempDir()
	return cfg
}

// newServer returns an engine serving /mcp in front of the real API routes
// and a fresh database. The tool tests run against it, so a tool that has
// drifted from the route it wraps fails them.
func newServer(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// A file rather than ":memory:": the API runs transactions and background
	// work side by side, which needs more than one connection to the database.
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "donetick.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, database.Migration(db))

	cfg := testConfig(t)
	users := uRepo.NewUserRepository(db, cfg)
	circles := cRepo.NewCircleRepository(db)
	chores := chRepo.NewChoreRepository(db, cfg)
	things := tRepo.NewThingRepository(db, cfg)
	labels := lRepo.NewLabelRepository(db, cfg)
	projects := pjRepo.NewProjectRepository(db, cfg, chores)
	devices := dRepo.NewDeviceRepository(db, cfg)
	notifications := nRepo.NewNotificationRepository(db)
	planner := nps.NewNotificationPlanner(notifications, circles, users, devices)
	producer := events.NewEventsProducer(cfg)
	producer.Start(context.Background())
	sender := push.NewSender(devices, notifier.NewNotifier(nil, nil, producer, nil, nil, nil))

	engine := gin.New()
	// Tool calls reach the API with an API token, so the JWT half of its
	// authentication is never consulted.
	apiAuth := auth.NewMultiAuthMiddleware(nil, users)
	chore.Routes(engine, chore.NewHandler(chores, circles, nil, planner, notifications, things, labels,
		producer, stRepo.NewSubTasksRepository(db), storage.NewLocalStorage(cfg), users, devices, sender,
		storageRepo.NewStorageRepository(db, cfg), storage.NewURLSignerLocal(cfg), nil,
		dispatch.NewDispatcher(notifications, users, devices, sender), projects), apiAuth)
	label.Routes(engine, label.NewHandler(labels), apiAuth)
	project.Routes(engine, project.NewHandler(projects, chores, planner, nil), apiAuth)
	filter.Routes(engine, filter.NewHandler(fRepo.NewFilterRepository(db, cfg)), apiAuth)
	thing.Routes(engine, thing.NewHandler(chores, circles, planner, notifications, things, producer), apiAuth)
	Routes(cfg, engine, newHandler(cfg, users, engine), utils.NewRateLimiter(cfg))

	for _, u := range []struct {
		id, circle  int
		name, token string
		role        cModel.UserRole
	}{
		{alex, 1, "alex", alexToken, cModel.UserRoleAdmin},
		{sam, 1, "sam", samToken, cModel.UserRoleMember},
		{pat, 2, "pat", patToken, cModel.UserRoleAdmin},
	} {
		require.NoError(t, db.Create(&uModel.User{ID: u.id, Username: u.name, DisplayName: u.name, CircleID: u.circle}).Error)
		require.NoError(t, db.Create(&uModel.APIToken{UserID: u.id, Name: u.name, Token: u.token}).Error)
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: u.id, CircleID: u.circle, Role: u.role, IsActive: true}).Error)
	}
	return engine, db
}

// tokens is a user lookup backed by a fixed set of API tokens, for tests that
// put /mcp in front of stand-in API routes.
type tokens map[string]int

func (known tokens) GetUserByToken(_ context.Context, token string) (*uModel.UserDetails, error) {
	id, ok := known[token]
	if !ok {
		return nil, errors.New("record not found")
	}
	return &uModel.UserDetails{User: uModel.User{ID: id}}, nil
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

// callTool calls a tool and returns the text of its result. Extra headers are
// given as name, value pairs.
func callTool(t *testing.T, engine *gin.Engine, token, name string, arguments map[string]any, headers ...string) (text string, isError bool) {
	t.Helper()
	r := post(t, engine, token, legacy("tools/call", map[string]any{"name": name, "arguments": arguments}), headers...)
	require.Equal(t, http.StatusOK, r.status, r.body)
	require.Nil(t, r.err, r.body)
	content := r.result["content"].([]any)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	require.Equal(t, "text", block["type"])
	return block["text"].(string), r.result["isError"].(bool)
}

// mustCall calls a tool that is expected to succeed and decodes what it
// returned, taking it out of the API's "res" envelope.
func mustCall[T any](t *testing.T, engine *gin.Engine, token, name string, arguments map[string]any) T {
	t.Helper()
	text, isError := callTool(t, engine, token, name, arguments)
	require.False(t, isError, text)
	var result T
	require.NoError(t, json.Unmarshal(unwrap([]byte(text)), &result), text)
	return result
}

// mustSucceed calls a tool for its effect.
func mustSucceed(t *testing.T, engine *gin.Engine, token, name string, arguments map[string]any) {
	t.Helper()
	text, isError := callTool(t, engine, token, name, arguments)
	require.False(t, isError, text)
}

// mustFail calls a tool that is expected to be turned down and returns the
// explanation given to the model.
func mustFail(t *testing.T, engine *gin.Engine, token, name string, arguments map[string]any) string {
	t.Helper()
	text, isError := callTool(t, engine, token, name, arguments)
	require.True(t, isError, "%s unexpectedly succeeded: %s", name, text)
	return text
}
