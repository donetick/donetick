package migrations

import (
	"context"
	"time"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type Migration struct {
	// DB Representation for migration table, mainly used for tracking the migration status.
	ID          string    `json:"id" gorm:"primary_key"`
	Description string    `json:"description" gorm:"column:description"`
	AppliedAt   time.Time `json:"appliedAt" gorm:"column:applied_at"`
}

type MigrationScript interface {
	// Actual migration script interface which needs to be implemented by each migration script.
	Description() string
	ID() string
	Up(ctx context.Context, db *gorm.DB) error
	Down(ctx context.Context, db *gorm.DB) error
}

var registry []MigrationScript

// Register a migration
func Register(migration MigrationScript) {
	registry = append(registry, migration)
}

// Sort the migrations by ID
func sortMigrations(migrations []MigrationScript) []MigrationScript {
	for i := 0; i < len(migrations); i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].ID() > migrations[j].ID() {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}
	return migrations
}

// Run pending migrations
func Run(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)
	// Confirm the migrations table exists :)
	if err := db.AutoMigrate(&Migration{}); err != nil {
		return err
	}

	// Sort the registry by ID
	registry = sortMigrations(registry)
	var successfulCount int
	var skippedCount int
	for _, migration := range registry {
		// Check if migration is already applied
		var count int64
		db.Model(&Migration{}).Where("id = ?", migration.ID()).Count(&count)
		if count > 0 {
			skippedCount++
			log.Debugf("Skipping migration %s as it is already applied", migration.ID())
			continue
		}

		// Run the migration
		log.Infof("Applying migration: %s", migration.ID())
		if err := migration.Up(ctx, db); err != nil {

			log.Errorf("Failed to apply migration %s: %s", migration.ID(), err)
			if err := migration.Down(ctx, db); err != nil {
				log.Errorf("Failed to rollback migration %s: %s\n", migration.ID(), err)
			}
			return err
		}

		// Record the migration
		record := Migration{
			ID:          migration.ID(),
			Description: migration.Description(),
			AppliedAt:   time.Now().UTC(),
		}

		if err := db.Create(&record).Error; err != nil {
			log.Errorf("Failed to record migration %s: %s\n", migration.ID(), err)
			return err
		}

		successfulCount++
	}

	if len(registry) == 0 {
		log.Info("Migratons: No pending migrations")
	} else {
		var failedCount = len(registry) - successfulCount - skippedCount
		log.Infof("Migrations: %d successful, %d failed, %d skipped, %d total in registry", successfulCount, failedCount, skippedCount, len(registry))
	}
	return nil
}
