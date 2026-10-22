package chore

import (
	"context"
	chModel "donetick.com/core/internal/chore/model"
	circle "donetick.com/core/internal/circle/model"
	xp "donetick.com/core/internal/xp/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func xpFixture(t *testing.T) (*ChoreRepository, *gorm.DB, *chModel.Chore) {
	t.Helper()
	r, db := newTestChoreRepo(t)
	require.NoError(t, db.Create(&circle.UserCircle{CircleID: 1, UserID: 1, IsActive: true, Points: 30}).Error)
	category := xp.Category{CircleID: 1, Name: "Learning", NormalizedName: "learning"}
	require.NoError(t, db.Create(&category).Error)
	chore := createChore(t, db, "XP task", 1, false, nil)
	points := 50
	chore.Points = &points
	chore.XPCategoryID = &category.ID
	require.NoError(t, db.Save(chore).Error)
	return r, db, chore
}
func TestXPAwardSnapshotUndoAndReplay(t *testing.T) {
	r, db, chore := xpFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true))
	require.Error(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true), "stale completion must not award twice")
	var award xp.Award
	require.NoError(t, db.First(&award).Error)
	require.Equal(t, 50, award.Points)
	var m circle.UserCircle
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 80, m.Points)
	// A later edit and archival cannot move the original award.
	require.NoError(t, db.Model(chore).Updates(map[string]interface{}{"points": 99, "xp_category_id": nil}).Error)
	require.NoError(t, db.Model(&xp.Category{}).Where("id = ?", award.CategoryID).Update("archived", true).Error)
	require.NoError(t, r.UndoChoreAction(ctx, chore.ID, award.HistoryID, 1, nil, nil))
	require.Error(t, r.UndoChoreAction(ctx, chore.ID, award.HistoryID, 1, nil, nil))
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 30, m.Points)
	var n int64
	require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
	require.Zero(t, n)
}
func TestXPTaskDeletionRetainsLifetimeAward(t *testing.T) {
	r, db, chore := xpFixture(t)
	now := time.Now().UTC()
	require.NoError(t, r.CompleteChore(context.Background(), chore, nil, 1, nil, &now, nil, true))
	_, err := r.DeleteChore(context.Background(), chore.ID)
	require.NoError(t, err)
	var a xp.Award
	require.NoError(t, db.First(&a).Error)
	require.Equal(t, 50, a.Points)
	var m circle.UserCircle
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 80, m.Points)
}
func TestXPSkipAndNoPointsDoNotAward(t *testing.T) {
	for _, skip := range []bool{true, false} {
		t.Run(map[bool]string{true: "skip", false: "no points"}[skip], func(t *testing.T) {
			r, db, chore := xpFixture(t)
			now := time.Now().UTC()
			ctx := context.Background()
			if skip {
				require.NoError(t, r.SkipChore(ctx, chore, 1, nil, nil, &now))
			} else {
				require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, false))
			}
			var n int64
			require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
			require.Zero(t, n)
			var m circle.UserCircle
			require.NoError(t, db.First(&m).Error)
			require.Equal(t, 30, m.Points)
		})
	}
}
func TestXPApprovalAwardsOnce(t *testing.T) {
	r, db, chore := xpFixture(t)
	now := time.Now().UTC()
	history := chModel.ChoreHistory{ChoreID: chore.ID, CompletedBy: 1, Status: chModel.ChoreHistoryStatusPendingApproval, PerformedAt: &now}
	require.NoError(t, db.Create(&history).Error)
	var n int64
	require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
	require.Zero(t, n)
	require.NoError(t, r.ApproveChore(context.Background(), chore, 2, nil, nil, true))
	require.Error(t, r.ApproveChore(context.Background(), chore, 2, nil, nil, true))
	require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
	require.EqualValues(t, 1, n)
	var a xp.Award
	require.NoError(t, db.First(&a).Error)
	require.Equal(t, 1, a.UserID)
}
func TestXPValidation(t *testing.T) {
	_, db, chore := xpFixture(t)
	require.NoError(t, validateXP(db, chore, nil))
	previous := *chore
	require.NoError(t, db.Model(&xp.Category{}).Where("id = ?", *chore.XPCategoryID).Update("archived", true).Error)
	require.ErrorIs(t, validateXP(db, chore, nil), ErrInvalidXP)
	require.NoError(t, validateXP(db, chore, &previous))
	chore.CircleID = 2
	require.ErrorIs(t, validateXP(db, chore, &previous), ErrInvalidXP)
	chore.CircleID = 1
	chore.XPCategoryID = nil
	for _, points := range []int{-1, 1001} {
		chore.Points = &points
		require.ErrorIs(t, validateXP(db, chore, nil), ErrInvalidXP)
	}
	for _, points := range []int{0, 1000} {
		chore.Points = &points
		require.NoError(t, validateXP(db, chore, nil))
	}
	chore.Points = nil
	require.NoError(t, validateXP(db, chore, nil))
}
func TestXPAwardRollback(t *testing.T) {
	r, db, chore := xpFixture(t)
	now := time.Now().UTC()
	// Unique ledger constraint failure must also roll back points and completion.
	require.NoError(t, db.Create(&xp.Award{HistoryID: 1, CircleID: 1, UserID: 1, CategoryID: *chore.XPCategoryID, Points: 2}).Error)
	require.Error(t, r.CompleteChore(context.Background(), chore, nil, 1, nil, &now, nil, true))
	var m circle.UserCircle
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 30, m.Points)
	var count int64
	require.NoError(t, db.Model(&chModel.ChoreHistory{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestXPConcurrentCompletionAndUndo(t *testing.T) {
	r, db, chore := xpFixture(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	now := time.Now().UTC()
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- r.CompleteChore(context.Background(), chore, nil, 1, nil, &now, nil, true) }()
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	var a xp.Award
	require.NoError(t, db.First(&a).Error)
	start = make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- r.UndoChoreAction(context.Background(), chore.ID, a.HistoryID, 1, nil, nil)
		}()
	}
	close(start)
	success = 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	var m circle.UserCircle
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 30, m.Points)
}

