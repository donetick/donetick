package model

import "time"

// Reward is a circle-scoped catalog item that members can redeem points for.
type Reward struct {
	ID          int       `json:"id" gorm:"primary_key"`
	CircleID    int       `json:"circleId" gorm:"column:circle_id;index"`
	Name        string    `json:"name" gorm:"column:name"`
	Description string    `json:"description" gorm:"column:description"`
	PointsCost  int       `json:"pointsCost" gorm:"column:points_cost;not null"`
	IsActive    bool      `json:"isActive" gorm:"column:is_active;default:true"`
	CreatedBy   int       `json:"createdBy" gorm:"column:created_by"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

type RedemptionStatus int8

const (
	RedemptionStatusPending RedemptionStatus = iota
	RedemptionStatusApproved
	RedemptionStatusRejected
)

// RewardRedemption is a member's request to spend points on a Reward, subject
// to admin/manager approval — mirrors the chore ApproveChore/RejectChore flow.
type RewardRedemption struct {
	ID          int              `json:"id" gorm:"primary_key"`
	RewardID    int              `json:"rewardId" gorm:"column:reward_id;index"`
	CircleID    int              `json:"circleId" gorm:"column:circle_id;index"`
	UserID      int              `json:"userId" gorm:"column:user_id;index"`
	RewardName  string           `json:"rewardName" gorm:"column:reward_name"` // snapshot, survives reward edits/deletes
	PointsCost  int              `json:"pointsCost" gorm:"column:points_cost"` // snapshot at request time
	Status      RedemptionStatus `json:"status" gorm:"column:status"`
	Note        *string          `json:"note" gorm:"column:note"`
	RequestedAt time.Time        `json:"requestedAt" gorm:"column:requested_at"`
	ResolvedAt  *time.Time       `json:"resolvedAt" gorm:"column:resolved_at"`
	ResolvedBy  *int             `json:"resolvedBy" gorm:"column:resolved_by"`
}
