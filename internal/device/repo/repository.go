package device

import (
	"context"
	"fmt"
	"time"

	errorx "donetick.com/core/internal/error"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

const MaxDevicesPerUser = 5

type IDeviceRepository interface {
	RegisterDeviceToken(c context.Context, deviceToken *uModel.UserDeviceToken) error
	UnregisterDeviceToken(c context.Context, userID int, deviceID string) error
	UnregisterDeviceTokenByToken(c context.Context, userID int, token string) error
	GetUserDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	GetActiveDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	GetActiveDeviceCount(c context.Context, userID int) (int64, error)
	UpdateDeviceTokenActivity(c context.Context, userID int, deviceID string) error
	CleanupInactiveTokens(c context.Context, inactiveDays int) error
}

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// RegisterDeviceToken registers or updates a device token for a user
func (r *DeviceRepository) RegisterDeviceToken(c context.Context, deviceToken *uModel.UserDeviceToken) error {
	log := logging.FromContext(c)

	// Check if adding this device would exceed the limit
	var existingDevice uModel.UserDeviceToken
	isNewDevice := r.db.WithContext(c).
		Where("user_id = ? AND device_id = ? AND is_active = ?",
			deviceToken.UserID, deviceToken.DeviceID, true).
		First(&existingDevice).Error == gorm.ErrRecordNotFound

	if isNewDevice {
		// Count current active devices for this user
		var activeDeviceCount int64
		if err := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
			Where("user_id = ? AND is_active = ?", deviceToken.UserID, true).
			Count(&activeDeviceCount).Error; err != nil {
			log.Error("Failed to count active devices", "error", err)
			return err
		}

		if activeDeviceCount >= MaxDevicesPerUser {
			return errorx.ErrDeviceLimitExceeded
		}
	}

	// Start a transaction
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// First, deactivate any existing token for this user/device combination
		if err := tx.Model(&uModel.UserDeviceToken{}).
			Where("user_id = ? AND device_id = ? AND is_active = ?", deviceToken.UserID, deviceToken.DeviceID, true).
			Update("is_active", false).Error; err != nil {
			log.Error("Failed to deactivate existing device token", "error", err)
			return err
		}

		// Also deactivate any existing token with the same FCM token (in case device_id changed)
		if deviceToken.Token != "" {
			if err := tx.Model(&uModel.UserDeviceToken{}).
				Where("user_id = ? AND token = ? AND is_active = ?", deviceToken.UserID, deviceToken.Token, true).
				Update("is_active", false).Error; err != nil {
				log.Error("Failed to deactivate existing FCM token", "error", err)
				return err
			}
		}

		// Set token properties
		deviceToken.IsActive = true
		deviceToken.LastActiveAt = time.Now()
		deviceToken.CreatedAt = time.Now()

		// Create the new token
		if err := tx.Create(deviceToken).Error; err != nil {
			log.Error("Failed to register device token", "error", err)
			return err
		}

		log.Debugw("Device token registered successfully",
			"user_id", deviceToken.UserID,
			"device_id", deviceToken.DeviceID,
			"platform", deviceToken.Platform)
		return nil
	})
}

// UnregisterDeviceToken deletes a device token by device ID
func (r *DeviceRepository) UnregisterDeviceToken(c context.Context, userID int, deviceID string) error {
	log := logging.FromContext(c)

	result := r.db.WithContext(c).
		Where("user_id = ? AND device_id = ? AND is_active = ?", userID, deviceID, true).
		Delete(&uModel.UserDeviceToken{})

	if result.Error != nil {
		log.Error("Failed to delete device token", "error", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		log.Warn("No active device token found to delete", "user_id", userID, "device_id", deviceID)
		return fmt.Errorf("no active device token found for user_id: %d, device_id: %s", userID, deviceID)
	}

	log.Info("Device token deleted successfully", "user_id", userID, "device_id", deviceID)
	return nil
}

// UnregisterDeviceTokenByToken deactivates a device token by FCM token
func (r *DeviceRepository) UnregisterDeviceTokenByToken(c context.Context, userID int, token string) error {
	log := logging.FromContext(c)

	result := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("user_id = ? AND token = ? AND is_active = ?", userID, token, true).
		Update("is_active", false)

	if result.Error != nil {
		log.Error("Failed to unregister device token by token", "error", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		log.Warn("No active device token found to unregister by token", "user_id", userID)
		return fmt.Errorf("no active device token found for user_id: %d with provided token", userID)
	}

	log.Info("Device token unregistered by token successfully", "user_id", userID)
	return nil
}

// GetUserDeviceTokens retrieves all device tokens for a user (active and inactive)
func (r *DeviceRepository) GetUserDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error) {
	var tokens []*uModel.UserDeviceToken

	if err := r.db.WithContext(c).Where("user_id = ?", userID).
		Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}

	return tokens, nil
}

// GetActiveDeviceTokens retrieves only active device tokens for a user
func (r *DeviceRepository) GetActiveDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error) {
	var tokens []*uModel.UserDeviceToken

	if err := r.db.WithContext(c).Where("user_id = ? AND is_active = ?", userID, true).
		Order("last_active_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}

	return tokens, nil
}

// UpdateDeviceTokenActivity updates the last active timestamp for a device token
func (r *DeviceRepository) UpdateDeviceTokenActivity(c context.Context, userID int, deviceID string) error {
	return r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("user_id = ? AND device_id = ? AND is_active = ?", userID, deviceID, true).
		Update("last_active_at", time.Now()).Error
}

// GetActiveDeviceCount returns the count of active devices for a user
func (r *DeviceRepository) GetActiveDeviceCount(c context.Context, userID int) (int64, error) {
	var count int64
	err := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error
	return count, err
}

// CleanupInactiveTokens removes tokens that haven't been active for the specified number of days
func (r *DeviceRepository) CleanupInactiveTokens(c context.Context, inactiveDays int) error {
	log := logging.FromContext(c)

	cutoffDate := time.Now().AddDate(0, 0, -inactiveDays)

	result := r.db.WithContext(c).
		Where("last_active_at < ? OR (last_active_at IS NULL AND created_at < ?)", cutoffDate, cutoffDate).
		Delete(&uModel.UserDeviceToken{})

	if result.Error != nil {
		log.Error("Failed to cleanup inactive tokens", "error", result.Error)
		return result.Error
	}

	log.Info("Cleaned up inactive device tokens", "count", result.RowsAffected, "cutoff_days", inactiveDays)
	return nil
}
