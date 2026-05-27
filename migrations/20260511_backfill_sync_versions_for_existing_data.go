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

			// Batch update chores with sync_version = 0 using window functions
			var choreCount int64
			if err := tx.Raw(`
				SELECT COUNT(*) FROM chores WHERE circle_id = ? AND sync_version = 0
			`, circle.CircleID).Scan(&choreCount).Error; err != nil {
				return err
			}

			if choreCount > 0 {
				if err := tx.Exec(`
					WITH numbered_chores AS (
						SELECT id, ROW_NUMBER() OVER (ORDER BY created_at ASC, id ASC) AS rn
						FROM chores
						WHERE circle_id = ? AND sync_version = 0
					)
					UPDATE chores
					SET sync_version = ? + nc.rn
					FROM numbered_chores nc
					WHERE chores.id = nc.id
				`, circle.CircleID, currentVersion).Error; err != nil {
					return err
				}
				currentVersion += choreCount
			}

			// Batch update chore histories with sync_version = 0 using window functions
			var historyCount int64
			if err := tx.Raw(`
				SELECT COUNT(chore_histories.id)
				FROM chore_histories
				JOIN chores ON chores.id = chore_histories.chore_id
				WHERE chores.circle_id = ? AND chore_histories.sync_version = 0
			`, circle.CircleID).Scan(&historyCount).Error; err != nil {
				return err
			}

			if historyCount > 0 {
				if err := tx.Exec(`
					WITH numbered_histories AS (
						SELECT chore_histories.id, ROW_NUMBER() OVER (ORDER BY chore_histories.created_at ASC, chore_histories.id ASC) AS rn
						FROM chore_histories
						JOIN chores ON chores.id = chore_histories.chore_id
						WHERE chores.circle_id = ? AND chore_histories.sync_version = 0
					)
					UPDATE chore_histories
					SET sync_version = ? + nh.rn
					FROM numbered_histories nh
					WHERE chore_histories.id = nh.id
				`, circle.CircleID, currentVersion).Error; err != nil {
					return err
				}
				currentVersion += historyCount
			}

			// Batch update tombstones with sync_version = 0 using window functions
			var tombstoneCount int64
			if err := tx.Raw(`
				SELECT COUNT(*) FROM tombstones WHERE circle_id = ? AND sync_version = 0
			`, circle.CircleID).Scan(&tombstoneCount).Error; err != nil {
				return err
			}

			if tombstoneCount > 0 {
				if err := tx.Exec(`
					WITH numbered_tombstones AS (
						SELECT id, ROW_NUMBER() OVER (ORDER BY id ASC) AS rn
						FROM tombstones
						WHERE circle_id = ? AND sync_version = 0
					)
					UPDATE tombstones
					SET sync_version = ? + nt.rn
					FROM numbered_tombstones nt
					WHERE tombstones.id = nt.id
				`, circle.CircleID, currentVersion).Error; err != nil {
					return err
				}
				currentVersion += tombstoneCount
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

			totalChoresUpdated += int(choreCount)
			totalHistoriesUpdated += int(historyCount)
			totalTombstonesUpdated += int(tombstoneCount)

			if choreCount+historyCount+tombstoneCount > 0 {
				log.Infof("Sync backfill circle=%d chores=%d histories=%d tombstones=%d maxVersion=%d", circle.CircleID, choreCount, historyCount, tombstoneCount, currentVersion)
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
