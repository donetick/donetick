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

// InactiveDeviceRetentionDays is how long a device registration survives
// without the client checking in before it is deleted.
//
// Clients refresh last_active_at on every launch (either by re-registering a
// rotated token or via the lighter-weight activity heartbeat), so this only
// catches installs that are genuinely gone. Kept well above a typical holiday
// so an unused-but-alive phone is not dropped.
const InactiveDeviceRetentionDays = 90

type IDeviceRepository interface {
	RegisterDeviceToken(c context.Context, deviceToken *uModel.UserDeviceToken) error
	UnregisterDeviceToken(c context.Context, userID int, deviceID string) error
	UnregisterDeviceTokenByToken(c context.Context, userID int, token string) error
	DeactivateTokens(c context.Context, tokens []string) (int64, error)
	GetUserDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	GetActiveDeviceTokens(c context.Context, userID int) ([]*uModel.UserDeviceToken, error)
	GetActiveDeviceCount(c context.Context, userID int) (int64, error)
	UpdateDeviceTokenActivity(c context.Context, userID int, deviceID string) error
	CleanupInactiveTokens(c context.Context, inactiveDays int) error
	GetActiveDeviceByDeviceID(c context.Context, userID int, deviceID string) (*uModel.UserDeviceToken, error)
	GetActiveDeviceByToken(c context.Context, userID int, token string) (*uModel.UserDeviceToken, error)
	UpdateBridgeSyncStatus(c context.Context, id int, bridgeDeviceID *string, status string) error
	GetDevicesPendingBridgeSync(c context.Context, limit int) ([]*uModel.UserDeviceToken, error)
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
		if r.dbType == "postgres" {
			// Lock a row that always exists for this user. Locking only the current
			// device-token rows does not serialize registrations when the result is
			// empty and permits concurrent requests to exceed the device limit.
			var user uModel.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id").First(&user, deviceToken.UserID).Error; err != nil {
				log.Error("Failed to lock user for device registration", "error", err)
				return err
			}
		}

		var activeTokens []uModel.UserDeviceToken
		if err := tx.Where("user_id = ? AND is_active = ?", deviceToken.UserID, true).
			Find(&activeTokens).Error; err != nil {
			log.Error("Failed to load active device tokens", "error", err)
			return err
		}

		// Re-registering an active device does not consume another slot. Neither
		// does moving an existing FCM token to a new device ID, because that token's
		// old row is deactivated below in the same transaction.
		requiresAdditionalSlot := true
		for _, t := range activeTokens {
			if t.DeviceID == deviceToken.DeviceID || (deviceToken.Token != "" && t.Token == deviceToken.Token) {
				requiresAdditionalSlot = false
				break
			}
		}

		if requiresAdditionalSlot && len(activeTokens) >= MaxDevicesPerUser {
			return errorx.ErrDeviceLimitExceeded
		}

		// Deactivate every other active row carrying the same FCM token, for any
		// user. An FCM token identifies one app install on one physical device,
		// so at most one active row may own it.
		//
		// Scoping this to the registering user (as it previously was) left a
		// cross-account delivery leak: if user A never unregistered cleanly --
		// session expired, app force-deleted, logout while offline -- their row
		// kept this device's token. When user B then signed in on the same phone,
		// both rows were active with the same token and user B's device received
		// user A's chore notifications. Clearing by token regardless of user_id
		// also covers the same-user cases this already handled: a reinstall that
		// produced a new device_id (iOS identifierForVendor reset) reusing the
		// token, and a restore onto different hardware.
		if deviceToken.Token != "" {
			if err := tx.Model(&uModel.UserDeviceToken{}).
				Where("token = ? AND is_active = ? AND NOT (user_id = ? AND device_id = ?)",
					deviceToken.Token, true, deviceToken.UserID, deviceToken.DeviceID).
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

// DeactivateTokens marks the given FCM tokens inactive across all users and
// returns how many rows changed.
//
// Called after FCM reports a token as permanently invalid (UNREGISTERED /
// INVALID_ARGUMENT / SENDER_ID_MISMATCH). Without this, dead tokens stayed
// active forever and consumed the MaxDevicesPerUser budget, so a user who had
// reinstalled the app a few times hit ErrDeviceLimitExceeded on a device they
// were actively using. Deactivates rather than deletes so the row stays
// visible in the user's device list until the inactivity cleanup removes it.
func (r *DeviceRepository) DeactivateTokens(c context.Context, tokens []string) (int64, error) {
	if len(tokens) == 0 {
		return 0, nil
	}

	result := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("token IN ? AND is_active = ?", tokens, true).
		Update("is_active", false)

	return result.RowsAffected, result.Error
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

// UpdateDeviceTokenActivity updates the last active timestamp for a device
// token, and reports errorx.ErrDeviceNotFound when the user has no active row
// for that device.
//
// The not-found signal matters: the client uses this as a cheap launch-time
// heartbeat instead of re-POSTing an unchanged token, so it needs to be able
// to tell "noted" from "your registration is gone, register again" (the row
// may have been pruned as dead, removed from another device's settings, or
// cleaned up for inactivity).
func (r *DeviceRepository) UpdateDeviceTokenActivity(c context.Context, userID int, deviceID string) error {
	result := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("user_id = ? AND device_id = ? AND is_active = ?", userID, deviceID, true).
		Update("last_active_at", time.Now().UTC())

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorx.ErrDeviceNotFound
	}
	return nil
}

// GetActiveDeviceCount returns the count of active devices for a user
func (r *DeviceRepository) GetActiveDeviceCount(c context.Context, userID int) (int64, error) {
	var count int64
	err := r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error
	return count, err
}

// GetActiveDeviceByDeviceID returns the active device token row for a
// user/deviceId pair, or nil if none exists. Used before unregistering
// locally so the caller can still deactivate the corresponding Bridge
// device afterward.
func (r *DeviceRepository) GetActiveDeviceByDeviceID(c context.Context, userID int, deviceID string) (*uModel.UserDeviceToken, error) {
	var token uModel.UserDeviceToken
	err := r.db.WithContext(c).
		Where("user_id = ? AND device_id = ? AND is_active = ?", userID, deviceID, true).
		First(&token).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetActiveDeviceByToken returns the active device token row for a
// user/FCM-token pair, or nil if none exists. See GetActiveDeviceByDeviceID.
func (r *DeviceRepository) GetActiveDeviceByToken(c context.Context, userID int, token string) (*uModel.UserDeviceToken, error) {
	var t uModel.UserDeviceToken
	err := r.db.WithContext(c).
		Where("user_id = ? AND token = ? AND is_active = ?", userID, token, true).
		First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateBridgeSyncStatus records the outcome of a Bridge device
// registration attempt. bridgeDeviceID is nil when registration failed
// (status carries the sanitized failure category instead); it is non-nil
// only on success. Never receives or stores a raw FCM token.
func (r *DeviceRepository) UpdateBridgeSyncStatus(c context.Context, id int, bridgeDeviceID *string, status string) error {
	updates := map[string]interface{}{"bridge_sync_status": status}
	if bridgeDeviceID != nil {
		now := time.Now().UTC()
		updates["bridge_device_id"] = *bridgeDeviceID
		updates["bridge_registered_at"] = now
	}
	return r.db.WithContext(c).Model(&uModel.UserDeviceToken{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// GetDevicesPendingBridgeSync returns up to limit active devices that have
// never been successfully registered with Bridge (bridge_device_id is
// still empty). Used by the opportunistic, content-free retry job.
func (r *DeviceRepository) GetDevicesPendingBridgeSync(c context.Context, limit int) ([]*uModel.UserDeviceToken, error) {
	var tokens []*uModel.UserDeviceToken
	err := r.db.WithContext(c).
		Where("is_active = ? AND (bridge_device_id IS NULL OR bridge_device_id = ?)", true, "").
		Order("created_at ASC").
		Limit(limit).
		Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
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
