package repo

import (
	"context"
	"path/filepath"
	"testing"

	"donetick.com/core/config"
	fModel "donetick.com/core/internal/filter/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFilterPrivacy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "filters.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&fModel.Filter{}))
	// The existing by-usage query expects this column.
	require.NoError(t, db.Exec("ALTER TABLE filters ADD COLUMN usage_count INTEGER NOT NULL DEFAULT 0").Error)
	repo := NewFilterRepository(db, &config.Config{})
	ctx := context.Background()
	private := &fModel.Filter{Name: "personal", CircleID: 1, CreatedBy: 1, IsPinned: true}
	shared := &fModel.Filter{Name: "shared", CircleID: 1, CreatedBy: 1, IsShared: true, IsPinned: true}
	elsewhere := &fModel.Filter{Name: "elsewhere", CircleID: 2, CreatedBy: 1}
	conditions := fModel.FilterConditions{{Type: "status", Operator: "is", Value: "pending"}}
	for _, filter := range []*fModel.Filter{private, shared, elsewhere} {
		filter.Conditions = conditions
		require.NoError(t, repo.CreateFilter(ctx, filter))
	}
	for _, list := range []func(context.Context, int, int) ([]*fModel.Filter, error){repo.GetCircleFilters, repo.GetPinnedFilters, repo.GetFiltersByUsage} {
		owner, err := list(ctx, 1, 1)
		require.NoError(t, err)
		require.Len(t, owner, 2)
		other, err := list(ctx, 1, 2)
		require.NoError(t, err)
		require.Len(t, other, 1)
		require.Equal(t, shared.ID, other[0].ID)
	}
	_, err = repo.GetFilterByID(ctx, private.ID, 1, 2)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = repo.GetFilterByID(ctx, private.ID, 2, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	exists, err := repo.FilterNameExists(ctx, "PERSONAL", 1, 2, nil)
	require.NoError(t, err)
	require.False(t, exists, "private names should not leak through duplicate checks")
	exists, err = repo.FilterNameExists(ctx, "PERSONAL", 1, 1, nil)
	require.NoError(t, err)
	require.True(t, exists)
	require.Error(t, repo.UpdateFilter(ctx, private, 2, 1, nil))
	require.Error(t, repo.DeleteFilter(ctx, private.ID, 2, 1))
	_, err = repo.ToggleFilterPin(ctx, private.ID, 2, 1)
	require.Error(t, err)

	update := &fModel.Filter{ID: private.ID, Name: "renamed", Operator: fModel.LogicalOperatorAND, Conditions: conditions}
	require.NoError(t, repo.UpdateFilter(ctx, update, 1, 1, nil))
	saved, err := repo.GetFilterByID(ctx, private.ID, 1, 1)
	require.NoError(t, err)
	require.False(t, saved.IsShared)
	public := true
	require.NoError(t, repo.UpdateFilter(ctx, update, 1, 1, &public))
	saved, err = repo.GetFilterByID(ctx, private.ID, 1, 2)
	require.NoError(t, err)
	require.True(t, saved.IsShared)
}
