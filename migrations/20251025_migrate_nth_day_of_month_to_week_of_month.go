package migrations

import (
	"context"
	"encoding/json"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateNthDayOfMonthToWeekOfMonth20251025 struct{}

func (m MigrateNthDayOfMonthToWeekOfMonth20251025) ID() string {
	return "20251025_migrate_nth_day_of_month_to_week_of_month"
}

func (m MigrateNthDayOfMonthToWeekOfMonth20251025) Description() string {
	return `Migrate weekPattern from legacy 'nth_day_of_month' to 'week_of_month' in frequency_meta_v2 JSON column.`
}

func (m MigrateNthDayOfMonthToWeekOfMonth20251025) Down(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type Chore struct {
		ID                  int     `gorm:"column:id;primary_key"`
		FrequencyMetadataV2 *string `gorm:"column:frequency_meta_v2"`
	}

	// Check if the frequency_meta_v2 column exists
	if !db.Migrator().HasColumn(&Chore{}, "frequency_meta_v2") {
		log.Info("Column frequency_meta_v2 does not exist, skipping rollback")
		return nil
	}

	var chores []Chore
	if err := db.Table("chores").Select("id, frequency_meta_v2").Find(&chores).Error; err != nil {
		log.Errorf("Failed to fetch chores: %v", err)
		return err
	}

	for _, chore := range chores {
		if chore.FrequencyMetadataV2 == nil || *chore.FrequencyMetadataV2 == "" {
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(*chore.FrequencyMetadataV2), &metadata); err != nil {
			log.Warnf("Chore %d: failed to parse frequency_meta_v2: %v", chore.ID, err)
			continue
		}

		// Check if weekPattern exists and is "week_of_month"
		weekPattern, exists := metadata["weekPattern"]
		if !exists {
			continue
		}

		if weekPatternStr, ok := weekPattern.(string); ok && weekPatternStr == "week_of_month" {
			// Rollback to nth_day_of_month
			metadata["weekPattern"] = "nth_day_of_month"

			updatedJSON, err := json.Marshal(metadata)
			if err != nil {
				log.Warnf("Chore %d: failed to marshal updated frequency_meta_v2: %v", chore.ID, err)
				continue
			}

			updatedJSONStr := string(updatedJSON)
			if err := db.Table("chores").Where("id = ?", chore.ID).Update("frequency_meta_v2", updatedJSONStr).Error; err != nil {
				log.Warnf("Chore %d: failed to update frequency_meta_v2: %v", chore.ID, err)
				continue
			}

			log.Infof("Chore %d: rolled back frequency_meta_v2", chore.ID)
		}
	}
	return nil
}

func (m MigrateNthDayOfMonthToWeekOfMonth20251025) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type Chore struct {
		ID                  int     `gorm:"column:id;primary_key"`
		FrequencyMetadataV2 *string `gorm:"column:frequency_meta_v2"`
	}

	// Check if the frequency_meta_v2 column exists
	if !db.Migrator().HasColumn(&Chore{}, "frequency_meta_v2") {
		log.Info("Column frequency_meta_v2 does not exist, skipping migration")
		return nil
	}

	var chores []Chore
	if err := db.Table("chores").Select("id, frequency_meta_v2").Find(&chores).Error; err != nil {
		log.Errorf("Failed to fetch chores: %v", err)
		return err
	}

	for _, chore := range chores {
		if chore.FrequencyMetadataV2 == nil || *chore.FrequencyMetadataV2 == "" {
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(*chore.FrequencyMetadataV2), &metadata); err != nil {
			log.Warnf("Chore %d: failed to parse frequency_meta_v2: %v", chore.ID, err)
			continue
		}

		// Check if weekPattern exists and is "nth_day_of_month"
		weekPattern, exists := metadata["weekPattern"]
		if !exists {
			continue
		}

		if weekPatternStr, ok := weekPattern.(string); ok && weekPatternStr == "nth_day_of_month" {
			// Update to week_of_month
			metadata["weekPattern"] = "week_of_month"

			updatedJSON, err := json.Marshal(metadata)
			if err != nil {
				log.Warnf("Chore %d: failed to marshal updated frequency_meta_v2: %v", chore.ID, err)
				continue
			}

			updatedJSONStr := string(updatedJSON)
			if err := db.Table("chores").Where("id = ?", chore.ID).Update("frequency_meta_v2", updatedJSONStr).Error; err != nil {
				log.Warnf("Chore %d: failed to update frequency_meta_v2: %v", chore.ID, err)
				continue
			}

			log.Infof("Chore %d: migrated frequency_meta_v2", chore.ID)
		}
	}
	return nil
}

func init() {
	Register(MigrateNthDayOfMonthToWeekOfMonth20251025{})
}
