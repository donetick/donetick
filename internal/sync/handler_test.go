package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	lModel "donetick.com/core/internal/label/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
	uModel "donetick.com/core/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type syncResponse struct {
	Changes struct {
		Chores []chModel.Chore `json:"chores"`
	} `json:"changes"`
	Deletions struct {
		Chores []int `json:"chores"`
	} `json:"deletions"`
	Cursor  int64 `json:"cursor"`
	HasMore bool  `json:"hasMore"`
}

func newSyncHandlerTest(t *testing.T) (*chRepo.ChoreRepository, *gorm.DB, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&chModel.Chore{},
		&chModel.ChoreAssignees{},
		&chModel.ChoreHistory{},
		&chModel.ChoreLabels{},
		&chModel.TimeSession{},
		&cModel.UserCircle{},
		&lModel.Label{},
		&nModel.Notification{},
		&syncModel.SyncCursor{},
		&syncModel.Tombstone{},
		&pModel.Project{},
		&pModel.ProjectMember{},
	))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	repository := chRepo.NewChoreRepository(db, cfg)
	handler := NewHandler(repository)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: 2, CircleID: 1}})
		c.Next()
	})
	router.GET("/sync/changes", handler.GetChanges)
	return repository, db, router
}

func getSyncChanges(t *testing.T, router *gin.Engine, since int64) syncResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync/changes?since=%d", since), nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response syncResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetChangesPaginatesScopedTombstonesWithoutSkippingBoundary(t *testing.T) {
	_, db, router := newSyncHandlerTest(t)
	tombstones := make([]syncModel.Tombstone, defaultSyncLimit+1)
	userID := 2
	for i := range tombstones {
		tombstones[i] = syncModel.Tombstone{
			CircleID:    1,
			EntityType:  syncModel.EntityTypeChore,
			EntityID:    i + 1,
			UserID:      &userID,
			SyncVersion: int64(i + 1),
		}
	}
	require.NoError(t, db.Create(&tombstones).Error)

	firstPage := getSyncChanges(t, router, 0)
	require.True(t, firstPage.HasMore)
	require.Equal(t, int64(defaultSyncLimit), firstPage.Cursor)
	require.Len(t, firstPage.Deletions.Chores, defaultSyncLimit)
	require.Equal(t, 1, firstPage.Deletions.Chores[0])
	require.Equal(t, defaultSyncLimit, firstPage.Deletions.Chores[defaultSyncLimit-1])

	secondPage := getSyncChanges(t, router, firstPage.Cursor)
	require.False(t, secondPage.HasMore)
	require.Equal(t, int64(defaultSyncLimit+1), secondPage.Cursor)
	require.Equal(t, []int{defaultSyncLimit + 1}, secondPage.Deletions.Chores)
}

// A revocation followed by a re-grant used to reach a client as both a deletion and
// a change in the same response. Clients apply deletions after upserts, so the chore
// vanished until its next edit. The re-grant now removes the user's stale tombstone,
// so a client that syncs both steps at once simply keeps the chore.
func TestGetChangesDropsRevocationSupersededByRegrant(t *testing.T) {
	repository, db, router := newSyncHandlerTest(t)
	for _, userID := range []int{1, 2} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: userID, CircleID: 1, IsActive: true}).Error)
	}
	chore := &chModel.Chore{ID: 10, Name: "private chore", CircleID: 1, CreatedBy: 1, IsPrivate: true}
	require.NoError(t, db.Create(chore).Error)
	require.NoError(t, db.Create(&chModel.ChoreAssignees{ChoreID: chore.ID, UserID: 2}).Error)

	require.NoError(t, repository.UpdateChoreVisibility(t.Context(), chore, nil))
	var revocation syncModel.Tombstone
	require.NoError(t, db.Where("entity_id = ? AND user_id = ?", chore.ID, 2).First(&revocation).Error)

	require.NoError(t, repository.UpdateChoreVisibility(t.Context(), chore, []int{2}))
	require.Greater(t, chore.SyncVersion, revocation.SyncVersion)

	var remaining int64
	require.NoError(t, db.Model(&syncModel.Tombstone{}).Where("entity_id = ? AND user_id = ?", chore.ID, 2).Count(&remaining).Error)
	require.Zero(t, remaining, "re-grant should drop the user's stale tombstone")

	response := getSyncChanges(t, router, 0)
	require.Empty(t, response.Deletions.Chores)
	require.Len(t, response.Changes.Chores, 1)
	require.Equal(t, chore.ID, response.Changes.Chores[0].ID)
	require.Equal(t, chore.SyncVersion, response.Changes.Chores[0].SyncVersion)
}
