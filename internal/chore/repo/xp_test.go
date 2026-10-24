package chore

import (
	"context"
	"errors"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	pRepo "donetick.com/core/internal/points/repo"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func xpChoreFixture(t *testing.T) (*ChoreRepository, *gorm.DB, *chModel.Chore, int) {
	t.Helper()
	r, db := newTestChoreRepo(t)
	require.NoError(t, db.Create(&cModel.Circle{ID: 1, XPMode: true}).Error)
	for _, id := range []int{1, 2} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: 1, IsActive: true}).Error)
	}
	category := pModel.XPCategory{CircleID: 1, Name: "Housework", NameKey: "housework"}
	require.NoError(t, db.Create(&category).Error)
	points := 30
	chore := createChore(t, db, "XP task", 1, false, nil, 1, 2)
	chore.Points, chore.XPCategoryID = &points, &category.ID
	chore.FrequencyType = chModel.FrequencyTypeAlways
	require.NoError(t, db.Save(chore).Error)
	return r, db, chore, category.ID
}

func reloadXPChore(t *testing.T, db *gorm.DB, id int) *chModel.Chore {
	t.Helper()
	var chore chModel.Chore
	require.NoError(t, db.First(&chore, id).Error)
	return &chore
}

func TestXPCompletionUsesActualCompleterAndUndoOriginalSnapshot(t *testing.T) {
	r, db, chore, categoryID := xpChoreFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	// Starting a timer as another member must not give that starter the award.
	require.NoError(t, db.Create(&chModel.ChoreHistory{ChoreID: chore.ID, CompletedBy: 2, Status: chModel.ChoreHistoryStatusStarted}).Error)
	require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true))
	var history chModel.ChoreHistory
	require.NoError(t, db.Where("chore_id = ?", chore.ID).First(&history).Error)
	require.Equal(t, 1, history.CompletedBy)
	require.Equal(t, 30, *history.Points)
	// Editing the task and renaming/archiving its category do not rewrite history.
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Update("points", 90).Error)
	require.NoError(t, db.Model(&pModel.XPCategory{}).Where("id = ?", categoryID).Updates(map[string]any{"name": "Home", "archived": true}).Error)
	stats, err := pRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 30, stats.TotalXP)
	require.EqualValues(t, 30, stats.Categories["1"])
	other, err := pRepo.Stats(db, 1, 2)
	require.NoError(t, err)
	require.Zero(t, other.TotalXP)
	require.NoError(t, r.UndoChoreAction(ctx, chore.ID, history.ID, 1, nil, nil))
	stats, err = pRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.Zero(t, stats.TotalXP)
	require.Zero(t, stats.Categories["1"])
	require.ErrorIs(t, r.UndoChoreAction(ctx, chore.ID, history.ID, 1, nil, nil), gorm.ErrRecordNotFound)
}

func TestXPPendingApprovalKeepsValueThroughEditRejectUndoAndApproval(t *testing.T) {
	r, db, chore, categoryID := xpChoreFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	require.NoError(t, r.SetChorePendingApproval(ctx, chore, nil, 1, &now))
	stats, err := pRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.Zero(t, stats.TotalXP, "submission alone must not award XP")
	otherCategory := pModel.XPCategory{CircleID: 1, Name: "Exercise", NameKey: "exercise"}
	require.NoError(t, db.Create(&otherCategory).Error)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(map[string]any{"points": 90, "xp_category_id": otherCategory.ID}).Error)
	require.NoError(t, r.RejectChore(ctx, chore.ID, 1, nil))
	var rejected chModel.ChoreHistory
	require.NoError(t, db.Where("chore_id = ?", chore.ID).First(&rejected).Error)
	require.NoError(t, r.UndoChoreAction(ctx, chore.ID, rejected.ID, 1, nil, nil))
	var restored chModel.ChoreHistory
	require.NoError(t, db.First(&restored, rejected.ID).Error)
	require.Equal(t, chModel.ChoreHistoryStatusPendingApproval, restored.Status)
	require.Equal(t, categoryID, *restored.XPCategoryID)
	require.Equal(t, 30, *restored.XPValue)
	require.NoError(t, r.ApproveChore(ctx, reloadXPChore(t, db, chore.ID), 2, nil, nil, true))
	stats, err = pRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 30, stats.TotalXP)
	require.EqualValues(t, 30, stats.Categories["1"])
	require.Zero(t, stats.Categories["2"])
	require.Error(t, r.ApproveChore(ctx, reloadXPChore(t, db, chore.ID), 2, nil, nil, true), "approval cannot be applied twice")
}

