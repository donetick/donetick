package chore

import (
	"context"
	"errors"

	config "donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	lModel "donetick.com/core/internal/label/model"
	syncModel "donetick.com/core/internal/sync/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LabelRepository struct {
	db *gorm.DB
}

func NewLabelRepository(db *gorm.DB, cfg *config.Config) *LabelRepository {
	return &LabelRepository{db: db}
}

func (r *LabelRepository) GetUserLabels(ctx context.Context, userID int, circleID int) ([]*lModel.Label, error) {
	var labels []*lModel.Label
	if err := r.db.WithContext(ctx).Where("created_by = ? OR circle_id = ? ", userID, circleID).Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}

func (r *LabelRepository) CreateLabels(ctx context.Context, labels []*lModel.Label) error {
	if err := r.db.WithContext(ctx).Create(&labels).Error; err != nil {
		return err
	}
	return nil
}

func (r *LabelRepository) GetLabelsByIDs(ctx context.Context, ids []int) ([]*lModel.Label, error) {
	var labels []*lModel.Label
	if err := r.db.WithContext(ctx).Where("id IN (?)", ids).Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}

func (r *LabelRepository) isLabelsAssignableByUser(ctx context.Context, userID int, circleID int, toBeAdded []int, toBeRemoved []int) bool {
	// combine toBeAdded and toBeRemoved into a new slice
	labelIDs := []int(nil)
	labelIDs = append(labelIDs, toBeAdded...)
	labelIDs = append(labelIDs, toBeRemoved...)

	log := logging.FromContext(ctx)
	var count int64
	if err := r.db.WithContext(ctx).Model(&lModel.Label{}).Where("id IN (?) AND (created_by = ?  OR circle_id = ?) ", labelIDs, userID, circleID).Count(&count).Error; err != nil {
		log.Error(err)
		return false
	}
	return count == int64(len(labelIDs))
}

