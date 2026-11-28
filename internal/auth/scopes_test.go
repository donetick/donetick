package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"donetick.com/core/config"
	cModel "donetick.com/core/internal/circle/model"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAPITokenScopeCatalog(t *testing.T) {
	require.False(t, ValidAPITokenScopes(nil))
	require.False(t, ValidAPITokenScopes([]string{"*"}))
	require.False(t, ValidAPITokenScopes([]string{"chores:read", "chores:read"}))
	require.True(t, ValidAPITokenScopes(APITokenScopes()))
	require.False(t, tokenHasScope([]string{"chores:write"}, "chores:execute"))
	require.Empty(t, requiredTokenScope("POST", "/api/v1/users/tokens"))
	require.Empty(t, requiredTokenScope("PUT", "/api/v1/users/tokens/:id"))
	require.Empty(t, requiredTokenScope("GET", "/api/v1/new-feature"))
}

func TestScopedTokenEnforcementAndLegacyMigration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.APIToken{}, &cModel.Circle{}))
	require.NoError(t, db.Create(&uModel.User{ID: 1, Username: "owner", CircleID: 1}).Error)
	require.NoError(t, db.Create(&uModel.User{ID: 2, Username: "other", CircleID: 1}).Error)
	users := uRepo.NewUserRepository(db, &config.Config{})
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	token := uModel.APIToken{UserID: 1, Name: "legacy", Token: "legacy-secret"}
	require.NoError(t, db.Create(&token).Error)
	router := gin.New()
	group := router.Group("/api/v1", NewMultiAuthMiddleware(nil, users).MiddlewareFunc())
	for _, route := range []struct{ method, path string }{
		{"GET", "/chores/"}, {"POST", "/chores/:id/do"}, {"POST", "/users/tokens"}, {"PUT", "/users/tokens/:id"}, {"GET", "/unknown"},
	} {
		group.Handle(route.method, route.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	}
	request := func(method, path, secret string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("secretkey", secret)
		// Invalid explicitly supplied API credentials never fall back to JWT.
		req.Header.Set("Authorization", "Bearer anything")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	require.Equal(t, 403, request("GET", "/api/v1/chores/", "legacy-secret"))
	require.Error(t, users.UpdateAPIToken(t.Context(), 2, fmt.Sprint(token.ID), uRepo.APITokenOptions{Scopes: []string{"chores:read"}, ExpiresAt: &future}))
	require.NoError(t, users.UpdateAPIToken(t.Context(), 1, fmt.Sprint(token.ID), uRepo.APITokenOptions{Scopes: []string{"chores:read"}, ExpiresAt: &future}))
	require.Equal(t, 204, request("GET", "/api/v1/chores/", "legacy-secret"))
	require.Equal(t, 403, request("POST", "/api/v1/chores/1/do", "legacy-secret"))
	require.Equal(t, 403, request("POST", "/api/v1/users/tokens", "legacy-secret"))
	require.Equal(t, 403, request("PUT", "/api/v1/users/tokens/1", "legacy-secret"))
	require.Equal(t, 403, request("GET", "/api/v1/unknown", "legacy-secret"))
	require.Equal(t, 401, request("GET", "/api/v1/chores/", "wrong"))
	stored, err := users.GetAPIToken(t.Context(), "legacy-secret")
	require.NoError(t, err)
	require.NotNil(t, stored.LastUsedAt)
	otherCircle := 2
	require.NoError(t, users.UpdateAPIToken(t.Context(), 1, fmt.Sprint(token.ID), uRepo.APITokenOptions{Scopes: []string{"chores:read"}, ExpiresAt: &future, CircleID: &otherCircle}))
	require.Equal(t, 403, request("GET", "/api/v1/chores/", "legacy-secret"))
	require.NoError(t, users.UpdateAPIToken(t.Context(), 1, fmt.Sprint(token.ID), uRepo.APITokenOptions{Scopes: []string{"chores:read"}, ExpiresAt: &past}))
	require.Equal(t, 401, request("GET", "/api/v1/chores/", "legacy-secret"))
	require.NoError(t, users.DeleteAPIToken(t.Context(), 1, fmt.Sprint(token.ID)))
	require.Error(t, users.UpdateAPIToken(t.Context(), 1, fmt.Sprint(token.ID), uRepo.APITokenOptions{Scopes: []string{"chores:read"}, ExpiresAt: &future}))
	require.Equal(t, 401, request("GET", "/api/v1/chores/", "legacy-secret"))
}
