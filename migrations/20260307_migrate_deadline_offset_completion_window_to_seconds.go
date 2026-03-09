package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateDeadlineOffsetCompletionWindowToSeconds20260307 struct{}

func (m MigrateDeadlineOffsetCompletionWindowToSeconds20260307) ID() string {
	return "20260307_migrate_deadline_offset_completion_window_to_seconds"
}

func (m MigrateDeadlineOffsetCompletionWindowToSeconds20260307) Description() string {
	return `Migrate deadline_offset and completion_window from hours to seconds (multiply by 3600).`
}

func (m MigrateDeadlineOffsetCompletionWindowToSeconds20260307) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: irreversible
	return nil
}

func (m MigrateDeadlineOffsetCompletionWindowToSeconds20260307) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	if err := db.Exec(`UPDATE chores SET deadline_offset = deadline_offset * 3600 WHERE deadline_offset IS NOT NULL`).Error; err != nil {
		log.Errorf("Failed to migrate deadline_offset to seconds: %v", err)
		return err
	}
	log.Infof("Migrated deadline_offset from hours to seconds")

	if err := db.Exec(`UPDATE chores SET completion_window = completion_window * 3600 WHERE completion_window IS NOT NULL`).Error; err != nil {
		log.Errorf("Failed to migrate completion_window to seconds: %v", err)
		return err
	}
	log.Infof("Migrated completion_window from hours to seconds")

	return nil
}

func init() {
	Register(MigrateDeadlineOffsetCompletionWindowToSeconds20260307{})
}
