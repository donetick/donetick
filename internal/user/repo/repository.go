package user

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	storageModel "donetick.com/core/internal/storage/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type IUserRepository interface {
	GetUserByUsername(username string) (*uModel.UserDetails, error)
	GetUser(id int) (*uModel.User, error)
	GetAllUsers() ([]*uModel.User, error)
	CreateUser(user *uModel.User) error
	UpdateUser(user *uModel.User) error
	UpdateUserCircle(userID, circleID int) error
	FindByEmail(email string) (*uModel.User, error)
	// MFA-related methods
	EnableMFA(c context.Context, userID int, secret string, backupCodes []string) error
	DisableMFA(c context.Context, userID int) error
	UpdateMFARecoveryCodes(c context.Context, userID int, usedCodes string) error
	// MFA Session methods
	CreateMFASession(c context.Context, session *uModel.MFASession) error
	GetMFASession(c context.Context, sessionToken string) (*uModel.MFASession, error)
	UpdateMFASession(c context.Context, session *uModel.MFASession) error
	DeleteMFASession(c context.Context, sessionToken string) error
	CleanupExpiredMFASessions(c context.Context) error
	// Device Token methods
	RegisterDeviceToken(c context.Context, deviceToken *uModel.UserDeviceToken) error
	UnregisterDeviceToken(c context.Context, userID int, deviceID string) error
	GetUserDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	GetActiveDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	UpdateDeviceTokenActivity(c context.Context, userID int, deviceID string) error
}

type UserRepository struct {
	db               *gorm.DB
	isDonetickDotCom bool
}

func NewUserRepository(db *gorm.DB, cfg *config.Config) *UserRepository {
	return &UserRepository{db, cfg.IsDoneTickDotCom}
}

func (r *UserRepository) GetAllUsers(c context.Context, circleID int) ([]*uModel.User, error) {
	var users []*uModel.User
	if err := r.db.WithContext(c).Where("circle_id = ?", circleID).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) GetAllUsersForSystemOnly(c context.Context) ([]*uModel.User, error) {
	var users []*uModel.User
	if err := r.db.WithContext(c).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
func (r *UserRepository) CreateUser(c context.Context, user *uModel.User) (*uModel.User, error) {
	if err := r.db.WithContext(c).Create(user).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(c).Create(&storageModel.StorageUsage{
		CircleID:  user.CircleID,
		UserID:    user.ID,
		UsedBytes: 0,
		UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		return nil, err
	}

	return user, nil
}
func (r *UserRepository) GetUserByUsername(c context.Context, username string) (*uModel.UserDetails, error) {
	var user *uModel.UserDetails
	if r.isDonetickDotCom {
		now := time.Now().UTC()
		if err := r.db.WithContext(c).Preload("UserNotificationTargets").Table("users u").Select("u.*, s.status as subscription, s.expires_at as expiration, c.webhook_url as webhook_url").Joins("left join subscriptions s on s.circle_id = u.circle_id AND s.status = 'active' AND (s.expires_at IS NULL OR s.expires_at > ?)", now).Joins("left join circles c on c.id = u.circle_id").Where("username = ?", username).First(&user).Error; err != nil {
			return nil, err
		}
	} else {
		// For self-hosted, first get the user without subscription/expiration fields
		if err := r.db.WithContext(c).Preload("UserNotificationTargets").Table("users u").Select("u.*, c.webhook_url as webhook_url").Joins("left join circles c on c.id = u.circle_id").Where("username = ?", username).First(&user).Error; err != nil {
			return nil, err
		}
		// Then manually set the subscription status and expiration for self-hosted users
		subscription := "active"
		futureDate := time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC)
		user.Subscription = &subscription
		user.Expiration = &futureDate
	}

	return user, nil
}

func (r *UserRepository) GetUserByID(c context.Context, userID int) (*uModel.User, error) {
	var user *uModel.User
	now := time.Now().UTC()
	if r.isDonetickDotCom {
		if err := r.db.WithContext(c).Preload("UserNotificationTargets").
			Table("users u").
			Select("u.*, s.status as subscription, s.expires_at as expiration, c.webhook_url as webhook_url").
			Joins("left join subscriptions s on s.circle_id = u.circle_id AND s.status = 'active' AND s.expires_at > ?", now).
			Joins("left join circles c on c.id = u.circle_id").
			Where("u.id = ?", userID).
			First(&user).Error; err != nil {
			return nil, err
		}
	} else {
		if err := r.db.WithContext(c).Preload("UserNotificationTargets").
			Table("users u").
			Select("u.*, c.webhook_url as webhook_url").
			Joins("left join circles c on c.id = u.circle_id").
			Where("u.id = ?", userID).
			First(&user).Error; err != nil {
			return nil, err
		}
		// Set default subscription/expiration for self-hosted
		subscription := "active"
		futureDate := time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC)

		user.Subscription = &subscription
		user.Expiration = &futureDate

	}
	return user, nil
}

