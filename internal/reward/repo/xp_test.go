package repo

import (
	"context"
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	xpRepo "donetick.com/core/internal/points/repo"
	rModel "donetick.com/core/internal/reward/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestXPRedemptionSelectsCategoryAndPreservesRequestedCost(t *testing.T) {
	db := setupTestDB(t)
	r, ctx := NewRewardRepository(db, nil), context.Background()
	seedMember(t, db, 1, 1, 20, 0)
	require.NoError(t, db.Model(&cModel.Circle{}).Where("id = ?", 1).Update("xp_mode", true).Error)
	cat := pModel.XPCategory{CircleID: 1, Name: "Home", NameKey: "home"}
	require.NoError(t, db.Create(&cat).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return xpRepo.ChangeEarned(tx, 1, 1, 50, &cat.ID, "seed", 0)
	}))
	reward := seedReward(t, db, 1, 40)
	_, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	require.ErrorIs(t, err, xpRepo.ErrCategoryRequired)
	zero := 0
	_, err = r.RequestRedemption(ctx, reward.ID, 1, 1, nil, &zero)
	require.Error(t, err, "the aggregate balance cannot cover an underfunded selected category")
	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil, &cat.ID)
	require.NoError(t, err)
	require.NoError(t, db.Model(reward).Update("points_cost", 60).Error)
	require.NoError(t, db.Model(&cat).Updates(map[string]any{"name": "Household", "archived": true}).Error)
	require.NoError(t, r.ApproveRedemption(ctx, redemption.ID, 1, 9))
	require.ErrorIs(t, r.ApproveRedemption(ctx, redemption.ID, 1, 9), ErrRedemptionNotPending)
	stats, err := xpRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 30, stats.TotalXP)
	require.EqualValues(t, 10, stats.Categories["1"])
	require.EqualValues(t, 20, stats.Categories["0"])
}

func TestXPPendingPointsModeRedemptionSurvivesModeChange(t *testing.T) {
	db := setupTestDB(t)
	r, ctx := NewRewardRepository(db, nil), context.Background()
	seedMember(t, db, 1, 1, 10, 0)
	cat := pModel.XPCategory{CircleID: 1, Name: "Home", NameKey: "home"}
	require.NoError(t, db.Create(&cat).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return xpRepo.ChangeEarned(tx, 1, 1, 50, &cat.ID, "seed", 0) }))
	reward := seedReward(t, db, 1, 40)
	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	require.NoError(t, err)
	require.NoError(t, db.Model(&cModel.Circle{}).Where("id = ?", 1).Update("xp_mode", true).Error)
	require.NoError(t, r.ApproveRedemption(ctx, redemption.ID, 1, 9))
	stats, err := xpRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 20, stats.TotalXP)
	require.Zero(t, stats.Categories["0"])
	require.EqualValues(t, 20, stats.Categories["1"])
}

func TestXPPendingRedemptionCannotDebitRejoinedMembership(t *testing.T) {
	db := setupTestDB(t)
	r, ctx := NewRewardRepository(db, nil), context.Background()
	seedMember(t, db, 1, 1, 100, 0)
	reward := seedReward(t, db, 1, 40)
	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	require.NoError(t, err)
	require.NoError(t, db.Where("circle_id = ? AND user_id = ?", 1, 1).Delete(&cModel.UserCircle{}).Error)
	seedMember(t, db, 1, 1, 70, 0)
	require.ErrorIs(t, r.ApproveRedemption(ctx, redemption.ID, 1, 9), xpRepo.ErrMembershipChanged)
	stats, err := xpRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 70, stats.TotalXP)
	var pending rModel.RewardRedemption
	require.NoError(t, db.First(&pending, redemption.ID).Error)
	require.Equal(t, rModel.RedemptionStatusPending, pending.Status)
}

func TestXPLargeLegacyRewardCanStillBeRedeemed(t *testing.T) {
	db := setupTestDB(t)
	r, ctx := NewRewardRepository(db, nil), context.Background()
	seedMember(t, db, 1, 1, 2000000000, 0)
	reward := seedReward(t, db, 1, 1500000000)
	redemption, err := r.RequestRedemption(ctx, reward.ID, 1, 1, nil)
	require.NoError(t, err)
	require.NoError(t, r.ApproveRedemption(ctx, redemption.ID, 1, 9))
	stats, err := xpRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 500000000, stats.TotalXP)
}
