package user

import (
	"time"

	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
)

type User struct {
	ID          int              `json:"id" gorm:"primary_key"`                  // Unique identifier
	DisplayName string           `json:"displayName" gorm:"column:display_name"` // Display name
	Username    string           `json:"username" gorm:"column:username;unique"` // Username (unique)
	Email       string           `json:"email" gorm:"column:email;unique"`       // Email (unique)
	Provider    AuthProviderType `json:"provider" gorm:"column:provider"`        // Provider
	Password    string           `json:"-" gorm:"column:password"`               // Password
	CircleID    int              `json:"circleID" gorm:"column:circle_id"`       // Circle ID
	ChatID      int64            `json:"chatID" gorm:"column:chat_id"`           // Telegram chat ID
	Image       string           `json:"image" gorm:"column:image"`              // Image
	Timezone    string           `json:"timezone" gorm:"column:timezone"`        // Timezone
	// MFA fields
	MFAEnabled      bool      `json:"mfaEnabled" gorm:"column:mfa_enabled;default:false;not null"`    // MFA enabled status
	MFASecret       string    `json:"-" gorm:"column:mfa_secret;type:text"`                           // TOTP secret (hidden from JSON)
	MFABackupCodes  string    `json:"-" gorm:"column:mfa_backup_codes;type:text"`                     // JSON array of backup codes
	MFARecoveryUsed string    `json:"-" gorm:"column:mfa_recovery_codes_used;type:text;default:'[]'"` // JSON array of used recovery codes
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at"`                            // Created at
	UpdatedAt       time.Time `json:"updated_at" gorm:"column:updated_at"`                            // Updated at
	Disabled        bool      `json:"disabled" gorm:"column:disabled"`                                // Disabled
	// Email    string `json:"email" gorm:"column:email"`       // Email
	CustomerID              *string                `gorm:"column:customer_id;<-:false"`                      // read only column
	Subscription            *string                `json:"subscription" gorm:"column:subscription;<-:false"` // read only column
	Expiration              *time.Time             `json:"expiration" gorm:"column:expiration;<-:false"`     // read only column
	UserNotificationTargets UserNotificationTarget `json:"notification_target" gorm:"foreignKey:UserID;references:ID"`
}
type UserDetails struct {
	User
	WebhookURL *string `json:"webhookURL" gorm:"column:webhook_url;<-:false"` // read only column
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
	UserID    int                         `json:"userId" gorm:"column:user_id;index;primaryKey"` // Index on userID
	Type      nModel.NotificationPlatform `json:"type" gorm:"column:type"`                       // Type
	TargetID  string                      `json:"target_id" gorm:"column:target_id"`             // Target ID
	CreatedAt time.Time                   `json:"-" gorm:"column:created_at"`
}

// UserDeviceToken represents FCM/push notification tokens for user devices
type UserDeviceToken struct {
	ID           int       `json:"id" gorm:"primaryKey;autoIncrement"`                                             // Primary key
	UserID       int       `json:"userId" gorm:"column:user_id;not null;uniqueIndex:idx_user_device"`              // User ID
	Token        string    `json:"-" gorm:"column:token;not null"`                                                 // FCM token (unique across system)
	DeviceID     string    `json:"deviceId" gorm:"column:device_id;type:varchar(255);uniqueIndex:idx_user_device"` // Device identifier
	Platform     string    `json:"platform" gorm:"column:platform;type:varchar(10)"`                               // ios, android
	AppVersion   string    `json:"appVersion,omitempty" gorm:"column:app_version;type:varchar(50)"`                // App version
	DeviceModel  string    `json:"deviceModel,omitempty" gorm:"column:device_model;type:varchar(100)"`             // Device model
	IsActive     bool      `json:"isActive" gorm:"column:is_active;default:true;not null;index:idx_user_active"`   // Active status
	LastActiveAt time.Time `json:"lastActiveAt,omitempty" gorm:"column:last_active_at"`                            // Last active timestamp
	CreatedAt    time.Time `json:"createdAt" gorm:"column:created_at"`                                             // Created timestamp
}
type AuthProviderType int

const (
	AuthProviderDonetick AuthProviderType = iota
	AuthProviderOAuth2
	AuthProviderGoogle
	AuthProviderApple
)

// MFASession represents a temporary session during MFA verification
type MFASession struct {
	ID           int       `json:"id" gorm:"primary_key;auto_increment"`
	SessionToken string    `json:"sessionToken" gorm:"column:session_token;type:varchar(255);unique;not null;index"`
	UserID       int       `json:"userId" gorm:"column:user_id;not null;index"`
	AuthMethod   string    `json:"authMethod" gorm:"column:auth_method;type:varchar(50);not null"` // 'local', 'google', 'oauth2'
	Verified     bool      `json:"verified" gorm:"column:verified;default:false;not null"`
	CreatedAt    time.Time `json:"createdAt" gorm:"column:created_at;not null"`
	ExpiresAt    time.Time `json:"expiresAt" gorm:"column:expires_at;not null;index"`
	UserData     string    `json:"-" gorm:"column:user_data;type:text"` // JSON data to complete auth after MFA
}

// MFASetupResponse represents the response when setting up MFA
type MFASetupResponse struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qrCodeUrl"`
	BackupCodes []string `json:"backupCodes"`
}

// MFAVerifyRequest represents a request to verify MFA code
type MFAVerifyRequest struct {
	Code         string `json:"code" binding:"required"`
	SessionToken string `json:"sessionToken,omitempty"` // For login flow
}

func (u User) IsPlusMember() bool {
	// if the user has a subscription, and the expiration date is in the future, then the user is a plus member
	if u.Expiration != nil {
		return u.Expiration.After(time.Now().UTC())
	}

	return false
}

func (u User) IsAdminOrManager(circleUsers []*cModel.UserCircleDetail) bool {
	for _, cu := range circleUsers {
		if cu.UserID == u.ID {
			return cu.Role == cModel.UserRoleAdmin || cu.Role == cModel.UserRoleManager
		}
	}
	return false
}
