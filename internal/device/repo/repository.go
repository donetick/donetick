package device

import (
	"context"
	"fmt"
	"time"

	config "donetick.com/core/config"
	errorx "donetick.com/core/internal/error"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	db     *gorm.DB
	dbType string
}

func NewDeviceRepository(db *gorm.DB, cfg *config.Config) *DeviceRepository {
	return &DeviceRepository{db: db, dbType: cfg.Database.Type}
}

// RegisterDeviceToken registers or updates a device token for a user.
//
// (user_id, device_id) is unique at the DB level, so re-registering a known
// device must be an upsert rather than deactivate-then-insert: inserting a new
// row while the old one is merely deactivated collides with that unique index.
func (r *DeviceRepository) RegisterDeviceToken(c context.Context, deviceToken *uModel.UserDeviceToken) error {
	log := logging.FromContext(c)

	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		lockedActive := tx.Model(&uModel.UserDeviceToken{})
		if r.dbType == "postgres" {
			lockedActive = lockedActive.Clauses(clause.Locking{Strength: "UPDATE"})
		}

		// Lock the user's active devices for the duration of the transaction so
		// concurrent registrations for the same user can't both pass the device
		// limit check before either commits.
		var activeTokens []uModel.UserDeviceToken
		if err := lockedActive.
			Where("user_id = ? AND is_active = ?", deviceToken.UserID, true).
			Find(&activeTokens).Error; err != nil {
			log.Error("Failed to lock active device tokens", "error", err)
			return err
		}

		isNewDevice := true
		for _, t := range activeTokens {
			if t.DeviceID == deviceToken.DeviceID {
				isNewDevice = false
				break
			}
		}

		if isNewDevice && len(activeTokens) >= MaxDevicesPerUser {
			return errorx.ErrDeviceLimitExceeded
		}

		// Deactivate any other active token carrying the same FCM token (e.g. the
		// device reinstalled the app and got a new device_id but reused the token).
		if deviceToken.Token != "" {
			if err := tx.Model(&uModel.UserDeviceToken{}).
				Where("user_id = ? AND token = ? AND device_id <> ? AND is_active = ?",
					deviceToken.UserID, deviceToken.Token, deviceToken.DeviceID, true).
				Update("is_active", false).Error; err != nil {
				log.Error("Failed to deactivate existing FCM token", "error", err)
				return err
			}
		}

		deviceToken.IsActive = true
		deviceToken.LastActiveAt = time.Now().UTC()
		if deviceToken.CreatedAt.IsZero() {
			deviceToken.CreatedAt = time.Now().UTC()
		}

		// Upsert on (user_id, device_id): re-registering a known device updates
		// the existing row in place instead of inserting a colliding duplicate.
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"token", "platform", "app_version", "device_model", "is_active", "last_active_at",
			}),
		}).Create(deviceToken).Error; err != nil {
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
		Update("last_active_at", time.Now().UTC()).Error
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

	cutoffDate := time.Now().UTC().AddDate(0, 0, -inactiveDays)

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
