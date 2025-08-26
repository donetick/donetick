package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type SetExistingChoresPrivate20250825 struct{}

func (m SetExistingChoresPrivate20250825) ID() string {
	return "20250825_set_existing_chores_private"
}

func (m SetExistingChoresPrivate20250825) Description() string {
	return `Set isPrivate to true for all existing chores that have this field.`
}

func (m SetExistingChoresPrivate20250825) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: irreversible
	return nil
}

func (m SetExistingChoresPrivate20250825) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type Chore struct {
		ID        int   `gorm:"column:id;primary_key"`
		IsPrivate *bool `gorm:"column:is_private"`
	}

	// Check if the is_private column exists
	if !db.Migrator().HasColumn(&Chore{}, "is_private") {
		log.Info("Column is_private does not exist, skipping migration")
		return nil
	}

	var chores []Chore
	if err := db.Table("chores").Select("id, is_private").Find(&chores).Error; err != nil {
		log.Errorf("Failed to fetch chores: %v", err)
		return err
	}

	updatedCount := 0
	for _, chore := range chores {
		// Only update chores where is_private is null or false
		if chore.IsPrivate == nil || !*chore.IsPrivate {
			isPrivateTrue := true
			if err := db.Table("chores").Where("id = ?", chore.ID).Update("is_private", isPrivateTrue).Error; err != nil {
				log.Warnf("Chore %d: failed to update is_private: %v", chore.ID, err)
				continue
			}
			updatedCount++
		}
	}

	log.Infof("Successfully set is_private to true for %d chores", updatedCount)
	return nil
}

func init() {
	Register(SetExistingChoresPrivate20250825{})
}
