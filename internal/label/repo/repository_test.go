package chore

import (
	"context"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	lModel "donetick.com/core/internal/label/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLabelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&lModel.Label{}, &chModel.ChoreLabels{}))
	return db
}

func TestAssignLabelsToChoreRequiresSharedCircle(t *testing.T) {
	db := setupLabelTestDB(t)
	repo := NewLabelRepository(db, nil)

	label := &lModel.Label{
		Name:      "Laundry",
		Color:     "#fff",
		CreatedBy: 1,
	}
	require.NoError(t, db.Create(label).Error)

	err := repo.AssignLabelsToChore(context.Background(), 99, 2, 123, []int{label.ID}, nil)
	require.Error(t, err, "assigning a label without circle visibility should fail")

	circleID := 123
	require.NoError(t, db.Model(label).Update("circle_id", circleID).Error)

	err = repo.AssignLabelsToChore(context.Background(), 99, 2, 123, []int{label.ID}, nil)
	require.NoError(t, err)

	var associations []chModel.ChoreLabels
	require.NoError(t, db.Find(&associations).Error)
	require.Len(t, associations, 1)
	require.Equal(t, 2, associations[0].UserID)
	require.Equal(t, label.ID, associations[0].LabelID)
	require.Equal(t, 99, associations[0].ChoreID)
}
