package chore

import (
	"context"
	"path/filepath"
	"testing"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	lModel "donetick.com/core/internal/label/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLabelPrivacy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "privacy.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	ctx := context.Background()
	repo := NewLabelRepository(db, &config.Config{})
	chores := chRepo.NewChoreRepository(db, &config.Config{})
	circleID := 1
	label := lModel.Label{Name: "personal", CreatedBy: 1, CircleID: &circleID}
	require.NoError(t, repo.CreateLabels(ctx, []*lModel.Label{&label}))
	chore := chModel.Chore{Name: "shared task", CircleID: circleID, CreatedBy: 1, IsActive: true}
	require.NoError(t, db.Create(&chore).Error)
	require.NoError(t, repo.AssignLabelsToChore(ctx, chore.ID, 1, circleID, []int{label.ID}, nil))
	require.Error(t, repo.AssignLabelsToChore(ctx, chore.ID, 2, circleID, []int{label.ID}, nil))
	require.Error(t, repo.AssignLabelsToChore(ctx, chore.ID, 2, circleID, nil, []int{label.ID}))

	ownerLabels, err := repo.GetUserLabels(ctx, 1, circleID)
	require.NoError(t, err)
	require.Len(t, ownerLabels, 1)
	otherLabels, err := repo.GetUserLabels(ctx, 2, circleID)
	require.NoError(t, err)
	require.Empty(t, otherLabels)

	ownerChore, err := chores.GetChore(ctx, chore.ID, 1, circleID)
	require.NoError(t, err)
	require.Len(t, *ownerChore.LabelsV2, 1)
	otherChore, err := chores.GetChore(ctx, chore.ID, 2, circleID)
	require.NoError(t, err)
	require.Empty(t, *otherChore.LabelsV2)
	list, err := chores.GetChores(ctx, circleID, 2, false, nil, false)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Empty(t, *list[0].LabelsV2)

	update := lModel.Label{ID: label.ID, Name: "renamed"}
	require.NoError(t, repo.UpdateLabel(ctx, 1, &update, nil))
	require.False(t, update.IsShared, "omitting sharing must keep a label private")
	shared := true
	require.NoError(t, repo.UpdateLabel(ctx, 1, &update, &shared))
	require.True(t, update.IsShared)
	otherChore, err = chores.GetChore(ctx, chore.ID, 2, circleID)
	require.NoError(t, err)
	require.Len(t, *otherChore.LabelsV2, 1)
	require.Greater(t, otherChore.SyncVersion, ownerChore.SyncVersion)
	require.Error(t, repo.UpdateLabel(ctx, 2, &update, &shared))

	private := false
	require.NoError(t, repo.UpdateLabel(ctx, 1, &update, &private))
	require.NoError(t, db.Model(&chore).Update("is_active", false).Error)
	archived, err := chores.GetArchivedChores(ctx, circleID, 2)
	require.NoError(t, err)
	require.Len(t, archived, 1)
	require.Empty(t, *archived[0].LabelsV2)
}
