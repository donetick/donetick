package migrations

import (
	"context"

	"gorm.io/gorm"
)

type BackfillLabelCircleID20250830 struct{}

func (m BackfillLabelCircleID20250830) ID() string {
	return "20250830_backfill_label_circle_id"
}

func (m BackfillLabelCircleID20250830) Description() string {
	return `Populate circle_id for labels so every circle member can use shared labels`
}

func (m BackfillLabelCircleID20250830) Down(ctx context.Context, db *gorm.DB) error {
	// no-op: we don't want to unset circle information once it is assigned
	return nil
}

func (m BackfillLabelCircleID20250830) Up(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Exec(`
		UPDATE labels
		SET circle_id = (
			SELECT users.circle_id
			FROM users
			WHERE users.id = labels.created_by
		)
		WHERE circle_id IS NULL
		  AND EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = labels.created_by
		  )
	`).Error
}

func init() {
	Register(BackfillLabelCircleID20250830{})
}
