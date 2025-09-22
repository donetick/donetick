package circle

import (
	"time"

	nModel "donetick.com/core/internal/notifier/model"
)

type Circle struct {
	ID                 int        `json:"id" gorm:"primary_key"`                 // Unique identifier
	Name               string     `json:"name" gorm:"column:name"`               // Full name
	CreatedBy          int        `json:"created_by" gorm:"column:created_by"`   // Created by
	CreatedAt          time.Time  `json:"created_at" gorm:"column:created_at"`   // Created at
	UpdatedAt          time.Time  `json:"updated_at" gorm:"column:updated_at"`   // Updated at
	InviteCode         string     `json:"invite_code" gorm:"column:invite_code"` // Invite code
	Disabled           bool       `json:"disabled" gorm:"column:disabled"`       // Disabled
	WebhookURL         *string    `json:"webhook_url" gorm:"column:webhook_url"` // Webhook URL
	SubscriptionStatus *string    `gorm:"column:status;<-:false"`                // read one column
	ExpiredAt          *time.Time `gorm:"column:expired_at;<-:false"`            // read one column
}

type CircleDetail struct {
	Circle
	UserRole string `json:"userRole" gorm:"column:role"`
}

type UserCircle struct {
	ID             int       `json:"id" gorm:"primary_key"`                                        // Unique identifier
	UserID         int       `json:"userId" gorm:"column:user_id;uniqueIndex:idx_user_circle"`     // User ID
	CircleID       int       `json:"circleId" gorm:"column:circle_id;uniqueIndex:idx_user_circle"` // Circle ID
	Role           UserRole  `json:"role" gorm:"column:role"`                                      // Role
	IsActive       bool      `json:"isActive" gorm:"column:is_active;default:false"`
	CreatedAt      time.Time `json:"createdAt" gorm:"column:created_at"`                              // Created at
	UpdatedAt      time.Time `json:"updatedAt" gorm:"column:updated_at"`                              // Updated at
	Points         int       `json:"points" gorm:"column:points;default:0;not null"`                  // Points
	PointsRedeemed int       `json:"pointsRedeemed" gorm:"column:points_redeemed;default:0;not null"` // Points Redeemed
}

type UserRole string

const (
	UserRoleAdmin   UserRole = "admin"
	UserRoleMember  UserRole = "member"
	UserRoleManager UserRole = "manager"
)

type UserCircleDetail struct {
	UserCircle
	Username         string                      `json:"username" gorm:"column:username"`
	DisplayName      string                      `json:"displayName" gorm:"column:display_name"`
	NotificationType nModel.NotificationPlatform `json:"-" gorm:"column:notification_type"`
	TargetID         string                      `json:"-" gorm:"column:target_id"` // Target ID
	Image            string                      `json:"image" gorm:"column:image"` // Image
}

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleMember  Role = "member"
	RoleManager Role = "manager"
)

// write isValidRole method for Role type:
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleMember, RoleManager:
		return true
	default:
		return false
	}
}

func (ucd UserCircleDetail) IsManagerOrAdmin() bool {
	return ucd.Role == UserRoleAdmin || ucd.Role == UserRoleManager
}