func (r *UserRepository) UpdateUser(c context.Context, user *uModel.User) error {
	return r.db.WithContext(c).Save(user).Error
}

func (r *UserRepository) UpdateUserCircle(c context.Context, userID, circleID int) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("circle_id", circleID).Error
}

func (r *UserRepository) FindByEmail(c context.Context, email string) (*uModel.UserDetails, error) {
	var user *uModel.UserDetails
	if err := r.db.WithContext(c).Table("users u").Select("u.*, c.webhook_url as webhook_url").Joins("left join circles c on c.id = u.circle_id").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) SetPasswordResetToken(c context.Context, email, token string) error {
	// confirm user exists with email:
	user, err := r.FindByEmail(c, email)
	if err != nil {
		return err
	}
	// save new token:
	if err := r.db.WithContext(c).Model(&uModel.UserPasswordReset{}).Save(&uModel.UserPasswordReset{
		UserID:         user.ID,
		Token:          token,
		Email:          email,
		ExpirationDate: time.Now().UTC().Add(time.Hour * 24),
	}).Error; err != nil {
		return err
	}
	return nil
}

func (r *UserRepository) UpdatePasswordByToken(ctx context.Context, email string, token string, password string) error {
	logger := logging.FromContext(ctx)

	logger.Debugw("account.db.UpdatePasswordByToken", "email", email)
	upr := &uModel.UserPasswordReset{
		Email: email,
		Token: token,
	}
	result := r.db.WithContext(ctx).Where("email = ?", email).Where("token = ?", token).Delete(upr)
	if result.RowsAffected <= 0 {
		return fmt.Errorf("invalid token")
	}
	// find account by email and update password:
	chain := r.db.WithContext(ctx).Model(&uModel.User{}).Where("email = ?", email).UpdateColumns(map[string]interface{}{"password": password})
	if chain.Error != nil {
		return chain.Error
	}
	if chain.RowsAffected == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *UserRepository) StoreAPIToken(c context.Context, userID int, name string, tokenCode string) (*uModel.APIToken, error) {
	token := &uModel.APIToken{
		UserID:    userID,
		Name:      name,
		Token:     tokenCode,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(c).Model(&uModel.APIToken{}).Save(
		token).Error; err != nil {
		return nil, err

	}
	return token, nil
}

func (r *UserRepository) GetUserByToken(c context.Context, token string) (*uModel.UserDetails, error) {
	var user *uModel.UserDetails
	now := time.Now().UTC()

	if r.isDonetickDotCom {
		if err := r.db.WithContext(c).Table("users u").Select("u.*, s.status as subscription, s.expires_at as expiration, c.webhook_url as webhook_url").Joins("left join api_tokens at on at.user_id = u.id").Joins("left join subscriptions s on s.circle_id = u.circle_id AND s.status = 'active' AND (s.expires_at IS NULL OR s.expires_at > ?)", now).Joins("left join circles c on c.id = u.circle_id").Where("at.token = ?", token).First(&user).Error; err != nil {
			return nil, err
		}
	} else {
		if err := r.db.WithContext(c).Table("users u").Select("u.*, c.webhook_url as webhook_url").Joins("left join api_tokens at on at.user_id = u.id").Joins("left join circles c on c.id = u.circle_id").Where("at.token = ?", token).First(&user).Error; err != nil {
			return nil, err
		}
		// Set default subscription/expiration for self-hosted
		subscription := "active"
		futureDate := time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC)
		user.Subscription = &subscription
		user.Expiration = &futureDate
	}

	return user, nil
}

