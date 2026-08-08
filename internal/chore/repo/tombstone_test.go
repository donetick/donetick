package chore

import (
	"context"
	"fmt"
	"testing"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	syncModel "donetick.com/core/internal/sync/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newVisibilityTestRepository(t *testing.T) (*ChoreRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&chModel.Chore{},
		&chModel.ChoreAssignees{},
		&cModel.UserCircle{},
		&syncModel.SyncCursor{},
		&syncModel.Tombstone{},
	))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return NewChoreRepository(db, cfg), db
}

func seedVisibilityTest(t *testing.T, db *gorm.DB, chore *chModel.Chore, assigneeIDs ...int) {
	t.Helper()
	for _, userID := range []int{1, 2, 3, 4} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: userID, CircleID: chore.CircleID, IsActive: true}).Error)
	}
	require.NoError(t, db.Create(chore).Error)
	for _, userID := range assigneeIDs {
		require.NoError(t, db.Create(&chModel.ChoreAssignees{ChoreID: chore.ID, UserID: userID}).Error)
	}
}

func TestGetTombstonesSinceFiltersUserScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&syncModel.Tombstone{}))

	userID := 7
	otherUserID := 8
	require.NoError(t, db.Create([]syncModel.Tombstone{
		{CircleID: 1, EntityType: syncModel.EntityTypeChore, EntityID: 10, SyncVersion: 1},
		{CircleID: 1, EntityType: syncModel.EntityTypeChore, EntityID: 11, UserID: &userID, SyncVersion: 2},
		{CircleID: 1, EntityType: syncModel.EntityTypeChore, EntityID: 12, UserID: &otherUserID, SyncVersion: 3},
	}).Error)

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	repository := NewChoreRepository(db, cfg)
	tombstones, err := repository.GetTombstonesSince(context.Background(), 1, syncModel.EntityTypeChore, userID, 0, 0)
	require.NoError(t, err)
	require.Equal(t, []int{10, 11}, []int{tombstones[0].EntityID, tombstones[1].EntityID})

	otherUserTombstones, err := repository.GetTombstonesSince(context.Background(), 1, syncModel.EntityTypeChore, otherUserID, 0, 0)
	require.NoError(t, err)
	require.Equal(t, []int{10, 12}, []int{otherUserTombstones[0].EntityID, otherUserTombstones[1].EntityID})

	firstPage, err := repository.GetTombstonesSince(context.Background(), 1, syncModel.EntityTypeChore, userID, 0, 1)
	require.NoError(t, err)
	require.Equal(t, []int{10}, []int{firstPage[0].EntityID})
	secondPage, err := repository.GetTombstonesSince(context.Background(), 1, syncModel.EntityTypeChore, userID, firstPage[0].SyncVersion, 1)
	require.NoError(t, err)
	require.Equal(t, []int{11}, []int{secondPage[0].EntityID})
}

func TestUpdateChoreVisibilityPrivateAssigneeChanges(t *testing.T) {
	t.Run("removed assignee is revoked", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true}
		seedVisibilityTest(t, db, chore, 2, 3)

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, []int{3}))

		var tombstones []syncModel.Tombstone
		require.NoError(t, db.Find(&tombstones).Error)
		require.Len(t, tombstones, 1)
		require.Equal(t, 2, *tombstones[0].UserID)
		require.Greater(t, tombstones[0].SyncVersion, chore.SyncVersion)
	})

	t.Run("retained assignee is not revoked", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true}
		seedVisibilityTest(t, db, chore, 2)

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, []int{2}))

		var count int64
		require.NoError(t, db.Model(&syncModel.Tombstone{}).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("added assignee receives bumped private chore", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true}
		seedVisibilityTest(t, db, chore, 2)
		previousVersion := chore.SyncVersion

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, []int{2, 3}))
		require.Greater(t, chore.SyncVersion, previousVersion)

		var count int64
		require.NoError(t, db.Model(&syncModel.Tombstone{}).Count(&count).Error)
		require.Zero(t, count)

		chores, err := repository.GetChores(context.Background(), chore.CircleID, 3, true, &SyncOptions{
			SyncVersion: &previousVersion,
			Limit:       10,
		}, false)
		require.NoError(t, err)
		require.Len(t, chores, 1)
		require.Equal(t, chore.ID, chores[0].ID)
		require.Equal(t, chore.SyncVersion, chores[0].SyncVersion)
	})

	t.Run("creator remains visible without assignment", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true}
		seedVisibilityTest(t, db, chore, 1, 2)

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, nil))

		var tombstones []syncModel.Tombstone
		require.NoError(t, db.Find(&tombstones).Error)
		require.Len(t, tombstones, 1)
		require.Equal(t, 2, *tombstones[0].UserID)
	})
}

func TestUpdateChoreVisibilityPrivacyChanges(t *testing.T) {
	t.Run("public to private revokes newly unauthorized users with distinct versions", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: false}
		seedVisibilityTest(t, db, chore, 2)
		chore.IsPrivate = true

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, []int{2}))

		var tombstones []syncModel.Tombstone
		require.NoError(t, db.Order("sync_version asc").Find(&tombstones).Error)
		require.Len(t, tombstones, 2)
		require.Equal(t, []int{3, 4}, []int{*tombstones[0].UserID, *tombstones[1].UserID})
		require.Equal(t, chore.SyncVersion+1, tombstones[0].SyncVersion)
		require.Equal(t, tombstones[0].SyncVersion+1, tombstones[1].SyncVersion)
		var cursor syncModel.SyncCursor
		require.NoError(t, db.Where("circle_id = ? AND entity_type = ?", chore.CircleID, syncModel.EntityTypeChore).First(&cursor).Error)
		require.Equal(t, tombstones[1].SyncVersion, cursor.MaxVersion)
	})

	t.Run("private to public grants access without revocations", func(t *testing.T) {
		repository, db := newVisibilityTestRepository(t)
		chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true}
		seedVisibilityTest(t, db, chore, 2)
		chore.IsPrivate = false

		require.NoError(t, repository.UpdateChoreVisibility(context.Background(), chore, []int{2}))

		var count int64
		require.NoError(t, db.Model(&syncModel.Tombstone{}).Count(&count).Error)
		require.Zero(t, count)
		require.Positive(t, chore.SyncVersion)
	})
}

func TestUpdateChoreVisibilityRollsBackWhenTombstoneInsertFails(t *testing.T) {
	repository, db := newVisibilityTestRepository(t)
	chore := &chModel.Chore{ID: 10, CircleID: 1, CreatedBy: 1, IsPrivate: true, Name: "before"}
	seedVisibilityTest(t, db, chore, 2)
	require.NoError(t, db.Migrator().DropTable(&syncModel.Tombstone{}))
	chore.Name = "after"

	err := repository.UpdateChoreVisibility(context.Background(), chore, nil)
	require.Error(t, err)

	var storedChore chModel.Chore
	require.NoError(t, db.First(&storedChore, chore.ID).Error)
	require.Equal(t, "before", storedChore.Name)
	var assignees []chModel.ChoreAssignees
	require.NoError(t, db.Where("chore_id = ?", chore.ID).Find(&assignees).Error)
	require.Len(t, assignees, 1)
	require.Equal(t, 2, assignees[0].UserID)
	var cursorCount int64
	require.NoError(t, db.Model(&syncModel.SyncCursor{}).Count(&cursorCount).Error)
	require.Zero(t, cursorCount)
}
