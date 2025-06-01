package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type RepurposeCompletedAtToPerformedAt20250528 struct{}

func (m RepurposeCompletedAtToPerformedAt20250528) ID() string {
	return "20250528_repurpose_completed_at_to_performed_at"
}

func (m RepurposeCompletedAtToPerformedAt20250528) Description() string {
	return "Repurpose completed_at to performed_at and update status logic"
}

func (m RepurposeCompletedAtToPerformedAt20250528) Down(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Check if completed_at column exists, if not add it
		if !tx.Migrator().HasColumn("chore_histories", "completed_at") {
			if err := tx.Exec("ALTER TABLE chore_histories ADD COLUMN completed_at DATETIME").Error; err != nil {
				log.Errorf("Failed to add completed_at column: %v", err)
				return err
			}
		}

		// Copy data back from performed_at to completed_at
		if err := tx.Exec("UPDATE chore_histories SET completed_at = performed_at").Error; err != nil {
			log.Errorf("Failed to copy performed_at to completed_at: %v", err)
			return err
		}

		// Reset status to 0 (pending)
		if err := tx.Exec("UPDATE chore_histories SET status = 0").Error; err != nil {
			log.Errorf("Failed to reset status: %v", err)
			return err
		}

		return nil
	})
}

func (m RepurposeCompletedAtToPerformedAt20250528) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Check if completed_at column exists
		hasCompletedAt := tx.Migrator().HasColumn("chore_histories", "completed_at")

		if hasCompletedAt {
			// Migration for existing installations with completed_at column
			log.Info("Found completed_at column, migrating data to performed_at")

			// Copy data from completed_at to performed_at and set status
			// Only update records where performed_at is NULL to avoid overwriting existing data
			if err := tx.Exec(`
				UPDATE chore_histories 
				SET performed_at = completed_at,
					status = CASE 
						WHEN completed_at IS NOT NULL THEN 1
						ELSE 2
					END
				WHERE performed_at IS NULL
			`).Error; err != nil {
				log.Errorf("Failed to migrate completed_at to performed_at: %v", err)
				return err
			}

			// For records that still have NULL performed_at, use updated_at as fallback
			if err := tx.Exec(`
				UPDATE chore_histories
				SET performed_at = updated_at
				WHERE performed_at IS NULL
			`).Error; err != nil {
				log.Errorf("Failed to set performed_at from updated_at: %v", err)
				return err
			}

			// Check again if completed_at column exists before dropping it
			if tx.Migrator().HasColumn("chore_histories", "completed_at") {
				tx.Exec("ALTER TABLE chore_histories DROP COLUMN completed_at")
				if err := tx.Error; err != nil {
					log.Errorf("Failed to drop completed_at column: %v", err)
					return err
				}
			} else {
				log.Warn("completed_at column does not exist, skipping drop")
			}

			log.Info("Successfully migrated from completed_at to performed_at")
		} else {
			// Fresh installation - no completed_at column exists
			log.Info("No completed_at column found, handling fresh installation")

			// For fresh installations, just ensure any records with NULL performed_at
			// use updated_at as a fallback and set appropriate status
			if err := tx.Exec(`
				UPDATE chore_histories
				SET performed_at = updated_at,
					status = CASE 
						WHEN status = 0 THEN 1
						ELSE status
					END
				WHERE performed_at IS NULL
			`).Error; err != nil {
				log.Errorf("Failed to set performed_at from updated_at in fresh installation: %v", err)
				return err
			}

			log.Info("Fresh installation migration completed")
		}

		return nil
	})
}

// Register this migration
func init() {
	Register(RepurposeCompletedAtToPerformedAt20250528{})
}
