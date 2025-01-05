package migrations

import (
	"context"

	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateNotificationTarget20250105 struct{}

func (m MigrateNotificationTarget20250105) ID() string {
	return "20250105_migrate_notification_target_to_notification_target_v2"
}

func (m MigrateNotificationTarget20250105) Description() string {
	return `Migrate notification target to notification target v2 table, Allows setting webhook as a notification target`
}

func (m MigrateNotificationTarget20250105) Down(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (m MigrateNotificationTarget20250105) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	// Create additional columns for webhook url and method
	if err := db.AutoMigrate(&uModel.UserNotificationTarget{}); err != nil {
		log.Errorf("Failed to update user_notification_targets table: %v", err)
	}

	// Similarly for the notifications table
	if err := db.AutoMigrate(&nModel.Notification{}); err != nil {
		log.Errorf("Failed to update notifications table: %v", err)
	}

	return nil
}

// Register this migration
func init() {
	Register(MigrateNotificationTarget20250105{})
}