func TestXPAlwaysTaskFreshCompletions(t *testing.T) {
	r, db, chore := xpFixture(t)
	chore.FrequencyType = chModel.FrequencyTypeAlways
	require.NoError(t, db.Save(chore).Error)
	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		require.NoError(t, db.First(chore, chore.ID).Error)
		require.NoError(t, r.CompleteChore(context.Background(), chore, nil, 1, nil, &now, nil, true))
	}
	var n int64
	require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
	require.EqualValues(t, 2, n)
	var m circle.UserCircle
	require.NoError(t, db.First(&m).Error)
	require.Equal(t, 130, m.Points)
}

func TestXPUndoUsesAwardMembershipAndOriginalPoints(t *testing.T) {
	for _, rejoin := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale history", true: "rejoined member"}[rejoin], func(t *testing.T) {
			r, db, chore := xpFixture(t)
			now := time.Now().UTC()
			ctx := context.Background()
			require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true))
			var award xp.Award
			require.NoError(t, db.First(&award).Error)
			var m circle.UserCircle
			require.NoError(t, db.First(&m).Error)
			expected := 30
			if rejoin {
				require.NoError(t, db.Delete(&m).Error)
				m = circle.UserCircle{CircleID: 1, UserID: 1, IsActive: true, Points: 9}
				require.NoError(t, db.Create(&m).Error)
				require.NotEqual(t, award.MembershipID, m.ID)
				expected = 9
			} else {
				// A history edit may have loaded pending state before approval and later
				// saved that stale snapshot. The award remains the source for reversal.
				require.NoError(t, db.Model(&chModel.ChoreHistory{}).Where("id = ?", award.HistoryID).Updates(map[string]interface{}{"points": 1, "status": chModel.ChoreHistoryStatusPendingApproval}).Error)
			}
			require.NoError(t, r.UndoChoreAction(ctx, chore.ID, award.HistoryID, 1, nil, nil))
			require.NoError(t, db.First(&m, m.ID).Error)
			require.Equal(t, expected, m.Points)
			var n int64
			require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
			require.Zero(t, n)
		})
	}
}
