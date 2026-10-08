package chore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	lModel "donetick.com/core/internal/label/model"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	pModel "donetick.com/core/internal/project/model"
	storageModel "donetick.com/core/internal/storage/model"
	stModel "donetick.com/core/internal/subtask/model"
	syncModel "donetick.com/core/internal/sync/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAPIPermissionTest(t *testing.T) (*gin.Engine, *gorm.DB) {
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
		&stModel.SubTask{}, &lModel.Label{}, &pModel.Project{}, &tModel.Thing{}, &tModel.ThingChore{},
		&nModel.Notification{}, &storageModel.StorageFile{}, &syncModel.SyncCursor{}, &syncModel.Tombstone{},
	))
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	users := uRepo.NewUserRepository(db, cfg)
	api := &API{choreRepo: chRepo.NewChoreRepository(db, cfg), userRepo: users,
		circleRepo: cRepo.NewCircleRepository(db), nRepo: nRepo.NewNotificationRepository(db)}
	router := gin.New()
	group := router.Group("/eapi/v1/chore", auth.APITokenMiddleware(users))
	group.PUT("/:id", auth.RequirePlusMemberMiddleware(), api.UpdateChore)
	group.DELETE("/:id", api.DeleteChore)

	for id := 1; id <= 6; id++ {
		circleID := 1
		if id == 6 {
			circleID = 2
		}
		require.NoError(t, db.Create(&uModel.User{ID: id, Username: fmt.Sprintf("user%d", id), CircleID: circleID}).Error)
		require.NoError(t, db.Create(&uModel.APIToken{UserID: id, Name: fmt.Sprintf("token%d", id), Token: fmt.Sprintf("test-token-%d", id)}).Error)
		// User 5 reproduces a valid token for an account missing circle membership.
		if id == 5 {
			continue
		}
		role := cModel.UserRoleMember
		if id == 2 || id == 6 {
			role = cModel.UserRoleAdmin
		} else if id == 3 {
			role = cModel.UserRoleManager
		}
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: circleID, Role: role, IsActive: true}).Error)
	}
	// A future stored timestamp must not fail a partial API update: this request
	// format supplies no client timestamp for optimistic concurrency checking.
	require.NoError(t, db.Create(&chModel.Chore{ID: 47, Name: "Original", CreatedBy: 1, CircleID: 1,
		UpdatedAt: time.Now().UTC().Add(time.Hour)}).Error)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", 47).Update("is_active", false).Error)
	return router, db
}

func permissionRequest(router *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("secretkey", token)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestExternalAPIUpdateAndDeletePermissions(t *testing.T) {
	for _, test := range []struct {
		name string
		id   int
		want int
	}{
		{"creator", 1, http.StatusOK},
		{"admin", 2, http.StatusOK},
		{"manager", 3, http.StatusOK},
		{"ordinary member", 4, http.StatusForbidden},
		{"token owner without membership", 5, http.StatusForbidden},
		{"admin in another circle", 6, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, db := newAPIPermissionTest(t)
			token := fmt.Sprintf("test-token-%d", test.id)
			body := `{"name":"Updated","description":"Changed","dueDate":"2026-10-09T12:00:00Z","forceUnarchive":true}`
			response := permissionRequest(router, http.MethodPut, "/eapi/v1/chore/47", token, body)
			require.Equal(t, test.want, response.Code, response.Body.String())
			var stored chModel.Chore
			require.NoError(t, db.First(&stored, 47).Error)
			if test.want == http.StatusOK {
				var result chModel.Chore
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				require.Equal(t, "Updated", result.Name)
				require.Equal(t, "Changed", *result.Description)
				require.Equal(t, "2026-10-09T12:00:00Z", result.NextDueDate.UTC().Format(time.RFC3339))
				require.True(t, result.IsActive)
				require.Equal(t, test.id, stored.UpdatedBy)
			} else {
				require.Equal(t, "Original", stored.Name)
				require.False(t, stored.IsActive)
			}
			response = permissionRequest(router, http.MethodDelete, "/eapi/v1/chore/47", token, "")
			require.Equal(t, test.want, response.Code, response.Body.String())
			var remaining int64
			require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", 47).Count(&remaining).Error)
			if test.want == http.StatusOK {
				require.Zero(t, remaining)
			} else {
				require.EqualValues(t, 1, remaining)
			}
		})
	}
}

func TestExternalAPIUpdateValidationAndPartialFields(t *testing.T) {
	router, db := newAPIPermissionTest(t)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		response := permissionRequest(router, method, "/eapi/v1/chore/47", "invalid", `{"name":"Updated"}`)
		require.Equal(t, http.StatusUnauthorized, response.Code)
	}
	response := permissionRequest(router, http.MethodPut, "/eapi/v1/chore/47", "test-token-2", `{"dueDate":"bad-date"}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	response = permissionRequest(router, http.MethodPut, "/eapi/v1/chore/47", "test-token-2", `{"dueDate":"2026-10-09"}`)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var stored chModel.Chore
	require.NoError(t, db.First(&stored, 47).Error)
	require.Equal(t, "Original", stored.Name)
	require.False(t, stored.IsActive)
	require.Equal(t, "2026-10-09", stored.NextDueDate.UTC().Format("2006-01-02"))
}

func TestExternalAPIPermissionsRespectChorePrivacy(t *testing.T) {
	router, db := newAPIPermissionTest(t)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", 47).Update("is_private", true).Error)
	for _, userID := range []int{2, 3} {
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			response := permissionRequest(router, method, "/eapi/v1/chore/47",
				fmt.Sprintf("test-token-%d", userID), `{"name":"Updated"}`)
			require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
		}
	}
	var stored chModel.Chore
	require.NoError(t, db.First(&stored, 47).Error)
	require.Equal(t, "Original", stored.Name)
	response := permissionRequest(router, http.MethodPut, "/eapi/v1/chore/47", "test-token-1", `{"name":"Updated"}`)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}
