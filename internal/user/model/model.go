package user

import (
	"time"

	nModel "donetick.com/core/internal/notifier/model"
)

type User struct {
	ID          int          `json:"id" gorm:"primary_key"`                  // Unique identifier
	DisplayName string       `json:"displayName" gorm:"column:display_name"` // Display name
	Username    string       `json:"username" gorm:"column:username;unique"` // Username (unique)
	Email       string       `json:"email" gorm:"column:email;unique"`       // Email (unique)
	Provider    AuthProvider `json:"provider" gorm:"column:provider"`        // Provider
	Password    string       `json:"-" gorm:"column:password"`               // Password
	CircleID    int          `json:"circleID" gorm:"column:circle_id"`       // Circle ID
	ChatID      int64        `json:"chatID" gorm:"column:chat_id"`           // Telegram chat ID
	Image       string       `json:"image" gorm:"column:image"`              // Image
	CreatedAt   time.Time    `json:"created_at" gorm:"column:created_at"`    // Created at
	UpdatedAt   time.Time    `json:"updated_at" gorm:"column:updated_at"`    // Updated at
	Disabled    bool         `json:"disabled" gorm:"column:disabled"`        // Disabled
	// Email    string `json:"email" gorm:"column:email"`       // Email
	CustomerID              *string                `gorm:"column:customer_id;<-:false"`                      // read only column
	Subscription            *string                `json:"subscription" gorm:"column:subscription;<-:false"` // read only column
	Expiration              *string                `json:"expiration" gorm:"column:expiration;<-:false"`     // read only column
	UserNotificationTargets UserNotificationTarget `json:"notification_target" gorm:"foreignKey:UserID;references:ID"`
}

type UserPasswordReset struct {
	ID             int       `gorm:"column:id"`
	UserID         int       `gorm:"column:user_id"`
	Email          string    `gorm:"column:email"`
	Token          string    `gorm:"column:token"`
	ExpirationDate time.Time `gorm:"column:expiration_date"`
}

type APIToken struct {
	ID        int       `json:"id" gorm:"primary_key"`              // Unique identifier
	Name      string    `json:"name" gorm:"column:name;unique"`     // Name (unique)
	UserID    int       `json:"userId" gorm:"column:user_id;index"` // Index on userID
	Token     string    `json:"token" gorm:"column:token;index"`    // Index on token
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
}

type UserNotificationTarget struct {
	UserID    int                     `json:"userId" gorm:"column:user_id;index;primaryKey"` // Index on userID
	Type      nModel.NotificationType `json:"type" gorm:"column:type"`                       // Type
	TargetID  string                  `json:"target_id" gorm:"column:target_id"`             // Target ID
	CreatedAt time.Time               `json:"-" gorm:"column:created_at"`
}
type AuthProvider int

const (
	AuthProviderDonetick AuthProvider = iota
	AuthProviderOAuth2
	AuthProviderGoogle
)
