package chore

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/project/model"
	pjRepo "donetick.com/core/internal/project/repo"
)

const (
	inheritTestCircleID = 1
	inheritTestOwnerID  = 1
	inheritTestOtherID  = 2
)

func newInheritPrivacyTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	choreRepository := chRepo.NewChoreRepository(db, cfg)
	projectRepository := pjRepo.NewProjectRepository(db, cfg, choreRepository)

	return &Handler{choreRepo: choreRepository, pjRepo: projectRepository}, db
}

func createInheritTestProject(t *testing.T, db *gorm.DB, isPrivate bool) *pModel.Project {
	t.Helper()
	project := &pModel.Project{
		Name:      "test",
		CircleID:  inheritTestCircleID,
		CreatedBy: inheritTestOwnerID,
		IsPrivate: isPrivate,
	}
	require.NoError(t, db.Create(project).Error)
	return project
}

func TestInheritProjectPrivacyFollowsProjectFlagInBothDirections(t *testing.T) {
	t.Run("moving a public chore into a private project makes it private", func(t *testing.T) {
		h, db := newInheritPrivacyTestHandler(t)
		project := createInheritTestProject(t, db, true)

		isPrivate := false
		req := &ChoreReq{ID: 1, ProjectID: &project.ID, IsPrivate: &isPrivate}

		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID))
		require.NotNil(t, req.IsPrivate)
		require.True(t, *req.IsPrivate)
	})

	t.Run("moving a private chore into a public project makes it public", func(t *testing.T) {
		h, db := newInheritPrivacyTestHandler(t)
		project := createInheritTestProject(t, db, false)

		isPrivate := true
		req := &ChoreReq{ID: 1, ProjectID: &project.ID, IsPrivate: &isPrivate}

		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID))
		require.NotNil(t, req.IsPrivate)
		require.False(t, *req.IsPrivate)
	})

	t.Run("a chore outside any project keeps its own flag", func(t *testing.T) {
		h, _ := newInheritPrivacyTestHandler(t)

		isPrivate := true
		req := &ChoreReq{ID: 1, IsPrivate: &isPrivate}

		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID))
		require.NotNil(t, req.IsPrivate)
		require.True(t, *req.IsPrivate)
	})

	t.Run("moving into a private project owned by someone else is rejected", func(t *testing.T) {
		h, db := newInheritPrivacyTestHandler(t)
		project := createInheritTestProject(t, db, true)

		req := &ChoreReq{ID: 1, ProjectID: &project.ID}

		err := h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOtherID, inheritTestCircleID)
		require.Error(t, err)
	})

	t.Run("moving into a private project rejects assignees other than its owner", func(t *testing.T) {
		h, db := newInheritPrivacyTestHandler(t)
		project := createInheritTestProject(t, db, true)

		req := &ChoreReq{
			ID:        1,
			ProjectID: &project.ID,
			Assignees: []chModel.ChoreAssignees{{ChoreID: 1, UserID: inheritTestOtherID}},
		}

		err := h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID)
		require.Error(t, err)
	})
}
