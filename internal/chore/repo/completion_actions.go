package chore

import (
	"context"
	"fmt"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	tModel "donetick.com/core/internal/thing/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Completion and its Thing changes share a transaction. Skipping never calls this.
func (r *ChoreRepository) applyCompletionActions(ctx context.Context, tx *gorm.DB, chore *chModel.Chore) error {
	changed := make(map[int]string)
	for _, action := range chore.CompletionActions {
		var thing tModel.Thing
		query := tx
		if r.dbType == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND user_id = ?", action.ThingID, chore.CreatedBy).First(&thing).Error; err != nil {
			return fmt.Errorf("Completion action Thing is unavailable: %w", err)
		}
		state, err := action.NextState(&thing)
		if err != nil {
			return err
		}
		changed[thing.ID] = state
		now := time.Now().UTC()
		if err := tx.Model(&thing).Updates(map[string]interface{}{"state": state, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&tModel.ThingHistory{ThingID: thing.ID, State: state, CreatedAt: &now, UpdatedAt: &now}).Error; err != nil {
			return err
		}
	}
	// Evaluate the final value once after all actions, without recursively completing tasks.
	for thingID, state := range changed {
		var links []tModel.ThingChore
		if err := tx.Where("thing_id = ?", thingID).Find(&links).Error; err != nil {
			return err
		}
		for _, link := range links {
			if !link.Matches(state) {
				continue
			}
			var target chModel.Chore
			if err := tx.First(&target, link.ChoreID).Error; err != nil {
				return err
			}
			if target.IsActive && target.NextDueDate != nil {
				continue
			}
			version, err := r.nextSyncVersionWithDB(ctx, tx, target.CircleID)
			if err != nil {
				return err
			}
			if err := tx.Model(&target).Updates(map[string]interface{}{"next_due_date": time.Now().UTC(), "is_active": true, "sync_version": version}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