func (r *UserRepository) GetAllUserTokens(c context.Context, userID int) ([]*uModel.APIToken, error) {
	var tokens []*uModel.APIToken
	if err := r.db.WithContext(c).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *UserRepository) DeleteAPIToken(c context.Context, userID int, tokenID string) error {
	return r.db.WithContext(c).Where("id = ? AND user_id = ?", tokenID, userID).Delete(&uModel.APIToken{}).Error
}

func (r *UserRepository) UpdateNotificationTarget(c context.Context, userID int, targetID string, targetType nModel.NotificationPlatform) error {
	return r.db.WithContext(c).Save(&uModel.UserNotificationTarget{
		UserID:    userID,
		TargetID:  targetID,
		Type:      targetType,
		CreatedAt: time.Now().UTC(),
	}).Error
}

func (r *UserRepository) DeleteNotificationTarget(c context.Context, userID int) error {
	return r.db.WithContext(c).Where("user_id = ?", userID).Delete(&uModel.UserNotificationTarget{}).Error
}

func (r *UserRepository) UpdateNotificationTargetForAllNotifications(c context.Context, userID int, targetID string, targetType nModel.NotificationPlatform) error {
	return r.db.WithContext(c).Model(&nModel.Notification{}).Where("user_id = ?", userID).Update("target_id", targetID).Update("type", targetType).Error
}
func (r *UserRepository) UpdatePasswordByUserId(c context.Context, userID int, password string) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("password", password).Error
}
func (r *UserRepository) UpdateUserImage(c context.Context, userID int, image string) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("image", image).Error
}

// MFA-related methods

// EnableMFA enables MFA for a user with the provided secret and backup codes
func (r *UserRepository) EnableMFA(c context.Context, userID int, secret string, backupCodes []string) error {
	backupCodesJSON, err := json.Marshal(backupCodes)
	if err != nil {
		return err
	}

	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"mfa_enabled":             true,
		"mfa_secret":              secret,
		"mfa_backup_codes":        string(backupCodesJSON),
		"mfa_recovery_codes_used": "[]",
	}).Error
}

// DisableMFA disables MFA for a user
func (r *UserRepository) DisableMFA(c context.Context, userID int) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"mfa_enabled":             false,
		"mfa_secret":              "",
		"mfa_backup_codes":        "",
		"mfa_recovery_codes_used": "",
	}).Error
}

// UpdateMFARecoveryCodes updates the used recovery codes for a user
func (r *UserRepository) UpdateMFARecoveryCodes(c context.Context, userID int, usedCodes string) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("mfa_recovery_codes_used", usedCodes).Error
}

// MFA Session methods

// CreateMFASession creates a new MFA session
func (r *UserRepository) CreateMFASession(c context.Context, session *uModel.MFASession) error {
	return r.db.WithContext(c).Create(session).Error
}

// GetMFASession retrieves an MFA session by token
func (r *UserRepository) GetMFASession(c context.Context, sessionToken string) (*uModel.MFASession, error) {
	var session uModel.MFASession
	if err := r.db.WithContext(c).Where("session_token = ? AND expires_at > ?", sessionToken, time.Now()).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateMFASession updates an MFA session
func (r *UserRepository) UpdateMFASession(c context.Context, session *uModel.MFASession) error {
	return r.db.WithContext(c).Save(session).Error
}

// DeleteMFASession deletes an MFA session
func (r *UserRepository) DeleteMFASession(c context.Context, sessionToken string) error {
	return r.db.WithContext(c).Where("session_token = ?", sessionToken).Delete(&uModel.MFASession{}).Error
}

// CleanupExpiredMFASessions removes expired MFA sessions
func (r *UserRepository) CleanupExpiredMFASessions(c context.Context) error {
	return r.db.WithContext(c).Where("expires_at < ?", time.Now()).Delete(&uModel.MFASession{}).Error
}
