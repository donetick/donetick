package circle

import (
	"time"

	nModel "donetick.com/core/internal/notifier/model"
)

type Circle struct {
	ID         int       `json:"id" gorm:"primary_key"`                 // Unique identifier
	Name       string    `json:"name" gorm:"column:name"`               // Full name
	CreatedBy  int       `json:"created_by" gorm:"column:created_by"`   // Created by
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at"`   // Created at
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at"`   // Updated at
	InviteCode string    `json:"invite_code" gorm:"column:invite_code"` // Invite code
	Disabled   bool      `json:"disabled" gorm:"column:disabled"`       // Disabled
}

type CircleDetail struct {
	Circle
	UserRole string `json:"userRole" gorm:"column:role"`
}

type UserCircle struct {
	ID             int       `json:"id" gorm:"primary_key"`            // Unique identifier
	UserID         int       `json:"userId" gorm:"column:user_id"`     // User ID
	CircleID       int       `json:"circleId" gorm:"column:circle_id"` // Circle ID
	Role           string    `json:"role" gorm:"column:role"`          // Role
	IsActive       bool      `json:"isActive" gorm:"column:is_active;default:false"`
	CreatedAt      time.Time `json:"createdAt" gorm:"column:created_at"`                              // Created at
	UpdatedAt      time.Time `json:"updatedAt" gorm:"column:updated_at"`                              // Updated at
	Points         int       `json:"points" gorm:"column:points;default:0;not null"`                  // Points
	PointsRedeemed int       `json:"pointsRedeemed" gorm:"column:points_redeemed;default:0;not null"` // Points Redeemed
}

type UserCircleDetail struct {
	UserCircle
	Username         string                  `json:"-" gorm:"column:username"`
	DisplayName      string                  `json:"displayName" gorm:"column:display_name"`
	NotificationType nModel.NotificationType `json:"-" gorm:"column:notification_type"`
	TargetID         string                  `json:"-" gorm:"column:target_id"` // Target ID
}
