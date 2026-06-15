package repo

import (
	"context"
	"time"

	rModel "donetick.com/core/internal/reward"
	"gorm.io/gorm"
)

type RewardRepository struct {
	db *gorm.DB
}

func NewRewardRepository(db *gorm.DB) *RewardRepository {
	return &RewardRepository{db}
}

func (r *RewardRepository) GetCircleRewards(c context.Context, circleID int) ([]*rModel.Reward, error) {
	var rewards []*rModel.Reward
	err := r.db.WithContext(c).Where("circle_id = ?", circleID).Order("created_at asc").Find(&rewards).Error
	return rewards, err
}

func (r *RewardRepository) GetRewardByID(c context.Context, circleID, rewardID int) (*rModel.Reward, error) {
	var reward rModel.Reward
	err := r.db.WithContext(c).Where("id = ? AND circle_id = ?", rewardID, circleID).First(&reward).Error
	return &reward, err
}

func (r *RewardRepository) CreateReward(c context.Context, reward *rModel.Reward) error {
	reward.CreatedAt = time.Now().UTC()
	reward.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(c).Create(reward).Error
}

func (r *RewardRepository) UpdateReward(c context.Context, reward *rModel.Reward) error {
	reward.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(c).Model(reward).Updates(map[string]interface{}{
		"name":        reward.Name,
		"description": reward.Description,
		"cost":        reward.Cost,
		"updated_at":  reward.UpdatedAt,
	}).Error
}

func (r *RewardRepository) DeleteReward(c context.Context, circleID, rewardID int) error {
	return r.db.WithContext(c).Where("id = ? AND circle_id = ?", rewardID, circleID).Delete(&rModel.Reward{}).Error
}
