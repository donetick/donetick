package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/points"
	pRepo "donetick.com/core/internal/points/repo"
	uModel "donetick.com/core/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func xpRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, database.Migration(db))
	require.NoError(t, db.Create(&[]cModel.Circle{{ID: 1}, {ID: 2}}).Error)
	require.NoError(t, db.Create(&[]cModel.UserCircle{
		{UserID: 1, CircleID: 1, Role: cModel.UserRoleAdmin, IsActive: true},
		{UserID: 2, CircleID: 1, Role: cModel.UserRoleMember, IsActive: true, Points: 11},
		{UserID: 3, CircleID: 1, Role: cModel.UserRoleManager, IsActive: true},
		{UserID: 4, CircleID: 2, Role: cModel.UserRoleAdmin, IsActive: true, Points: 99},
	}).Error)
	require.NoError(t, db.Create(&[]pModel.XPCategory{
		{CircleID: 1, Name: "Home", NameKey: "home"},
		{CircleID: 2, Name: "Private", NameKey: "private"},
	}).Error)
	h := NewHandler(db, pRepo.NewPointsRepository(db))
	r := gin.New()
	// Test authentication context only; authorization must still read live DB.
	r.Use(func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("X-Test-User"))
		if id > 0 {
			c.Set("id", &uModel.UserDetails{User: uModel.User{ID: id, CircleID: 1}})
		}
	})
	r.GET("/xp/settings", h.settings)
	r.PUT("/xp/settings", h.updateSettings)
	r.GET("/xp/categories", h.categories)
	r.POST("/xp/categories", h.saveCategory)
	r.PUT("/xp/categories/:id", h.saveCategory)
	r.GET("/users/:id/xp", h.userXP)
	r.POST("/users/:id/xp/adjust", h.adjust)
	r.GET("/circles/:id/xp", h.circleXP)
	return r, db
}

func xpCall(r *gin.Engine, user int, method, path, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", strconv.Itoa(user))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	return res
}

func TestXPAPIScopesLiveMembershipAndRoles(t *testing.T) {
	r, db := xpRouter(t)
	for _, item := range []struct {
		user               int
		method, path, body string
		status             int
	}{
		{0, "GET", "/xp/settings", "", 401},
		{4, "GET", "/users/2/xp", "", 403},
		{2, "GET", "/users/4/xp", "", 404},
		{2, "GET", "/circles/2/xp", "", 403},
		{2, "PUT", "/xp/settings", `{"xpMode":true}`, 403},
		{3, "PUT", "/xp/settings", `{"xpMode":true}`, 200},
		{3, "POST", "/users/2/xp/adjust", `{"action":"add","points":5,"categoryId":1}`, 403},
		{2, "POST", "/xp/categories", `{"name":"Other"}`, 403},
		{1, "PUT", "/xp/categories/2", `{"name":"Stolen"}`, 404},
	} {
		res := xpCall(r, item.user, item.method, item.path, item.body, "key")
		require.Equal(t, item.status, res.Code, "%s %s: %s", item.method, item.path, res.Body.String())
	}
	res := xpCall(r, 2, "GET", "/xp/categories", "", "")
	require.Equal(t, 200, res.Code)
	require.NotContains(t, res.Body.String(), "Private")
	require.NoError(t, db.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", 1, 1).Update("role", cModel.UserRoleMember).Error)
	require.Equal(t, 403, xpCall(r, 1, "PUT", "/xp/settings", `{"xpMode":false}`, "").Code, "an old admin token cannot retain revoked rights")
	require.NoError(t, db.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", 2, 1).Update("is_active", false).Error)
	require.Equal(t, 403, xpCall(r, 2, "GET", "/xp/categories", "", "").Code)
}

func TestXPAPIManualAdjustmentsAreCategorizedAtomicAndIdempotent(t *testing.T) {
	r, db := xpRouter(t)
	body := `{"action":"add","points":5,"categoryId":1}`
	for i := 0; i < 2; i++ {
		res := xpCall(r, 1, "POST", "/users/2/xp/adjust", body, "retry-safe")
		require.Equal(t, 200, res.Code, res.Body.String())
	}
	require.Equal(t, 409, xpCall(r, 1, "POST", "/users/2/xp/adjust", `{"action":"add","points":6,"categoryId":1}`, "retry-safe").Code)
	res := xpCall(r, 1, "POST", "/users/2/xp/adjust", `{"action":"remove","points":3,"categoryId":1}`, "remove")
	require.Equal(t, 200, res.Code, res.Body.String())
	require.Equal(t, 400, xpCall(r, 1, "POST", "/users/2/xp/adjust", `{"action":"remove","points":3,"categoryId":1}`, "overdraw").Code)
	stats, err := pRepo.Stats(db, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 13, stats.TotalXP)
	require.EqualValues(t, 2, stats.Categories["1"])
	require.EqualValues(t, 11, stats.Categories["0"])
	var count int64
	require.NoError(t, db.Model(&pModel.PointsHistory{}).Count(&count).Error)
	require.EqualValues(t, 2, count, "retry/conflict/failed spend must not add history")
	for _, invalid := range []string{
		`{"action":"add","points":1}`, `{"action":"add","points":1,"categoryId":0}`,
		`{"action":"add","points":1,"categoryId":2}`, `{"action":"add","points":1.5,"categoryId":1}`,
		`{"action":"add","points":0,"categoryId":1}`, `{"action":"remove","points":-1,"categoryId":1}`,
		`{"action":"add","points":1000000001,"categoryId":1}`, `{"action":"oops","points":1,"categoryId":1}`,
	} {
		require.Equal(t, 400, xpCall(r, 1, "POST", "/users/2/xp/adjust", invalid, invalid).Code, invalid)
	}
	require.Equal(t, 400, xpCall(r, 1, "POST", "/users/2/xp/adjust", body, "").Code)
}

func TestXPAPISettingsCategoriesAndStatsPersist(t *testing.T) {
	r, db := xpRouter(t)
	for _, mode := range []string{"true", "false"} {
		require.Equal(t, http.StatusOK, xpCall(r, 1, "PUT", "/xp/settings", `{"xpMode":`+mode+`}`, "").Code)
		var circle cModel.Circle
		require.NoError(t, db.First(&circle, 1).Error)
		require.Equal(t, mode == "true", circle.XPMode)
	}
	require.Equal(t, 409, xpCall(r, 1, "POST", "/xp/categories", `{"name":"  HOME  "}`, "").Code)
	require.Equal(t, 400, xpCall(r, 1, "POST", "/xp/categories", `{"name":"Uncategorized"}`, "").Code)
	require.Equal(t, 200, xpCall(r, 1, "PUT", "/xp/categories/1", `{"name":"Household","archived":true}`, "").Code)
	require.Equal(t, 400, xpCall(r, 1, "POST", "/users/2/xp/adjust", `{"action":"add","points":5,"categoryId":1}`, "archived").Code)
	res := xpCall(r, 2, "GET", "/users/2/xp", "", "")
	require.Equal(t, 200, res.Code)
	var stats pModel.XPStats
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &stats))
	require.EqualValues(t, 11, stats.TotalXP)
	require.EqualValues(t, 11, stats.Categories["0"])
	res = xpCall(r, 2, "GET", "/circles/1/xp", "", "")
	require.Equal(t, 200, res.Code)
	var leaderboard struct {
		Users []pModel.XPStats `json:"users"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &leaderboard))
	require.Len(t, leaderboard.Users, 3)
	require.Equal(t, 2, leaderboard.Users[0].UserID)
}
