package migrations

import (
	"context"
	"encoding/json"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateNotificationTemplates20250622 struct{}

func (m MigrateNotificationTemplates20250622) ID() string {
	return "20250622_migrate_notification_templates"
}

func (m MigrateNotificationTemplates20250622) Description() string {
	return `Migrate notification_meta_v2: move dueDate, nagging, predue flags into templates array.`
}

func (m MigrateNotificationTemplates20250622) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: irreversible
	return nil
}

func (m MigrateNotificationTemplates20250622) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type Chore struct {
		ID                 int     `gorm:"column:id;primary_key"`
		NotificationMetaV2 *string `gorm:"column:notification_meta_v2"`
	}

	var chores []Chore
	if err := db.Table("chores").Select("id, notification_meta_v2").Find(&chores).Error; err != nil {
		log.Errorf("Failed to fetch chores: %v", err)
		return err
	}

	for _, chore := range chores {
		if chore.NotificationMetaV2 == nil || *chore.NotificationMetaV2 == "" {
			continue
		}
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(*chore.NotificationMetaV2), &meta); err != nil {
			log.Warnf("Chore %d: failed to parse notification_meta_v2: %v", chore.ID, err)
			continue
		}

		// Only add templates if templates is missing or empty
		templates, hasTemplates := meta["templates"]
		templatesArr, _ := templates.([]interface{})
		if hasTemplates && len(templatesArr) > 0 {
			continue
		}

		var newTemplates []map[string]interface{}
		if due, ok := meta["dueDate"].(bool); ok && due {
			newTemplates = append(newTemplates, map[string]interface{}{"value": 0, "unit": "m"})
		}
		if nag, ok := meta["nagging"].(bool); ok && nag {
			newTemplates = append(newTemplates, map[string]interface{}{"value": 1, "unit": "d"})
			newTemplates = append(newTemplates, map[string]interface{}{"value": 2, "unit": "d"})
		}
		if predue, ok := meta["predue"].(bool); ok && predue {
			newTemplates = append(newTemplates, map[string]interface{}{"value": -3, "unit": "h"})
		}
		if len(newTemplates) == 0 {
			continue
		}
		meta["templates"] = newTemplates

		newMetaBytes, err := json.Marshal(meta)
		if err != nil {
			log.Warnf("Chore %d: failed to marshal new notification_meta_v2: %v", chore.ID, err)
			continue
		}
		newMetaStr := string(newMetaBytes)
		if err := db.Table("chores").Where("id = ?", chore.ID).Update("notification_meta_v2", newMetaStr).Error; err != nil {
			log.Warnf("Chore %d: failed to update notification_meta_v2: %v", chore.ID, err)
			continue
		}
		log.Infof("Chore %d: migrated notification_meta_v2", chore.ID)
	}
	return nil
}

func init() {
	Register(MigrateNotificationTemplates20250622{})
}
