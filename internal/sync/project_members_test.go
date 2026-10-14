package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
	pjRepo "donetick.com/core/internal/project/repo"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fullSyncResponse struct {
	Changes struct {
		Chores         []chModel.Chore        `json:"chores"`
		ChoreHistories []chModel.ChoreHistory `json:"choreHistories"`
	} `json:"changes"`
	Deletions struct {
		Chores         []int `json:"chores"`
		ChoreHistories []int `json:"choreHistories"`
	} `json:"deletions"`
	Cursor int64 `json:"cursor"`
}

func syncAs(t *testing.T, router *gin.Engine, since int64) fullSyncResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sync/changes?since=%d", since), nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out fullSyncResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// The router of newSyncHandlerTest acts as user 2: here the colleague a work
// project is shared with. A client that already synced the project's tasks must
// drop them, and their history, once the share ends — and get both back when the
// project is shared again.
func TestSyncFollowsProjectShareAndRevocation(t *testing.T) {
	repository, db, router := newSyncHandlerTest(t)
	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	require.NoError(t, db.AutoMigrate(&cModel.Circle{}))
	for _, id := range []int{1, 2, 3} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: 1, IsActive: true}).Error)
	}
	projects := pjRepo.NewProjectRepository(db, cfg, repository)
	work := &pModel.Project{Name: "work", CircleID: 1, CreatedBy: 1, IsPrivate: true, SharedWithMembers: true, MemberIDs: []int{2}}
	require.NoError(t, projects.CreateProject(t.Context(), work))
	chore := &chModel.Chore{Name: "install", CircleID: 1, CreatedBy: 1, ProjectID: &work.ID, IsPrivate: true, IsActive: true, SyncVersion: 1}
	require.NoError(t, db.Create(chore).Error)
	now := time.Now().UTC()
	history := &chModel.ChoreHistory{ChoreID: chore.ID, CompletedBy: 2, PerformedAt: &now, Status: chModel.ChoreHistoryStatusCompleted, SyncVersion: 2}
	require.NoError(t, db.Create(history).Error)
	// The rows above were written with explicit versions; move the circle's version
	// counter past them as the repository would have.
	require.NoError(t, db.Exec("INSERT INTO sync_cursors (circle_id, entity_type, max_version) VALUES (1, 1, 2)").Error)

	first := syncAs(t, router, 0)
	require.Len(t, first.Changes.Chores, 1)
	require.Len(t, first.Changes.ChoreHistories, 1)

	members := []int{}
	vis := pModel.ProjectVisibilityMembers
	_, err := projects.UpdateProjectWithAccess(t.Context(), &pModel.Project{ID: work.ID, Name: "work"}, &pModel.ProjectReq{Visibility: &vis, MemberIDs: &members}, 1, 1)
	require.NoError(t, err)

	afterRevoke := syncAs(t, router, first.Cursor)
	require.Empty(t, afterRevoke.Changes.Chores)
	require.Equal(t, []int{chore.ID}, afterRevoke.Deletions.Chores)
	require.Equal(t, []int{history.ID}, afterRevoke.Deletions.ChoreHistories)
	// A full resync of a fresh client shows nothing either.
	fresh := syncAs(t, router, 0)
	require.Empty(t, fresh.Changes.Chores)
	require.Empty(t, fresh.Changes.ChoreHistories)

	members = []int{2}
	_, err = projects.UpdateProjectWithAccess(t.Context(), &pModel.Project{ID: work.ID, Name: "work"}, &pModel.ProjectReq{Visibility: &vis, MemberIDs: &members}, 1, 1)
	require.NoError(t, err)

	afterRegrant := syncAs(t, router, afterRevoke.Cursor)
	require.Len(t, afterRegrant.Changes.Chores, 1)
	require.Len(t, afterRegrant.Changes.ChoreHistories, 1, "history is re-sent to a client that synced past it")
	require.Empty(t, afterRegrant.Deletions.Chores)

	// A client that was offline the whole time sees no deletion, only the task.
	offline := syncAs(t, router, first.Cursor)
	require.Len(t, offline.Changes.Chores, 1)
	require.Empty(t, offline.Deletions.Chores)
	require.Empty(t, offline.Deletions.ChoreHistories)
}
