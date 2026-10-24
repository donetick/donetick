package xp

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	circle "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	user "donetick.com/core/internal/user/model"
	xp "donetick.com/core/internal/xp/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func fixture(t *testing.T) (*Handler, *gorm.DB, *user.UserDetails) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "xp.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	for _, m := range []circle.UserCircle{{UserID: 1, CircleID: 1, Role: circle.UserRoleAdmin, IsActive: true, Points: 80, PointsRedeemed: 20}, {UserID: 2, CircleID: 1, Role: circle.UserRoleMember, IsActive: true}, {UserID: 3, CircleID: 2, Role: circle.UserRoleAdmin, IsActive: true}} {
		require.NoError(t, db.Create(&m).Error)
	}
	require.NoError(t, db.Create(&user.User{ID: 1, Username: "owner"}).Error)
	return NewHandler(db), db, &user.UserDetails{User: user.User{ID: 1, CircleID: 1}}
}
func request(h *Handler, u *user.UserDetails, method, path, body string) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if u != nil {
			c.Set("id", u)
		}
	})
	r.GET("/users/:userID/xp", h.userXP)
	r.GET("/circles/:circleID/xp", h.circleXP)
	r.GET("/xp/categories", h.categories)
	r.POST("/xp/categories", h.saveCategory)
	r.PUT("/xp/categories/:id", h.saveCategory)
	r.PUT("/xp/preference", h.preference)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}
func TestStatisticsLegacyCategoriesAndRewards(t *testing.T) {
	h, db, u := fixture(t)
	require.NoError(t, db.Create(&xp.Award{MembershipID: 1, HistoryID: 1, CircleID: 1, UserID: 1, CategoryID: 7, Points: 50}).Error)
	require.NoError(t, db.Create(&xp.Award{MembershipID: 3, HistoryID: 2, CircleID: 2, UserID: 1, CategoryID: 9, Points: 999}).Error)
	w := request(h, u, "GET", "/users/1/xp", "")
	require.Equal(t, 200, w.Code)
	var stats Statistics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	require.EqualValues(t, 80, stats.TotalXP)
	require.EqualValues(t, 50, stats.Categories["category_7"])
	require.EqualValues(t, 30, stats.Categories["uncategorized"])
	require.Len(t, stats.Categories, 2)
	w = request(h, u, "GET", "/users/2/xp", "")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"total_xp":0`)
	w = request(h, u, "GET", "/circles/1/xp", "")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"leaderboard"`)
}
func TestAuthorization(t *testing.T) {
	h, db, u := fixture(t)
	for _, path := range []string{"/xp/categories", "/users/1/xp", "/circles/1/xp"} {
		require.Equal(t, 401, request(h, nil, "GET", path, "").Code)
	}
	require.Equal(t, 403, request(h, u, "GET", "/circles/2/xp", "").Code)
	require.Equal(t, 404, request(h, u, "GET", "/users/3/xp", "").Code)
	require.Equal(t, 400, request(h, u, "GET", "/users/-1/xp", "").Code)
	member := &user.UserDetails{User: user.User{ID: 2, CircleID: 1}}
	require.Equal(t, 200, request(h, member, "GET", "/xp/categories", "").Code)
	require.Equal(t, 403, request(h, member, "POST", "/xp/categories", `{"name":"Learning"}`).Code)
	require.NoError(t, db.Model(&circle.UserCircle{}).Where("user_id = ?", 1).Update("is_active", false).Error)
	require.Equal(t, 403, request(h, u, "GET", "/users/1/xp", "").Code)
	require.Equal(t, 403, request(h, u, "POST", "/xp/categories", `{"name":"Learning"}`).Code)
}
func TestCategoryLifecycleAndPreference(t *testing.T) {
	h, db, u := fixture(t)
	require.Equal(t, 200, request(h, u, "POST", "/xp/categories", `{"name":" Learning "}`).Code)
	require.Equal(t, 409, request(h, u, "POST", "/xp/categories", `{"name":"learning"}`).Code)
	require.Equal(t, 400, request(h, u, "POST", "/xp/categories", `{"name":"  "}`).Code)
	require.Equal(t, 400, request(h, u, "POST", "/xp/categories", `{"name":"`+strings.Repeat("a", 81)+`"}`).Code)
	require.Equal(t, 200, request(h, u, "PUT", "/xp/categories/1", `{"name":"Study","archived":true}`).Code)
	var category xp.Category
	require.NoError(t, db.First(&category).Error)
	require.True(t, category.Archived)
	require.Equal(t, "Study", category.Name)
	outsider := &user.UserDetails{User: user.User{ID: 3, CircleID: 2}}
	require.Equal(t, 404, request(h, outsider, "PUT", "/xp/categories/1", `{"name":"Stolen"}`).Code)
	require.Equal(t, 200, request(h, u, "PUT", "/xp/preference", `{"xpMode":true}`).Code)
	var stored user.User
	require.NoError(t, db.First(&stored, 1).Error)
	require.True(t, stored.XPMode)
	require.Equal(t, 400, request(h, u, "PUT", "/xp/preference", `{}`).Code)
	// Additive migration is safe to repeat and never recategorizes legacy points.
	require.NoError(t, database.Migration(db))
	var m circle.UserCircle
	require.NoError(t, db.Where("user_id = ? AND circle_id = ?", 1, 1).First(&m).Error)
	require.Equal(t, 80, m.Points)
}

func TestXPRejoinedMembershipDoesNotInheritOldAwards(t *testing.T) {
	h, db, u := fixture(t)
	require.NoError(t, db.Create(&xp.Award{MembershipID: 1, HistoryID: 1, CircleID: 1, UserID: 1, CategoryID: 7, Points: 50}).Error)
	require.NoError(t, db.Where("user_id = ? AND circle_id = ?", 1, 1).Delete(&circle.UserCircle{}).Error)
	require.NoError(t, db.Create(&circle.UserCircle{UserID: 1, CircleID: 1, IsActive: true, Role: circle.UserRoleMember}).Error)
	w := request(h, u, "GET", "/users/1/xp", "")
	require.Equal(t, 200, w.Code)
	var stats Statistics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	require.Zero(t, stats.TotalXP)
	require.Len(t, stats.Categories, 1)
	require.Zero(t, stats.Categories["uncategorized"])
}

func TestManagerCanManageButDisabledUserCannot(t *testing.T) {
	h, db, u := fixture(t)
	require.NoError(t, db.Model(&circle.UserCircle{}).Where("user_id = ?", 1).Update("role", circle.UserRoleManager).Error)
	require.Equal(t, 200, request(h, u, "POST", "/xp/categories", `{"name":"Manager category"}`).Code)
	u.Disabled = true
	require.Equal(t, 401, request(h, u, "GET", "/users/1/xp", "").Code)
	require.Equal(t, 401, request(h, u, "PUT", "/xp/preference", `{"xpMode":true}`).Code)
}
