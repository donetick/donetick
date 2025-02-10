package migrations

import (
	"context"
	"fmt"

	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateChatIdToNotificationTarget20241212 struct{}

func (m MigrateChatIdToNotificationTarget20241212) ID() string {
	return "20241212_migrate_chat_id_to_notification_target"
}

func (m MigrateChatIdToNotificationTarget20241212) Description() string {
	return `Migrate Chat ID to notification target to support multiple notification targets and platform other than telegram`
}

func (m MigrateChatIdToNotificationTarget20241212) Down(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (m MigrateChatIdToNotificationTarget20241212) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)
	// if UserNotificationTarget table already exists drop it and recreate it:
	if err := db.Migrator().DropTable(&uModel.UserNotificationTarget{}); err != nil {
		log.Errorf("Failed to drop user_notification_targets table: %v", err)
	}

	// Create UserNotificationTarget table
	if err := db.AutoMigrate(&uModel.UserNotificationTarget{}); err != nil {
		log.Errorf("Failed to create user_notification_targets table: %v", err)
	}

	// Start a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Get All Users
		var users []uModel.User
		if err := tx.Table("users").Find(&users).Error; err != nil {
			log.Errorf("Failed to fetch users: %v", err)
		}

		var notificationTargets []uModel.UserNotificationTarget
		for _, user := range users {
			if user.ChatID == 0 {
				continue
			}
			notificationTargets = append(notificationTargets, uModel.UserNotificationTarget{
				UserID:   user.ID,
				TargetID: fmt.Sprint(user.ChatID),
				Type:     nModel.NotificationPlatformTelegram,
			})
		}

		// Insert all notification targets
		if err := tx.Table("user_notification_targets").Create(&notificationTargets).Error; err != nil {
			log.Errorf("Failed to insert notification targets: %v", err)
		}

		return nil
	})
}

// Register this migration
func init() {
	Register(MigrateChatIdToNotificationTarget20241212{})
}
