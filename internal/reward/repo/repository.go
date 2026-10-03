package repo

import (
	"context"
	"errors"
	"time"

	config "donetick.com/core/config"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	rModel "donetick.com/core/internal/reward/model"
	"gorm.io/gorm"
)

var ErrInsufficientPoints = errors.New("insufficient points balance")
var ErrRedemptionNotPending = errors.New("redemption is not pending")

type RewardRepository struct {
	db *gorm.DB
}

func NewRewardRepository(db *gorm.DB, cfg *config.Config) *RewardRepository {
	return &RewardRepository{db: db}
}

func (r *RewardRepository) GetRewardsByCircle(ctx context.Context, circleID int) ([]*rModel.Reward, error) {
	var rewards []*rModel.Reward
	if err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("points_cost asc").Find(&rewards).Error; err != nil {
		return nil, err
	}
	return rewards, nil
}

func (r *RewardRepository) GetReward(ctx context.Context, id int, circleID int) (*rModel.Reward, error) {
	var reward rModel.Reward
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", id, circleID).First(&reward).Error; err != nil {
		return nil, err
	}
	return &reward, nil
}

func (r *RewardRepository) CreateReward(ctx context.Context, reward *rModel.Reward) error {
	return r.db.WithContext(ctx).Create(reward).Error
}

func (r *RewardRepository) UpdateReward(ctx context.Context, reward *rModel.Reward) error {
	return r.db.WithContext(ctx).Model(&rModel.Reward{}).
		Where("id = ? AND circle_id = ?", reward.ID, reward.CircleID).
		Updates(map[string]interface{}{
			"name":        reward.Name,
			"description": reward.Description,
			"points_cost": reward.PointsCost,
			"is_active":   reward.IsActive,
		}).Error
}

func (r *RewardRepository) DeleteReward(ctx context.Context, id int, circleID int) error {
	return r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", id, circleID).Delete(&rModel.Reward{}).Error
}

// RequestRedemption validates the reward exists/is active and the user has enough
// available balance (points - points_redeemed), then inserts a pending redemption
// request. Balance is re-checked at approval time too (see ApproveRedemption) since
// balance can change between request and approval.
func (r *RewardRepository) RequestRedemption(ctx context.Context, rewardID int, userID int, circleID int, note *string) (*rModel.RewardRedemption, error) {
	var redemption *rModel.RewardRedemption
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reward rModel.Reward
		if err := tx.Where("id = ? AND circle_id = ? AND is_active = ?", rewardID, circleID, true).First(&reward).Error; err != nil {
			return err
		}

		var member cModel.UserCircle
		if err := tx.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&member).Error; err != nil {
			return err
		}
		available := member.Points - member.PointsRedeemed
		if available < reward.PointsCost {
			return ErrInsufficientPoints
		}

		redemption = &rModel.RewardRedemption{
			RewardID:    reward.ID,
			CircleID:    circleID,
			UserID:      userID,
			RewardName:  reward.Name,
			PointsCost:  reward.PointsCost,
			Status:      rModel.RedemptionStatusPending,
			Note:        note,
			RequestedAt: time.Now().UTC(),
		}
		return tx.Create(redemption).Error
	})
	if err != nil {
		return nil, err
	}
	return redemption, nil
}

// ApproveRedemption debits the user's points (same points_redeemed increment +
// PointsHistory pattern as CircleRepository.RedeemPoints) and marks the redemption
// approved. Re-validates balance and pending status inside the transaction to avoid
// a race with other redemptions/undo actions approved in between.
func (r *RewardRepository) ApproveRedemption(ctx context.Context, redemptionID int, circleID int, adminUserID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var redemption rModel.RewardRedemption
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ? AND circle_id = ?", redemptionID, circleID).First(&redemption).Error; err != nil {
			return err
		}
		if redemption.Status != rModel.RedemptionStatusPending {
			return ErrRedemptionNotPending
		}

		var member cModel.UserCircle
		if err := tx.Where("user_id = ? AND circle_id = ?", redemption.UserID, circleID).First(&member).Error; err != nil {
			return err
		}
		available := member.Points - member.PointsRedeemed
		if available < redemption.PointsCost {
			return ErrInsufficientPoints
		}

		if err := tx.Model(&cModel.UserCircle{}).
			Where("user_id = ? AND circle_id = ?", redemption.UserID, circleID).
			Update("points_redeemed", gorm.Expr("points_redeemed + ?", redemption.PointsCost)).Error; err != nil {
			return err
		}
		if err := tx.Create(&pModel.PointsHistory{
			Action:    pModel.PointsHistoryActionRedeem,
			CircleID:  circleID,
			UserID:    redemption.UserID,
			Points:    redemption.PointsCost,
			CreatedAt: time.Now().UTC(),
			CreatedBy: adminUserID,
		}).Error; err != nil {
			return err
		}

		now := time.Now().UTC()
		return tx.Model(&rModel.RewardRedemption{}).Where("id = ?", redemption.ID).Updates(map[string]interface{}{
			"status":      rModel.RedemptionStatusApproved,
			"resolved_at": now,
			"resolved_by": adminUserID,
		}).Error
	})
}

func (r *RewardRepository) RejectRedemption(ctx context.Context, redemptionID int, circleID int, adminUserID int, note *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var redemption rModel.RewardRedemption
		if err := tx.Where("id = ? AND circle_id = ?", redemptionID, circleID).First(&redemption).Error; err != nil {
			return err
		}
		if redemption.Status != rModel.RedemptionStatusPending {
			return ErrRedemptionNotPending
		}

		now := time.Now().UTC()
		updates := map[string]interface{}{
			"status":      rModel.RedemptionStatusRejected,
			"resolved_at": now,
			"resolved_by": adminUserID,
		}
		if note != nil {
			updates["note"] = *note
		}
		return tx.Model(&rModel.RewardRedemption{}).Where("id = ?", redemption.ID).Updates(updates).Error
	})
}

func (r *RewardRepository) GetRedemptionsByCircle(ctx context.Context, circleID int, status *rModel.RedemptionStatus) ([]*rModel.RewardRedemption, error) {
	q := r.db.WithContext(ctx).Where("circle_id = ?", circleID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var redemptions []*rModel.RewardRedemption
	if err := q.Order("requested_at desc").Find(&redemptions).Error; err != nil {
		return nil, err
	}
	return redemptions, nil
}

func (r *RewardRepository) GetRedemptionsByUser(ctx context.Context, userID int, circleID int) ([]*rModel.RewardRedemption, error) {
	var redemptions []*rModel.RewardRedemption
	if err := r.db.WithContext(ctx).Where("user_id = ? AND circle_id = ?", userID, circleID).
		Order("requested_at desc").Find(&redemptions).Error; err != nil {
		return nil, err
	}
	return redemptions, nil
}
