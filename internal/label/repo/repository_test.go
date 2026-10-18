package chore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	"donetick.com/core/internal/database"
	lModel "donetick.com/core/internal/label/model"
)

// A label shown on a chore can be removed by any member allowed to edit it,
// not only by the member who attached it.
func TestAssignLabelsToChoreRemovesLabelAddedByAnotherMember(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "labels.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	repo := NewLabelRepository(db, &config.Config{})
	ctx := context.Background()

	circleID := 1
	label := lModel.Label{Name: "kitchen", Color: "#ff9800", CircleID: &circleID, CreatedBy: 2}
	require.NoError(t, db.Create(&label).Error)
	chore := chModel.Chore{Name: "dishes", FrequencyType: chModel.FrequencyTypeOnce,
		AssignStrategy: chModel.AssignmentStrategyRandom, CircleID: circleID, CreatedBy: 2, IsActive: true}
	require.NoError(t, db.Create(&chore).Error)
	require.NoError(t, repo.AssignLabelsToChore(ctx, chore.ID, 2, circleID, []int{label.ID}, nil))

	require.NoError(t, repo.AssignLabelsToChore(ctx, chore.ID, 1, circleID, nil, []int{label.ID}))

	var count int64
	require.NoError(t, db.Model(&chModel.ChoreLabels{}).Where("chore_id = ?", chore.ID).Count(&count).Error)
	require.Zero(t, count, "the label is still attached to the chore")
}
