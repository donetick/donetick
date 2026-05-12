package migrations

import (
	"context"

	"donetick.com/core/internal/sync/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type BackfillSyncVersionsForExistingData20260511 struct{}

func (m BackfillSyncVersionsForExistingData20260511) ID() string {
	return "20260511_backfill_sync_versions_for_existing_data"
}

func (m BackfillSyncVersionsForExistingData20260511) Description() string {
	return `Backfill sync_version for legacy chores, chore histories, and tombstones with zero values; align sync cursor per circle.`
}

func (m BackfillSyncVersionsForExistingData20260511) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: irreversible data migration
	return nil
}

func maxInt64(values ...int64) int64 {
	maxValue := int64(0)
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func (m BackfillSyncVersionsForExistingData20260511) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type circleRow struct {
		CircleID int `gorm:"column:circle_id"`
	}
	type idRow struct {
		ID int `gorm:"column:id"`
	}

	var circles []circleRow
	if err := db.WithContext(ctx).Raw(`
		SELECT circle_id FROM chores
		UNION
		SELECT circle_id FROM tombstones
		UNION
		SELECT circle_id FROM sync_cursors
	`).Scan(&circles).Error; err != nil {
		log.Errorf("Failed to load circles for sync backfill: %v", err)
		return err
	}

	totalChoresUpdated := 0
	totalHistoriesUpdated := 0
	totalTombstonesUpdated := 0

	for _, circle := range circles {
		if circle.CircleID == 0 {
			continue
		}

		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var maxChoreVersion int64
			if err := tx.Raw(`SELECT COALESCE(MAX(sync_version), 0) FROM chores WHERE circle_id = ?`, circle.CircleID).Scan(&maxChoreVersion).Error; err != nil {
				return err
			}

			var maxHistoryVersion int64
			if err := tx.Raw(`
				SELECT COALESCE(MAX(chore_histories.sync_version), 0)
				FROM chore_histories
				JOIN chores ON chores.id = chore_histories.chore_id
				WHERE chores.circle_id = ?
			`, circle.CircleID).Scan(&maxHistoryVersion).Error; err != nil {
				return err
			}

			var maxTombstoneVersion int64
			if err := tx.Raw(`SELECT COALESCE(MAX(sync_version), 0) FROM tombstones WHERE circle_id = ?`, circle.CircleID).Scan(&maxTombstoneVersion).Error; err != nil {
				return err
			}

			var maxCursorVersion int64
			if err := tx.Raw(`
				SELECT COALESCE(MAX(max_version), 0)
				FROM sync_cursors
				WHERE circle_id = ?
			`, circle.CircleID).Scan(&maxCursorVersion).Error; err != nil {
				return err
			}

			currentVersion := maxInt64(maxChoreVersion, maxHistoryVersion, maxTombstoneVersion, maxCursorVersion)

			var choreIDs []idRow
			if err := tx.Raw(`
				SELECT id
				FROM chores
				WHERE circle_id = ? AND sync_version = 0
				ORDER BY created_at ASC, id ASC
			`, circle.CircleID).Scan(&choreIDs).Error; err != nil {
				return err
			}
			for _, chore := range choreIDs {
				currentVersion++
				if err := tx.Table("chores").Where("id = ?", chore.ID).Update("sync_version", currentVersion).Error; err != nil {
					return err
				}
			}

			var historyIDs []idRow
			if err := tx.Raw(`
				SELECT chore_histories.id
				FROM chore_histories
				JOIN chores ON chores.id = chore_histories.chore_id
				WHERE chores.circle_id = ? AND chore_histories.sync_version = 0
				ORDER BY chore_histories.created_at ASC, chore_histories.id ASC
			`, circle.CircleID).Scan(&historyIDs).Error; err != nil {
				return err
			}
			for _, history := range historyIDs {
				currentVersion++
				if err := tx.Table("chore_histories").Where("id = ?", history.ID).Update("sync_version", currentVersion).Error; err != nil {
					return err
				}
			}

			var tombstoneIDs []idRow
			if err := tx.Raw(`
				SELECT id
				FROM tombstones
				WHERE circle_id = ? AND sync_version = 0
				ORDER BY id ASC
			`, circle.CircleID).Scan(&tombstoneIDs).Error; err != nil {
				return err
			}
			for _, tombstone := range tombstoneIDs {
				currentVersion++
				if err := tx.Table("tombstones").Where("id = ?", tombstone.ID).Update("sync_version", currentVersion).Error; err != nil {
					return err
				}
			}

			if err := tx.Exec(`
				INSERT INTO sync_cursors (circle_id, entity_type, max_version)
				VALUES (?, ?, ?)
				ON CONFLICT (circle_id, entity_type)
				DO UPDATE SET max_version =
					CASE
						WHEN sync_cursors.max_version > excluded.max_version THEN sync_cursors.max_version
						ELSE excluded.max_version
					END
			`, circle.CircleID, model.EntityTypeChore, currentVersion).Error; err != nil {
				return err
			}

			totalChoresUpdated += len(choreIDs)
			totalHistoriesUpdated += len(historyIDs)
			totalTombstonesUpdated += len(tombstoneIDs)

			if len(choreIDs)+len(historyIDs)+len(tombstoneIDs) > 0 {
				log.Infof("Sync backfill circle=%d chores=%d histories=%d tombstones=%d maxVersion=%d", circle.CircleID, len(choreIDs), len(historyIDs), len(tombstoneIDs), currentVersion)
			}

			return nil
		})
		if err != nil {
			log.Errorf("Failed sync backfill for circle %d: %v", circle.CircleID, err)
			return err
		}
	}

	log.Infof("Sync backfill completed: chores=%d histories=%d tombstones=%d", totalChoresUpdated, totalHistoriesUpdated, totalTombstonesUpdated)
	return nil
}

func init() {
	Register(BackfillSyncVersionsForExistingData20260511{})
}
