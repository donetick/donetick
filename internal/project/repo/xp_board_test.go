package repo

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	xpModel "donetick.com/core/internal/points"
	xpRepo "donetick.com/core/internal/points/repo"
	"github.com/stretchr/testify/require"
)

func TestBoardMoveRejectsStaleXPCompletionAndPreservesUndoSnapshot(t *testing.T) {
	r, db := newTestRepo(t)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	ctx := context.Background()
	require.NoError(t, db.Create(&cModel.Circle{ID: testCircleID, XPMode: true}).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: testOwnerID, CircleID: testCircleID, IsActive: true}).Error)
	category := xpModel.XPCategory{CircleID: testCircleID, Name: "Home", NameKey: "home"}
	require.NoError(t, db.Create(&category).Error)
	project := createTestProject(t, db, "Board", testOwnerID, false)
	first, err := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Todo")
	require.NoError(t, err)
	second, err := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Doing")
	require.NoError(t, err)
	points := 30
	chore := &chModel.Chore{
		Name: "Categorized card", CircleID: testCircleID, CreatedBy: testOwnerID, IsActive: true,
		ProjectID: &project.ID, SectionID: &first.ID, Points: &points, XPCategoryID: &category.ID,
		FrequencyType: chModel.FrequencyTypeAlways, AssignStrategy: chModel.AssignmentStrategyRandom,
	}
	_, err = r.choreRepo.CreateChoreWithAssignees(ctx, chore, []int{testOwnerID}, testOwnerID)
	require.NoError(t, err)
	stale := *reloadChore(t, db, chore.ID)
	require.NoError(t, r.MoveChore(ctx, project.ID, chore.ID, second.ID, nil, testCircleID, testOwnerID))
	moved := reloadChore(t, db, chore.ID)
	require.Equal(t, second.ID, *moved.SectionID)
	require.Equal(t, category.ID, *moved.XPCategoryID)
	require.Greater(t, moved.SyncVersion, stale.SyncVersion)
	now := time.Now().UTC()
	require.ErrorIs(t, r.choreRepo.CompleteChore(ctx, &stale, nil, testOwnerID, nil, &now, nil, true), chRepo.ErrChoreChanged)
	stats, err := xpRepo.Stats(db, testCircleID, testOwnerID)
	require.NoError(t, err)
	require.Zero(t, stats.TotalXP)
	require.NoError(t, r.choreRepo.CompleteChore(ctx, moved, nil, testOwnerID, nil, &now, nil, true))
	require.ErrorIs(t, r.choreRepo.CompleteChore(ctx, &stale, nil, testOwnerID, nil, &now, nil, true), chRepo.ErrChoreChanged)
	stats, err = xpRepo.Stats(db, testCircleID, testOwnerID)
	require.NoError(t, err)
	require.EqualValues(t, 30, stats.TotalXP)
	var history chModel.ChoreHistory
	require.NoError(t, db.Where("chore_id = ?", chore.ID).First(&history).Error)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(map[string]any{"points": 90, "xp_category_id": nil}).Error)
	require.NoError(t, r.choreRepo.UndoChoreAction(ctx, chore.ID, history.ID, testOwnerID, nil, nil))
	stats, err = xpRepo.Stats(db, testCircleID, testOwnerID)
	require.NoError(t, err)
	require.Zero(t, stats.TotalXP)
	require.Zero(t, stats.Categories["1"], "undo must reverse the original category, not the edited task")
	require.Equal(t, second.ID, *reloadChore(t, db, chore.ID).SectionID)
}