func TestXPStaleCompletionCannotAwardSameOccurrenceTwice(t *testing.T) {
	for _, frequency := range []chModel.FrequencyType{chModel.FrequencyTypeOnce, chModel.FrequencyTypeAlways} {
		t.Run(string(frequency), func(t *testing.T) {
			r, db, chore, _ := xpChoreFixture(t)
			chore.FrequencyType = frequency
			require.NoError(t, db.Save(chore).Error)
			stale := *chore // Two handlers loaded the same task before either write.
			ctx, now := context.Background(), time.Now().UTC()
			require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true))
			require.ErrorIs(t, r.CompleteChore(ctx, &stale, nil, 1, nil, &now, nil, true), ErrChoreChanged)
			stats, err := pRepo.Stats(db, 1, 1)
			require.NoError(t, err)
			require.EqualValues(t, 30, stats.TotalXP)
			if frequency == chModel.FrequencyTypeAlways {
				require.NoError(t, r.CompleteChore(ctx, reloadXPChore(t, db, chore.ID), nil, 1, nil, &now, nil, true))
				stats, err = pRepo.Stats(db, 1, 1)
				require.NoError(t, err)
				require.EqualValues(t, 60, stats.TotalXP, "a fresh intentional completion of an always task remains supported")
			}
		})
	}
}

func TestXPStalePendingSubmissionCannotCreateTwoAwards(t *testing.T) {
	r, db, chore, _ := xpChoreFixture(t)
	stale := *chore
	ctx, now := context.Background(), time.Now().UTC()
	require.NoError(t, r.SetChorePendingApproval(ctx, chore, nil, 1, &now))
	require.ErrorIs(t, r.SetChorePendingApproval(ctx, &stale, nil, 1, &now), ErrChoreChanged)
	var count int64
	require.NoError(t, db.Model(&chModel.ChoreHistory{}).Where("chore_id = ? AND status = ?", chore.ID, chModel.ChoreHistoryStatusPendingApproval).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestXPMembershipChangeCannotGrantPendingAwardOrReverseOldMembership(t *testing.T) {
	for _, pending := range []bool{true, false} {
		t.Run(map[bool]string{true: "pending", false: "completed"}[pending], func(t *testing.T) {
			r, db, chore, _ := xpChoreFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			if pending {
				require.NoError(t, r.SetChorePendingApproval(ctx, chore, nil, 1, &now))
			} else {
				require.NoError(t, r.CompleteChore(ctx, chore, nil, 1, nil, &now, nil, true))
			}
			var history chModel.ChoreHistory
			require.NoError(t, db.Where("chore_id = ?", chore.ID).First(&history).Error)
			require.NoError(t, db.Where("circle_id = ? AND user_id = ?", 1, 1).Delete(&cModel.UserCircle{}).Error)
			require.NoError(t, db.Create(&cModel.UserCircle{CircleID: 1, UserID: 1, Points: 7, IsActive: true}).Error)
			if pending {
				require.True(t, errors.Is(r.ApproveChore(ctx, reloadXPChore(t, db, chore.ID), 2, nil, nil, true), pRepo.ErrMembershipChanged))
			} else {
				require.NoError(t, r.UndoChoreAction(ctx, chore.ID, history.ID, 1, nil, nil))
			}
			stats, err := pRepo.Stats(db, 1, 1)
			require.NoError(t, err)
			require.EqualValues(t, 7, stats.TotalXP)
			require.EqualValues(t, 7, stats.Categories["0"])
		})
	}
}