func (r *LabelRepository) AssignLabelsToChore(ctx context.Context, choreID int, userID int, circleID int, toBeAdded []int, toBeRemoved []int) error {
	if len(toBeAdded) < 1 && len(toBeRemoved) < 1 {
		return nil
	}
	if !r.isLabelsAssignableByUser(ctx, userID, circleID, toBeAdded, toBeRemoved) {
		return errors.New("labels are not assignable by user")
	}

	var choreLabels []*chModel.ChoreLabels
	for _, labelID := range toBeAdded {
		choreLabels = append(choreLabels, &chModel.ChoreLabels{
			ChoreID: choreID,
			LabelID: labelID,
			UserID:  userID,
		})
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(toBeRemoved) > 0 {
			if err := r.db.WithContext(ctx).Where("chore_id = ? AND user_id = ? AND label_id IN (?)", choreID, userID, toBeRemoved).Delete(&chModel.ChoreLabels{}).Error; err != nil {
				return err
			}
		}
		if len(toBeAdded) > 0 {
			if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "chore_id"}, {Name: "label_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).Create(&choreLabels).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *LabelRepository) DeassignLabelsFromChore(ctx context.Context, choreID int, userID int, labelIDs []int) error {
	if err := r.db.WithContext(ctx).Where("chore_id = ? AND user_id = ? AND label_id IN (?)", choreID, userID, labelIDs).Delete(&chModel.ChoreLabels{}).Error; err != nil {
		return err
	}
	return nil
}

// bumpChoreSyncVersions assigns a fresh sync version to every chore in choreIDs so
// clients syncing on sync_version pick up the label change. Versions are reserved as a
// contiguous range per circle and applied one per chore, so no two chores share a
// version (sharing one would let sync pagination skip records). Must run inside tx.
func (r *LabelRepository) bumpChoreSyncVersions(ctx context.Context, tx *gorm.DB, choreIDs []int) error {
	if len(choreIDs) < 1 {
		return nil
	}

	var chores []struct {
		ID       int
		CircleID int
	}
	if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Select("id, circle_id").Where("id IN (?)", choreIDs).Find(&chores).Error; err != nil {
		return err
	}

	choresByCircle := make(map[int][]int)
	for _, chore := range chores {
		choresByCircle[chore.CircleID] = append(choresByCircle[chore.CircleID], chore.ID)
	}

	for circleID, ids := range choresByCircle {
		var endVersion int64
		if err := tx.WithContext(ctx).Raw(`
			INSERT INTO sync_cursors (circle_id, entity_type, max_version)
			VALUES (?, ?, ?)
			ON CONFLICT (circle_id, entity_type) DO UPDATE SET max_version = sync_cursors.max_version + ?
			RETURNING max_version`,
			circleID, syncModel.EntityTypeChore, len(ids), len(ids),
		).Scan(&endVersion).Error; err != nil {
			return err
		}
		startVersion := endVersion - int64(len(ids)) + 1
		for i, choreID := range ids {
			if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", choreID).
				Update("sync_version", startVersion+int64(i)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// choreIDsWithLabels returns the distinct chores currently carrying any of labelIDs.
func (r *LabelRepository) choreIDsWithLabels(ctx context.Context, tx *gorm.DB, labelIDs []int) ([]int, error) {
	var choreIDs []int
	if err := tx.WithContext(ctx).Model(&chModel.ChoreLabels{}).
		Where("label_id IN (?)", labelIDs).
		Distinct().
		Pluck("chore_id", &choreIDs).Error; err != nil {
		return nil, err
	}
	return choreIDs, nil
}

func (r *LabelRepository) DeassignLabelFromAllChoreAndDelete(ctx context.Context, userID int, labelID int) error {
	// create one transaction to confirm if the label is owned by the user then delete all ChoreLabels record for this label:
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		log := logging.FromContext(ctx)
		var labelCount int64
		if err := tx.Model(&lModel.Label{}).Where("id = ? AND created_by = ?", labelID, userID).Count(&labelCount).Error; err != nil {
			log.Debug(err)
			return err
		}
		if labelCount < 1 {
			return errors.New("label is not owned by user")
		}

		// Capture affected chores before the join rows disappear.
		choreIDs, err := r.choreIDsWithLabels(ctx, tx, []int{labelID})
		if err != nil {
			log.Debug("Error getting chores for label")
			return err
		}

		if err := tx.Where("label_id = ?", labelID).Delete(&chModel.ChoreLabels{}).Error; err != nil {
			log.Debug("Error deleting chore labels")
			return err
		}

		if err := r.bumpChoreSyncVersions(ctx, tx, choreIDs); err != nil {
			log.Debug("Error bumping chore sync versions")
			return err
		}
		// delete the actual label:
		if err := tx.Where("id = ?", labelID).Delete(&lModel.Label{}).Error; err != nil {
			log.Debug("Error deleting label")
			return err
		}

		return nil
	})
}

func (r *LabelRepository) isLabelsOwner(ctx context.Context, userID int, labelIDs []int) bool {
	var count int64
	r.db.WithContext(ctx).Model(&lModel.Label{}).Where("id IN (?) AND created_by = ?", labelIDs, userID).Count(&count)
	return count == int64(len(labelIDs))
}

func (r *LabelRepository) DeleteLabels(ctx context.Context, userID int, ids []int) error {
	// remove all ChoreLabels record for this:
	if !r.isLabelsOwner(ctx, userID, ids) {
		return errors.New("labels are not owned by user")
	}

	tx := r.db.WithContext(ctx).Begin()

	// Capture affected chores before the join rows disappear.
	choreIDs, err := r.choreIDsWithLabels(ctx, tx, ids)
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("label_id IN (?)", ids).Delete(&chModel.ChoreLabels{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("id IN (?)", ids).Delete(&lModel.Label{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := r.bumpChoreSyncVersions(ctx, tx, choreIDs); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (r *LabelRepository) UpdateLabel(ctx context.Context, userID int, label *lModel.Label) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.WithContext(ctx).Model(&lModel.Label{}).Where("id = ? and created_by = ?", label.ID, userID).Updates(label)
		if result.Error != nil {
			return result.Error
		}
		// Not the owner (or no such label): nothing changed, so nothing to sync.
		if result.RowsAffected < 1 {
			return nil
		}

		// Chores embed the label (name/color) in their payload, so they have to be
		// re-synced even though the chore rows themselves are untouched.
		choreIDs, err := r.choreIDsWithLabels(ctx, tx, []int{label.ID})
		if err != nil {
			return err
		}
		return r.bumpChoreSyncVersions(ctx, tx, choreIDs)
	})
}
