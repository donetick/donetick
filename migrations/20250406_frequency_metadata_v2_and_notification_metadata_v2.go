package migrations

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406 struct{}

func (m MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406) ID() string {
	return "20250406_.frequency_metadata_v2_and_notification_metadata_v2_migration"
}

func (m MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406) Description() string {
	return `migrate frequency_metadata_v2 and notification_metadata_v2 in chores table.`
}

func (m MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406) Down(ctx context.Context, db *gorm.DB) error {
	return nil
}
func (m MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406) isApplicableScript(ctx context.Context, db *gorm.DB) bool {
	log := logging.FromContext(ctx)

	// if columns `frequency_meta_v2` or `notification_meta_v2` do not exist in the `chores` table, then this migration is not applicable.
	if db.Migrator().HasColumn("chores", "frequency_meta_v2") == false || db.Migrator().HasColumn("chores", "notification_meta_v2") == false {
		log.Infof("Skipping migration %s as required columns do not exist in the chores table", m.ID())
		return false
	}
	if db.Migrator().HasColumn("chores", "frequency_meta") == false && db.Migrator().HasColumn("chores", "notification_meta") == false {
		// if both `frequency_meta` and `notification_meta` do not exist, then this migration is not applicable.
		//  as mostly already applied before and column clean up happened
		log.Infof("Skipping migration %s as no legacy columns found in the chores table", m.ID())
		return false
	}

	return true

}
func (m MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	if !m.isApplicableScript(ctx, db) {
		log.Debugf("Migration %s is not applicable, skipping.", m.ID())
		return nil
	}
	// Start a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Update all chore where notification metadata is a null stirng 'null' to empty json {}:

		// if err := tx.Table("chores").Where("notification_meta = ?", "null").Update("notification_meta", "{}").Error; err != nil {
		// 	log.Errorf("Failed to update chores with null notification metadata: %v", err)
		// 	return err
		type ChoreRecord struct {
			ID                     int                   `gorm:"column:id;primary_key"` // Ensure we have the ID to update the record
			FrequencyMetadata      *string               `gorm:"column:frequency_meta"` // For backward compatibility, this will be removed in future
			FrequencyMetadataV2    *FrequencyMetadata    `gorm:"column:frequency_meta_v2;type:json"`
			NotificationMetadata   *string               `gorm:"column:notification_meta"` // For backward compatibility, this will be removed in future
			NotificationMetadataV2 *NotificationMetadata `gorm:"column:notification_meta_v2;type:json"`
		}
		var choreRecords []*ChoreRecord
		if err := tx.Table("chores").Select("id, notification_meta, notification_meta_v2, frequency_meta, frequency_meta_v2").Find(&choreRecords).Error; err != nil {
			log.Errorf("Failed to fetch chores for migration: %v", err)
		}
		if len(choreRecords) == 0 {
			log.Infof("No chores found for migration, skipping.")
			// No chores to migrate, return nil to indicate success
			return nil
		}

		for _, choreRecord := range choreRecords {

			// Handle FrequencyMetadataV2
			if choreRecord.FrequencyMetadata != nil && *choreRecord.FrequencyMetadata != "" {
				// Migrate from FrequencyMetadata to FrequencyMetadataV2
				var freqMeta FrequencyMetadata
				if err := json.Unmarshal([]byte(*choreRecord.FrequencyMetadata), &freqMeta); err != nil {
					log.Errorf("Failed to unmarshal frequency_metadata for chore ID %d: %v", choreRecord.ID, err)
					freqMeta = FrequencyMetadata{}
				}
				// Set the FrequencyMetadataV2
				if choreRecord.FrequencyMetadataV2 == nil {
					choreRecord.FrequencyMetadataV2 = &freqMeta
				}
			}
			// Handle NotificationMetadataV2
			if choreRecord.NotificationMetadata != nil && *choreRecord.NotificationMetadata != "" {
				// Migrate from NotificationMetadata to NotificationMetadataV2
				var notifMeta NotificationMetadata
				if err := json.Unmarshal([]byte(*choreRecord.NotificationMetadata), &notifMeta); err != nil {
					log.Errorf("Failed to unmarshal notification_metadata for chore ID %d: %v", choreRecord.ID, err)
					continue
				}
				// Set the NotificationMetadataV2
				if choreRecord.NotificationMetadataV2 == nil {
					choreRecord.NotificationMetadataV2 = &notifMeta
				}
			}

		}
		// Now update the database with the new values for FrequencyMetadataV2 and NotificationMetadataV2
		return tx.Table("chores").Save(choreRecords).Error

	})

}

type FrequencyMetadata struct {
	Days     []*string `json:"days,omitempty"`
	Months   []*string `json:"months,omitempty"`
	Unit     *string   `json:"unit,omitempty"`
	Time     string    `json:"time,omitempty"`
	Timezone string    `json:"timezone,omitempty"`
}

type NotificationMetadata struct {
	DueDate       bool   `json:"dueDate,omitempty"`
	Completion    bool   `json:"completion,omitempty"`
	Nagging       bool   `json:"nagging,omitempty"`
	PreDue        bool   `json:"predue,omitempty"`
	CircleGroup   bool   `json:"circleGroup,omitempty"`
	CircleGroupID *int64 `json:"circleGroupID,omitempty"`
}

// Implement driver.Valuer to convert the struct to JSON when saving to the database otherwise will
// get `error converting argument $12 type: unsupported type model.NotificationMetadata,  a struct` need
// the `Value()` and `Scan()` methods to store and retrieve the `NotificationMetadata` struct in the database as JSON.
func (n NotificationMetadata) Value() (driver.Value, error) {
	return json.Marshal(n)
}

func (n *NotificationMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, n)
}

func (f FrequencyMetadata) Value() (driver.Value, error) {
	return json.Marshal(f)
}

func (f *FrequencyMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, f)
}

// Register this migration
func init() {
	Register(MigrateFrequencyMetadataV2AndNotificationMetadataV2Migration20250406{})
}
