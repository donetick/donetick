package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateFixNotificationMetadataExperimentModal20241212 struct{}

func (m MigrateFixNotificationMetadataExperimentModal20241212) ID() string {
	return "20250314_fix_notification_metadata_experiment_modal"
}

func (m MigrateFixNotificationMetadataExperimentModal20241212) Description() string {
	return `Fix notification metadata for experiment modal, where notification metadata is a null string 'null' to empty json {}`
}

func (m MigrateFixNotificationMetadataExperimentModal20241212) Down(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (m MigrateFixNotificationMetadataExperimentModal20241212) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	// Start a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Update all chore where notification metadata is a null stirng 'null' to empty json {}:

		if err := tx.Table("chores").Where("notification_meta = ?", "null").Update("notification_meta", "{}").Error; err != nil {
			log.Errorf("Failed to update chores with null notification metadata: %v", err)
			return err
		}

		return nil
	})
}

// Register this migration
func init() {
	Register(MigrateFixNotificationMetadataExperimentModal20241212{})
}
