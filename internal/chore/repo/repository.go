package chore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	config "donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	storageModel "donetick.com/core/internal/storage/model"
	stModel "donetick.com/core/internal/subtask/model"
	syncModel "donetick.com/core/internal/sync/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChoreRepository struct {
	db     *gorm.DB
	dbType string
}

// SyncOptions configures sync-aware queries for chores and histories.
// When SyncVersion is nil, the query is NOT filtered by sync version (normal operation).
// When SyncVersion is set, the query returns only records with sync_version > *SyncVersion, in ascending order.
type SyncOptions struct {
	SyncVersion *int64 // nil = no sync filtering; pointer = filter by version
	Limit       int    // Number of records to fetch (used with sync filtering)
}

func NewChoreRepository(db *gorm.DB, cfg *config.Config) *ChoreRepository {
	return &ChoreRepository{db: db, dbType: cfg.Database.Type}
}

// privacyPredicate returns a GORM clause expression that enforces chore visibility
// rules using bound parameters (no string interpolation). A chore is visible if:
//   - It is not private, OR
//   - It is private AND (the user created it OR the user is assigned to it)
//
// Use as: db.Where(privacyPredicate(userID))
func privacyPredicate(userID int) clause.Expr {
	return clause.Expr{
		SQL:  `((chores.is_private = false) OR (chores.is_private = true AND (chores.created_by = ? OR chore_assignees.user_id = ?)))`,
		Vars: []interface{}{userID, userID},
	}
}

func (r *ChoreRepository) nextSyncVersionWithDB(ctx context.Context, db *gorm.DB, circleID int) (int64, error) {
	var version int64
	err := db.WithContext(ctx).Raw(`
		INSERT INTO sync_cursors (circle_id, entity_type, max_version)
		VALUES (?, ?, 1)
		ON CONFLICT (circle_id, entity_type) DO UPDATE SET max_version = sync_cursors.max_version + 1
		RETURNING max_version`,
		circleID, syncModel.EntityTypeChore,
	).Scan(&version).Error
	return version, err
}

func (r *ChoreRepository) nextSyncVersion(ctx context.Context, circleID int) (int64, error) {
	return r.nextSyncVersionWithDB(ctx, r.db, circleID)
}

// nextSyncVersionRangeWithDB atomically reserves count contiguous sync versions
// and returns the first (lowest) version in the range.
func (r *ChoreRepository) nextSyncVersionRangeWithDB(ctx context.Context, db *gorm.DB, circleID int, count int) (int64, error) {
	var endVersion int64
	err := db.WithContext(ctx).Raw(`
		INSERT INTO sync_cursors (circle_id, entity_type, max_version)
		VALUES (?, ?, ?)
		ON CONFLICT (circle_id, entity_type) DO UPDATE SET max_version = sync_cursors.max_version + ?
		RETURNING max_version`,
		circleID, syncModel.EntityTypeChore, count, count,
	).Scan(&endVersion).Error
	return endVersion - int64(count) + 1, err
}

func (r *ChoreRepository) insertTombstone(ctx context.Context, tx *gorm.DB, circleID int, entityID int) (int64, error) {
	// Use tx (not r.db) so the version increment and tombstone insert are atomic.
	version, err := r.nextSyncVersionWithDB(ctx, tx, circleID)
	if err != nil {
		return 0, err
	}
	if err := tx.WithContext(ctx).Create(&syncModel.Tombstone{
		CircleID:    circleID,
		EntityType:  syncModel.EntityTypeChore,
		EntityID:    entityID,
		SyncVersion: version,
	}).Error; err != nil {
		return 0, err
	}
	return version, nil
}

func (r *ChoreRepository) UpsertChore(c context.Context, chore *chModel.Chore) error {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return err
	}
	chore.SyncVersion = nextVersion
	return r.db.WithContext(c).Model(&chore).Save(chore).Error
}

func choreViewerSet(chore *chModel.Chore, assigneeIDs []int, circleUserIDs []int) map[int]struct{} {
	viewers := make(map[int]struct{})
	if !chore.IsPrivate {
		for _, userID := range circleUserIDs {
			viewers[userID] = struct{}{}
		}
		return viewers
	}

	viewers[chore.CreatedBy] = struct{}{}
	for _, userID := range assigneeIDs {
		viewers[userID] = struct{}{}
	}
	return viewers
}

// UpdateChoreVisibility atomically updates a chore and its complete assignee set,
// recording a user-scoped tombstone for each circle member who loses visibility.
func (r *ChoreRepository) UpdateChoreVisibility(ctx context.Context, chore *chModel.Chore, assigneeIDs []int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := tx.WithContext(ctx)
		if r.dbType == "postgres" {
			locked = locked.Clauses(clause.Locking{Strength: "UPDATE"})
		}

		var previousChore chModel.Chore
		if err := locked.Where("id = ? AND circle_id = ?", chore.ID, chore.CircleID).First(&previousChore).Error; err != nil {
			return err
		}

		var previousAssignees []chModel.ChoreAssignees
		if err := locked.Where("chore_id = ?", chore.ID).Find(&previousAssignees).Error; err != nil {
			return err
		}
		previousAssigneeIDs := make([]int, 0, len(previousAssignees))
		for _, assignee := range previousAssignees {
			previousAssigneeIDs = append(previousAssigneeIDs, assignee.UserID)
		}

		var circleUserIDs []int
		if err := tx.WithContext(ctx).Table("user_circles").Where("circle_id = ? AND is_active = ?", chore.CircleID, true).Pluck("user_id", &circleUserIDs).Error; err != nil {
			return err
		}

		previousViewers := choreViewerSet(&previousChore, previousAssigneeIDs, circleUserIDs)
		newViewers := choreViewerSet(chore, assigneeIDs, circleUserIDs)
		revokedUserIDs := make([]int, 0)
		for userID := range previousViewers {
			if _, stillVisible := newViewers[userID]; !stillVisible {
				revokedUserIDs = append(revokedUserIDs, userID)
			}
		}
		sort.Ints(revokedUserIDs)

		firstVersion, err := r.nextSyncVersionRangeWithDB(ctx, tx, chore.CircleID, 1+len(revokedUserIDs))
		if err != nil {
			return err
		}
		chore.SyncVersion = firstVersion
		if err := tx.WithContext(ctx).Save(chore).Error; err != nil {
			return err
		}

		if err := tx.WithContext(ctx).Where("chore_id = ?", chore.ID).Delete(&chModel.ChoreAssignees{}).Error; err != nil {
			return err
		}
		assignees := make([]chModel.ChoreAssignees, 0, len(assigneeIDs))
		seenAssignees := make(map[int]struct{}, len(assigneeIDs))
		for _, userID := range assigneeIDs {
			if _, exists := seenAssignees[userID]; exists {
				continue
			}
			seenAssignees[userID] = struct{}{}
			assignees = append(assignees, chModel.ChoreAssignees{ChoreID: chore.ID, UserID: userID})
		}
		if len(assignees) > 0 {
			if err := tx.WithContext(ctx).Create(&assignees).Error; err != nil {
				return err
			}
		}

		tombstones := make([]syncModel.Tombstone, 0, len(revokedUserIDs))
		for offset, userID := range revokedUserIDs {
			tombstones = append(tombstones, syncModel.Tombstone{
				CircleID:    chore.CircleID,
				EntityType:  syncModel.EntityTypeChore,
				EntityID:    chore.ID,
				UserID:      &userID,
				SyncVersion: firstVersion + int64(offset) + 1,
			})
		}
		if len(tombstones) > 0 {
			if err := tx.WithContext(ctx).Create(&tombstones).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ChoreRepository) UpdateChorePriority(c context.Context, userID int, choreID int, priority int, circleID int) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	result := r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? AND circle_id = ?", choreID, circleID).Updates(map[string]interface{}{
		"priority":     priority,
		"sync_version": nextVersion,
	})
	if result.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return result.Error
}

func (r *ChoreRepository) UpdateChoreFields(ctx context.Context, choreID int, fields map[string]interface{}) error {
	var chore chModel.Chore
	if err := r.db.WithContext(ctx).Select("circle_id").First(&chore, choreID).Error; err != nil {
		return err
	}
	nextVersion, err := r.nextSyncVersion(ctx, chore.CircleID)
	if err != nil {
		return err
	}
	fields["sync_version"] = nextVersion
	return r.db.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", choreID).Updates(fields).Error
}

func (r *ChoreRepository) nextSyncVersionRange(ctx context.Context, circleID int, count int) (int64, error) {
	return r.nextSyncVersionRangeWithDB(ctx, r.db, circleID, count)
}

func (r *ChoreRepository) UpdateChores(c context.Context, chores []*chModel.Chore) error {
	if len(chores) > 0 {
		circleIndexes := make(map[int][]int)
		for i, ch := range chores {
			circleIndexes[ch.CircleID] = append(circleIndexes[ch.CircleID], i)
		}

		for circleID, indexes := range circleIndexes {
			startVersion, err := r.nextSyncVersionRange(c, circleID, len(indexes))
			if err != nil {
				return err
			}
			for offset, idx := range indexes {
				chores[idx].SyncVersion = startVersion + int64(offset)
			}
		}
	}
	return r.db.WithContext(c).Save(&chores).Error
}
func (r *ChoreRepository) CreateChore(c context.Context, chore *chModel.Chore) (int, error) {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return 0, err
	}
	chore.SyncVersion = nextVersion
	if err := r.db.WithContext(c).Create(chore).Error; err != nil {
		return 0, err
	}
	return chore.ID, nil
}

func (r *ChoreRepository) GetChore(c context.Context, choreID int, userID int, circleID int) (*chModel.Chore, error) {
	var chore chModel.Chore
	query := r.db.WithContext(c).Model(&chModel.Chore{}).
		Preload("SubTasks", "chore_id = ?", choreID).
		Preload("Assignees").
		Preload("ThingChore").
		Preload("LabelsV2").
		Joins("LEFT JOIN chore_assignees ON chores.id = chore_assignees.chore_id AND chore_assignees.user_id = ?", userID).
		Where("chores.id = ? AND chores.circle_id = ? AND ((chores.is_private = false) OR (chores.is_private = true AND (chores.created_by = ? OR chore_assignees.user_id = ?)))", choreID, circleID, userID, userID)

	if err := query.First(&chore).Error; err != nil {
		return nil, err
	}
	r.db.WithContext(c).Where("entity_type = ? AND entity_id = ?", storageModel.EntityTypeChoreAttachment, choreID).Find(&chore.Attachments)
	return &chore, nil
}

// GetChores retrieves chores for a user in a circle, respecting privacy and optionally filtered by sync version.
// If syncOptions is nil or syncOptions.SyncVersion is nil, returns all visible chores ordered by next_due_date.
// If syncOptions.SyncVersion is set, returns only chores with sync_version > *SyncVersion, ordered by sync_version.
func (r *ChoreRepository) GetChores(c context.Context, circleID int, userID int, includeArchived bool, syncOptions *SyncOptions, includeSubtasks bool) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore

	query := r.db.WithContext(c).
		Preload("Assignees").
		Preload("LabelsV2").
		Joins("left join chore_assignees on chores.id = chore_assignees.chore_id").
		Where("chores.circle_id = ?", circleID).
		Where(privacyPredicate(userID)).
		Group("chores.id")

	if includeSubtasks {
		query = query.Preload("SubTasks")
	}
	if !includeArchived {
		query = query.Where("chores.is_active = ?", true)
	}

	// Sync-specific filtering if provided.
	if syncOptions != nil && syncOptions.SyncVersion != nil {
		query = query.Where("chores.sync_version > ?", *syncOptions.SyncVersion).
			Order("chores.sync_version asc").
			Limit(syncOptions.Limit + 1)
	} else {
		// Normal ordering when not syncing.
		query = query.Order("next_due_date asc")
	}

	if err := query.Find(&chores).Error; err != nil {
		return nil, err
	}
	return chores, nil
}

func (r *ChoreRepository) GetArchivedChores(c context.Context, circleID int, userID int) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore
	if err := r.db.WithContext(c).
		Preload("Assignees").
		Preload("LabelsV2").
		Joins("left join chore_assignees on chores.id = chore_assignees.chore_id").
		Where("chores.circle_id = ?", circleID).
		Where(privacyPredicate(userID)).
		Where("is_active = ?", false).
		Group("chores.id").
		Order("next_due_date asc").
		Find(&chores).Error; err != nil {
		return nil, err
	}
	return chores, nil
}

func (r *ChoreRepository) DeleteChore(c context.Context, id int) (int64, error) {
	var tombstoneSyncVersion int64
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		query := tx.WithContext(c)
		if r.dbType == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var chore chModel.Chore
		if err := query.Select("id, circle_id").First(&chore, id).Error; err != nil {
			return err
		}

		// Delete all chore assignees
		if err := tx.Where("chore_id = ?", id).Delete(&chModel.ChoreAssignees{}).Error; err != nil {
			return err
		}
		// Delete all time sessions
		if err := tx.Where("chore_id = ?", id).Delete(&chModel.TimeSession{}).Error; err != nil {
			return err
		}
		// Delete all chore histories
		if err := tx.Where("chore_id = ?", id).Delete(&chModel.ChoreHistory{}).Error; err != nil {
			return err
		}
		// Delete all subtasks
		if err := tx.Where("chore_id = ?", id).Delete(&stModel.SubTask{}).Error; err != nil {
			return err
		}
		// Delete all chore labels
		if err := tx.Where("chore_id = ?", id).Delete(&chModel.ChoreLabels{}).Error; err != nil {
			return err
		}
		// Delete all storage files associated with the chore
		if err := tx.Where("entity_type = ? AND entity_id = ?", storageModel.EntityTypeChoreDescription, id).Delete(&storageModel.StorageFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("entity_type = ? AND entity_id = ?", storageModel.EntityTypeChoreAttachment, id).Delete(&storageModel.StorageFile{}).Error; err != nil {
			return err
		}
		// Delete the chore itself
		if err := tx.Delete(&chModel.Chore{}, id).Error; err != nil {
			return err
		}
		// Record tombstone so clients can remove it during sync
		version, err := r.insertTombstone(c, tx, chore.CircleID, id)
		if err != nil {
			return err
		}
		tombstoneSyncVersion = version
		return nil
	})
	if err != nil {
		return 0, err
	}
	return tombstoneSyncVersion, nil
}

func (r *ChoreRepository) SoftDelete(c context.Context, id int, userID int, circleID int) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	return r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? AND created_by = ? AND circle_id = ?", id, userID, circleID).Updates(map[string]interface{}{
		"is_active":    false,
		"sync_version": nextVersion,
	}).Error
}

func (r *ChoreRepository) SetChorePendingApproval(c context.Context, chore *chModel.Chore, note *string, userID int, completedDate *time.Time) error {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return err
	}
	err = r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Look for existing chore history with start or pause status
		var existingHistory chModel.ChoreHistory
		err := tx.Where("chore_id = ? AND status = ? ",
			chore.ID, chModel.ChoreHistoryStatusStarted).
			First(&existingHistory).Error

		var ch *chModel.ChoreHistory
		switch {
		case err == nil:
			// Update existing history record to mark as pending approval
			existingHistory.PerformedAt = completedDate
			existingHistory.Note = note
			existingHistory.Status = chModel.ChoreHistoryStatusPendingApproval
			ch = &existingHistory
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Create a new chore history record
			ch = &chModel.ChoreHistory{
				ChoreID:     chore.ID,
				PerformedAt: completedDate,
				CompletedBy: userID,
				AssignedTo:  chore.AssignedTo,
				DueDate:     chore.NextDueDate,
				Note:        note,
				Status:      chModel.ChoreHistoryStatusPendingApproval,
			}
		default:
			return err
		}

		ch.SyncVersion = nextVersion
		if err := tx.Save(ch).Error; err != nil {
			return err
		}

		// Set chore status to pending approval
		if err := tx.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(map[string]interface{}{
			"status":       chModel.ChoreStatusPendingApproval,
			"sync_version": nextVersion,
		}).Error; err != nil {
			return err
		}

		// if there is any time session associated with the chore, mark them as finished:
		var timeSessions []*chModel.TimeSession
		tx.Model(&chModel.TimeSession{}).Where("chore_id = ? AND status < ?", chore.ID, chModel.TimeSessionStatusCompleted).Find(&timeSessions)
		if len(timeSessions) != 0 {
			for _, session := range timeSessions {
				session.Finish(userID)
			}
			if err := tx.Save(&timeSessions).Error; err != nil {
				return err
			}
		}

		return nil
	})
	return err
}

func (r *ChoreRepository) ApproveChore(c context.Context, chore *chModel.Chore, adminUserID int, dueDate *time.Time, nextAssignedTo *int, applyPoints bool) error {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return err
	}
	err = r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		choreUpdates := map[string]interface{}{}
		choreUpdates["next_due_date"] = dueDate
		choreUpdates["status"] = chModel.ChoreStatusNoStatus
		choreUpdates["sync_version"] = nextVersion

		if dueDate != nil {
			choreUpdates["assigned_to"] = nextAssignedTo
		} else {
			// one time task
			choreUpdates["is_active"] = false
		}

		// Get the latest history entry that's pending approval and update it to completed
		var history chModel.ChoreHistory
		err := tx.Where("chore_id = ? AND status = ?", chore.ID, chModel.ChoreHistoryStatusPendingApproval).
			Order("performed_at desc").
			First(&history).Error
		if err != nil {
			return err
		}

		// Update status to completed
		history.Status = chModel.ChoreHistoryStatusCompleted
		history.SyncVersion = nextVersion

		// Update UserCircle Points if applicable
		if applyPoints && chore.Points != nil && *chore.Points > 0 {
			history.Points = chore.Points
			if err := tx.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", history.CompletedBy, chore.CircleID).Update("points", gorm.Expr("points + ?", chore.Points)).Error; err != nil {
				return err
			}
		}

		// Save the updated history
		if err := tx.Save(&history).Error; err != nil {
			return err
		}

		// Perform the update operation once, using the prepared updates map.
		if err := tx.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(choreUpdates).Error; err != nil {
			return err
		}

		return nil
	})
	return err
}

func (r *ChoreRepository) RejectChore(c context.Context, choreID int, circleID int, rejectionNote *string) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Reset chore status to normal
		if err := tx.Model(&chModel.Chore{}).Where("id = ? AND circle_id = ?", choreID, circleID).Updates(map[string]interface{}{
			"status":       chModel.ChoreStatusNoStatus,
			"sync_version": nextVersion,
		}).Error; err != nil {
			return err
		}

		// Get the latest pending approval history entry and mark it as rejected
		var history chModel.ChoreHistory
		err := tx.Where("chore_id = ? AND status = ?", choreID, chModel.ChoreHistoryStatusPendingApproval).
			Order("performed_at desc").
			First(&history).Error
		if err != nil {
			return err
		}

		// Update status to rejected
		history.Status = chModel.ChoreHistoryStatusRejected
		history.SyncVersion = nextVersion

		// Save the updated history
		if err := tx.Save(&history).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *ChoreRepository) CompleteChore(c context.Context, chore *chModel.Chore, note *string, userID int, dueDate *time.Time, completedDate *time.Time, nextAssignedTo *int, applyPoints bool) error {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return err
	}
	err = r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {

		choreUpdates := map[string]interface{}{}
		choreUpdates["next_due_date"] = dueDate
		choreUpdates["status"] = chModel.ChoreStatusNoStatus
		choreUpdates["sync_version"] = nextVersion

		switch {
		case dueDate != nil:
			choreUpdates["assigned_to"] = nextAssignedTo
		case chore.FrequencyType == "trigger":
			// In case of trigger frequency type we need to still set the next assigned but need the task archived.
			choreUpdates["assigned_to"] = nextAssignedTo
			choreUpdates["is_active"] = false
		default:
			// one time task
			choreUpdates["is_active"] = false
		}

		// Look for existing chore history with start or pause status
		var existingHistory chModel.ChoreHistory
		err := tx.Where("chore_id = ? AND status = ? ",
			chore.ID, chModel.ChoreHistoryStatusStarted).
			First(&existingHistory).Error

		var ch *chModel.ChoreHistory
		switch {
		case err == nil:
			// Update existing history record
			existingHistory.PerformedAt = completedDate
			existingHistory.Note = note
			existingHistory.Status = chModel.ChoreHistoryStatusCompleted
			ch = &existingHistory
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Create a new chore history record
			ch = &chModel.ChoreHistory{
				ChoreID:     chore.ID,
				PerformedAt: completedDate,
				CompletedBy: userID,
				AssignedTo:  chore.AssignedTo,
				DueDate:     chore.NextDueDate,
				Note:        note,
				Status:      chModel.ChoreHistoryStatusCompleted,
			}
		default:
			return err
		}

		// Update UserCirclee Points :
		if applyPoints && chore.Points != nil && *chore.Points > 0 {
			ch.Points = chore.Points
			if err := tx.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", userID, chore.CircleID).Update("points", gorm.Expr("points + ?", chore.Points)).Error; err != nil {
				return err
			}
		}
		ch.SyncVersion = nextVersion
		// Perform the update operation once, using the prepared updates map.
		if err := tx.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(choreUpdates).Error; err != nil {
			return err
		}

		if err := tx.Save(ch).Error; err != nil {
			return err
		}
		// if there is any time session associated with the chore, mark them as finished:
		var timeSessions []*chModel.TimeSession
		tx.Model(&chModel.TimeSession{}).Where("chore_id = ? AND status < ?", chore.ID, chModel.TimeSessionStatusCompleted).Find(&timeSessions)
		if len(timeSessions) != 0 {
			for _, session := range timeSessions {
				session.Finish(userID)
			}
			if err := tx.Save(&timeSessions).Error; err != nil {
				return err
			}
		}

		return nil
	})
	return err
}

func (r *ChoreRepository) SkipChore(c context.Context, chore *chModel.Chore, userID int, dueDate *time.Time, nextAssignedTo *int, skippedAt *time.Time) error {
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return err
	}
	err = r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		choreUpdates := map[string]interface{}{}
		choreUpdates["next_due_date"] = dueDate
		choreUpdates["status"] = chModel.ChoreStatusNoStatus
		choreUpdates["sync_version"] = nextVersion

		if dueDate != nil {
			choreUpdates["assigned_to"] = nextAssignedTo
		} else {
			// one time task
			choreUpdates["is_active"] = false
		}

		// Look for existing chore history with start or pause status
		var existingHistory chModel.ChoreHistory
		err := tx.Where("chore_id = ? AND status = ? ",
			chore.ID, chModel.ChoreHistoryStatusStarted).
			First(&existingHistory).Error

		var ch *chModel.ChoreHistory
		effectiveSkippedAt := time.Now().UTC()
		if skippedAt != nil {
			effectiveSkippedAt = skippedAt.UTC()
		}

		switch {
		case err == nil && existingHistory.PerformedAt != nil:
			// Update existing history record
			existingHistory.PerformedAt = &effectiveSkippedAt
			existingHistory.Note = nil
			existingHistory.Status = chModel.ChoreHistoryStatusSkipped
			ch = &existingHistory
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Create a new chore history record for the skipped chore
			ch = &chModel.ChoreHistory{
				ChoreID:     chore.ID,
				PerformedAt: &effectiveSkippedAt,
				CompletedBy: userID,
				AssignedTo:  chore.AssignedTo,
				DueDate:     chore.NextDueDate,
				Note:        nil,
				Status:      chModel.ChoreHistoryStatusSkipped,
			}
		default:
			return err
		}

		ch.SyncVersion = nextVersion
		// Perform the update operation once, using the prepared updates map.
		if err := tx.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Updates(choreUpdates).Error; err != nil {
			return err
		}

		if err := tx.Save(ch).Error; err != nil {
			return err
		}

		// if there is any time session associated with the chore, mark them as finished:
		var timeSessions []*chModel.TimeSession
		tx.Model(&chModel.TimeSession{}).Where("chore_id = ? AND status < ?", chore.ID, chModel.TimeSessionStatusCompleted).Find(&timeSessions)
		if len(timeSessions) != 0 {
			for _, session := range timeSessions {
				session.Finish(userID)
			}
			if err := tx.Save(&timeSessions).Error; err != nil {
				return err
			}
		}

		return nil
	})
	return err
}

func (r *ChoreRepository) GetChoreHistory(c context.Context, choreID int) ([]*chModel.ChoreHistory, error) {
	var histories []*chModel.ChoreHistory
	if err := r.db.WithContext(c).
		Table("chore_histories").
		Select("chore_histories.*, time_sessions.duration").
		Joins("LEFT JOIN time_sessions ON chore_histories.id = time_sessions.chore_history_id").
		Where("chore_histories.chore_id = ?", choreID).
		Order("chore_histories.updated_at desc").
		Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}
func (r *ChoreRepository) GetChoreHistoryWithLimit(c context.Context, choreID int, limit int) ([]*chModel.ChoreHistory, error) {
	var histories []*chModel.ChoreHistory
	if err := r.db.WithContext(c).
		Table("chore_histories").
		Select("chore_histories.*, time_sessions.duration").
		Joins("LEFT JOIN time_sessions ON chore_histories.id = time_sessions.chore_history_id").
		Where("chore_histories.chore_id = ?", choreID).
		Order("chore_histories.performed_at desc").
		Limit(limit).
		Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

func (r *ChoreRepository) GetChoreHistoryByID(c context.Context, choreID int, historyID int) (*chModel.ChoreHistory, error) {
	var history chModel.ChoreHistory
	if err := r.db.WithContext(c).Where("id = ? and chore_id = ? ", historyID, choreID).First(&history).Error; err != nil {
		return nil, err
	}
	return &history, nil
}

func (r *ChoreRepository) CreateChoreHistory(c context.Context, history *chModel.ChoreHistory) error {
	// Get the circle_id from the associated chore
	var chore chModel.Chore
	if err := r.db.WithContext(c).Select("circle_id").First(&chore, history.ChoreID).Error; err != nil {
		return fmt.Errorf("failed to get chore for history creation: %w", err)
	}

	// Allocate sync version for this history
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return fmt.Errorf("failed to allocate sync version: %w", err)
	}

	history.SyncVersion = nextVersion
	return r.db.WithContext(c).Create(history).Error
}

func (r *ChoreRepository) UpdateChoreHistory(c context.Context, history *chModel.ChoreHistory) error {
	// Get the circle_id from the associated chore
	var chore chModel.Chore
	if err := r.db.WithContext(c).Select("circle_id").First(&chore, history.ChoreID).Error; err != nil {
		return fmt.Errorf("failed to get chore for history update: %w", err)
	}

	// Every history update must get a fresh sync version so it is visible to sync clients.
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return fmt.Errorf("failed to allocate sync version: %w", err)
	}
	history.SyncVersion = nextVersion

	return r.db.WithContext(c).Save(history).Error
}

func (r *ChoreRepository) UpdateLatestChoreHistory(c context.Context, choreID int, updates map[string]interface{}) error {
	var chore chModel.Chore
	if err := r.db.WithContext(c).Select("circle_id").First(&chore, choreID).Error; err != nil {
		return fmt.Errorf("failed to get chore for latest history update: %w", err)
	}
	nextVersion, err := r.nextSyncVersion(c, chore.CircleID)
	if err != nil {
		return fmt.Errorf("failed to allocate sync version: %w", err)
	}
	updates["sync_version"] = nextVersion

	// get the latest chore history for the given chore ID
	var latestHistory chModel.ChoreHistory
	if err := r.db.WithContext(c).Where("chore_id = ?", choreID).Order("created_at desc").First(&latestHistory).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("no history found for chore ID %d", choreID)
		}
		return err
	}
	// Update the latest history with the provided updates
	if err := r.db.WithContext(c).Model(&latestHistory).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update latest chore history: %w", err)
	}
	return nil
}

func (r *ChoreRepository) DeleteChoreHistory(c context.Context, historyID int, circleID int) error {
	// create transaction and delete all the chore timer associated with the chore history then delete the chore history
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Allocate next sync version within the transaction for atomicity
		nextVersion, err := r.nextSyncVersionWithDB(c, tx, circleID)
		if err != nil {
			return err
		}

		if err := tx.WithContext(c).Where("chore_history_id = ?", historyID).Delete(&chModel.TimeSession{}).Error; err != nil {
			return fmt.Errorf("failed to delete chore time sessions: %w", err)
		}
		// Now delete the chore history
		if err := tx.WithContext(c).Delete(&chModel.ChoreHistory{}, historyID).Error; err != nil {
			return fmt.Errorf("failed to delete chore history: %w", err)
		}
		// Record tombstone so clients can remove it during sync
		return tx.WithContext(c).Create(&syncModel.Tombstone{
			CircleID:    circleID,
			EntityType:  syncModel.EntityTypeChoreHistory,
			EntityID:    historyID,
			SyncVersion: nextVersion,
		}).Error
	})
}

func (r *ChoreRepository) UpdateChoreAssignees(c context.Context, assignees []*chModel.ChoreAssignees) error {
	return r.db.WithContext(c).Save(&assignees).Error
}

func (r *ChoreRepository) DeleteChoreAssignees(c context.Context, choreAssignees []*chModel.ChoreAssignees) error {
	return r.db.WithContext(c).Delete(&choreAssignees).Error
}

func (r *ChoreRepository) GetChoreAssignees(c context.Context, choreID int) ([]*chModel.ChoreAssignees, error) {
	var assignees []*chModel.ChoreAssignees
	if err := r.db.WithContext(c).Find(&assignees, "chore_id = ?", choreID).Error; err != nil {
		return nil, err
	}
	return assignees, nil
}

func (r *ChoreRepository) RemoveChoreAssigneeByCircleID(c context.Context, userID int, circleID int) error {
	return r.db.WithContext(c).Where("user_id = ? AND chore_id IN (SELECT id FROM chores WHERE circle_id = ? and created_by != ?)", userID, circleID, userID).Delete(&chModel.ChoreAssignees{}).Error
}

// func (r *ChoreReposity) GetOverdueChoresForNotification(c context.Context, overdueDuration time.Duration, everyDuration time.Duration, untilDuration time.Duration) ([]*chModel.Chore, error) {
// 	var chores []*chModel.Chore
// 	query := r.db.Debug().WithContext(c).Table("chores").Select("chores.*, MAX(n.created_at) as max_notification_created_at").Joins("left join notifications n on n.chore_id = chores.id and n.scheduled_for = chores.next_due_date and n.type = 2")
// 	if err := query.Where("chores.is_active = ? and chores.notification = ? and chores.next_due_date < ? and chores.next_due_date > ?", true, true, time.Now().UTC().Add(overdueDuration).UTC(), time.Now().Add(untilDuration).UTC()).Where(readJSONBooleanField(r.dbType, "chores.notification_meta", "nagging")).Having("MAX(n.created_at) is null or MAX(n.created_at) < ?", time.Now().Add(everyDuration).UTC()).Group("chores.id").Find(&chores).Error; err != nil {
// 		return nil, err
// 	}
// 	return chores, nil
// }

func (r *ChoreRepository) GetOverdueChoresForNotification(c context.Context, overdueFor time.Duration, everyDuration time.Duration, untilDuration time.Duration) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore
	now := time.Now().UTC()
	overdueTime := now.Add(-overdueFor)
	everyTime := now.Add(-everyDuration)
	untilTime := now.Add(-untilDuration)

	query := r.db.Debug().WithContext(c).
		Table("chores").
		Select("chores.*, MAX(n.created_at) as max_notification_created_at").
		Joins("left join notifications n on n.chore_id = chores.id and n.type = 2").
		Where("chores.is_active = ? AND chores.notification = ? AND chores.next_due_date < ? AND chores.next_due_date > ?", true, true, overdueTime, untilTime).
		Where(readJSONBooleanField(r.dbType, "chores.notification_meta", "nagging")).
		Group("chores.id").
		Having("MAX(n.created_at) IS NULL OR MAX(n.created_at) < ?", everyTime)

	if err := query.Find(&chores).Error; err != nil {
		return nil, err
	}

	return chores, nil
}

// a predue notfication is a notification send before the due date in 6 hours, 3 hours :
func (r *ChoreRepository) GetPreDueChoresForNotification(c context.Context, preDueDuration time.Duration, everyDuration time.Duration) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore
	query := r.db.WithContext(c).Table("chores").Select("chores.*, MAX(n.created_at) as max_notification_created_at").Joins("left join notifications n on n.chore_id = chores.id and n.scheduled_for = chores.next_due_date and n.type = 3")
	if err := query.Where("chores.is_active = ? and chores.notification = ? and chores.next_due_date > ? and chores.next_due_date < ?", true, true, time.Now().UTC(), time.Now().UTC().Add(everyDuration*2)).Where(readJSONBooleanField(r.dbType, "chores.notification_meta", "predue")).Having("MAX(n.created_at) is null or MAX(n.created_at) < ?", time.Now().UTC().Add(everyDuration)).Group("chores.id").Find(&chores).Error; err != nil {
		return nil, err
	}
	return chores, nil
}

func readJSONBooleanField(dbType string, columnName string, fieldName string) clause.Expr {
	if dbType == "postgres" {
		return gorm.Expr("("+columnName+"::json->>?)::boolean", fieldName)
	}
	return gorm.Expr("JSON_EXTRACT("+columnName+", ?)", "$."+fieldName)
}

func (r *ChoreRepository) SetDueDate(c context.Context, choreID int, dueDate time.Time) error {
	return r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ?", choreID).Updates(map[string]interface{}{
		"next_due_date": dueDate,
		"is_active":     true,
	}).Error
}

func (r *ChoreRepository) SetDueDateIfNotExisted(c context.Context, choreID int, dueDate time.Time) error {
	return r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? and next_due_date is null", choreID).Updates(map[string]interface{}{
		"next_due_date": dueDate,
		"is_active":     true,
	}).Error
}

func (r *ChoreRepository) GetChoreDetailByID(c context.Context, choreID int, circleID int, userID int) (*chModel.ChoreDetail, error) {
	var choreDetail chModel.ChoreDetail
	if err := r.db.WithContext(c).
		Table("chores").
		Preload("Subtasks").
		Select(`
        chores.id, 
        chores.name,
		chores.description, 
        chores.frequency_type, 
        chores.next_due_date, 
        chores.assigned_to,
        chores.created_by,
		chores.priority,
		chores.completion_window,
		chores.status,
		chores.is_active,
		chores.sync_version,
		CAST(MAX(time_sessions.duration) AS INTEGER) as duration,
		time_sessions.start_time as start_time,
		time_sessions.updated_at as timer_updated_at,
        recent_history.last_completed_date,
		recent_history.last_assigned_to,
		recent_history.notes,
        recent_history.last_completed_by as last_completed_by,
        COUNT(chore_histories.id) as total_completed`).
		Joins("LEFT JOIN chore_histories ON chores.id = chore_histories.chore_id").
		Joins(`LEFT JOIN (
        SELECT
            chore_id,
            assigned_to AS last_assigned_to,
            performed_at AS last_completed_date,
			completed_by AS last_completed_by,
			notes

        FROM chore_histories
        WHERE (chore_id, performed_at) IN (
            SELECT chore_id, MAX(performed_at)
            FROM chore_histories
            WHERE status IN (1, 2, 3, 4)
            GROUP BY chore_id
        )
        AND status IN (1, 2, 3, 4)
    ) AS recent_history ON chores.id = recent_history.chore_id`).
		Joins("LEFT JOIN time_sessions ON chores.id = time_sessions.chore_id AND time_sessions.status < ?", chModel.TimeSessionStatusCompleted).
		Joins("LEFT JOIN chore_assignees ON chores.id = chore_assignees.chore_id AND chore_assignees.user_id = ?", userID).
		Where("chores.id = ? AND chores.circle_id = ? AND ((chores.is_private = false) OR (chores.is_private = true AND (chores.created_by = ? OR chore_assignees.user_id = ?)))", choreID, circleID, userID, userID).
		Group("chores.id, recent_history.last_completed_date, recent_history.last_assigned_to, recent_history.last_completed_by, recent_history.notes, time_sessions.start_time, time_sessions.updated_at").
		First(&choreDetail).Error; err != nil {
		return nil, err
	}
	r.db.WithContext(c).Where("entity_type = ? AND entity_id = ?", storageModel.EntityTypeChoreAttachment, choreID).Find(&choreDetail.Attachments)
	return &choreDetail, nil
}

func (r *ChoreRepository) ArchiveChore(c context.Context, choreID int, userID int, circleID int) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	result := r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? AND created_by = ? AND circle_id = ?", choreID, userID, circleID).Updates(map[string]interface{}{
		"is_active":    false,
		"sync_version": nextVersion,
	})
	if result.RowsAffected == 0 {
		return errors.New("chore not found or not authorized")
	}
	return result.Error
}

func (r *ChoreRepository) UnarchiveChore(c context.Context, choreID int, userID int, circleID int) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	result := r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? AND created_by = ? AND circle_id = ?", choreID, userID, circleID).Updates(map[string]interface{}{
		"is_active":    true,
		"sync_version": nextVersion,
	})
	if result.RowsAffected == 0 {
		return errors.New("chore not found or not authorized")
	}
	return result.Error
}

func (r *ChoreRepository) GetChoresHistoryByUserID(c context.Context, userID int, circleID int, days int, includeCircle bool) ([]*chModel.ChoreHistory, error) {

	var chores []*chModel.ChoreHistory
	since := time.Now().UTC().AddDate(0, 0, days*-1)
	query := r.db.WithContext(c).
		Table("chore_histories").
		Select("chore_histories.*, circles.id as circle_id, time_sessions.duration, time_sessions.start_time, time_sessions.updated_at as timer_updated_at").
		Joins("LEFT JOIN chores ON chore_histories.chore_id = chores.id").
		Joins("LEFT JOIN circles ON chores.circle_id = circles.id").
		Joins("LEFT JOIN time_sessions ON chore_histories.id = time_sessions.chore_history_id").
		Joins("LEFT JOIN chore_assignees ON chores.id = chore_assignees.chore_id AND chore_assignees.user_id = ?", userID).
		Where("circles.id = ? AND chore_histories.updated_at > ?", circleID, since).
		Where("(chores.is_private = false) OR (chores.is_private = true AND (chores.created_by = ? OR chore_assignees.user_id = ?))", userID, userID).
		Order("chore_histories.performed_at desc, chore_histories.updated_at desc")

	if !includeCircle {
		query = query.Where("chore_histories.completed_by = ?", userID)
	}

	if err := query.Find(&chores).Error; err != nil {
		return nil, err
	}
	return chores, nil
}

func (r *ChoreRepository) UpdateChoreStatus(c context.Context, choreID int, status chModel.Status, circleID int) (int64, error) {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return 0, err
	}
	result := r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ? AND circle_id = ?", choreID, circleID).Updates(map[string]interface{}{
		"status":       status,
		"sync_version": nextVersion,
	})
	return nextVersion, result.Error
}

func (r *ChoreRepository) GetActiveTimeSession(c context.Context, choreID int) (*chModel.TimeSession, error) {
	var session chModel.TimeSession
	if err := r.db.WithContext(c).Where("chore_id = ? AND status < 2", choreID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No active session found
		}
		return nil, err
	}
	return &session, nil
}

func (r *ChoreRepository) CreateTimeSession(c context.Context, chore *chModel.Chore, userID int) (*chModel.TimeSession, error) {
	log := logging.FromContext(c)
	var timeSession *chModel.TimeSession
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		syncVersion, err := r.nextSyncVersionWithDB(c, tx, chore.CircleID)
		if err != nil {
			return fmt.Errorf("failed to allocate sync version: %w", err)
		}
		ch := &chModel.ChoreHistory{
			ChoreID:     chore.ID,
			CompletedBy: userID,
			AssignedTo:  chore.AssignedTo,
			DueDate:     chore.NextDueDate,
			Status:      chModel.ChoreHistoryStatusStarted,
			SyncVersion: syncVersion,
		}

		if err := tx.Create(ch).Error; err != nil {
			log.Errorf("Failed to create chore history: %v", err)
			return err
		}

		ts := &chModel.TimeSession{
			ChoreID:        chore.ID,
			ChoreHistoryID: ch.ID,
		}
		ts.Start(userID)
		if err := tx.Create(ts).Error; err != nil {
			log.Errorf("Failed to create time session: %v", err)
			return err
		}
		timeSession = ts
		return nil
	})
	return timeSession, err
}

func (r *ChoreRepository) UpdateTimeSession(c context.Context, session *chModel.TimeSession) error {
	return r.db.WithContext(c).Save(session).Error
}

func (r *ChoreRepository) CompleteTimeSession(c context.Context, session *chModel.TimeSession, chore *chModel.Chore, userID int) error {
	log := logging.FromContext(c)
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		session.Finish(userID)
		if err := tx.Save(session).Error; err != nil {
			log.Errorf("Failed to complete time session: %v", err)
			return err
		}
		return nil
	})

}

func (r *ChoreRepository) GetTimeSessionsByChoreID(c context.Context, choreID int, choreHistoryId *int) (*chModel.TimeSession, error) {
	var session *chModel.TimeSession

	if choreHistoryId != nil {
		// Get sessions for specific chore history ID
		if err := r.db.WithContext(c).Where("chore_id = ? AND chore_history_id = ?", choreID, *choreHistoryId).Find(&session).Error; err != nil {
			return nil, err
		}
	} else {
		// Get sessions for the most recent chore history (based on due_date)
		query := r.db.WithContext(c).
			Table("time_sessions").
			Select("time_sessions.*").
			Joins("LEFT JOIN chore_histories ON time_sessions.chore_history_id = chore_histories.id").
			Where("time_sessions.chore_id = ?", choreID).
			Order("chore_histories.due_date DESC")

		if err := query.Find(&session).Error; err != nil {
			return nil, err
		}
	}

	return session, nil
}

func (r *ChoreRepository) GetTimeSessionByID(c context.Context, sessionID int) (*chModel.TimeSession, error) {
	var session chModel.TimeSession
	if err := r.db.WithContext(c).First(&session, sessionID).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *ChoreRepository) UpdateTimeSessionData(c context.Context, session *chModel.TimeSession) error {
	// Recalculate duration based on pause log
	session.Duration = 0
	for _, entry := range session.PauseLog {
		if entry.Duration > 0 {
			session.Duration += entry.Duration
		}
	}

	session.UpdateAt = time.Now().UTC()
	return r.db.WithContext(c).Save(session).Error
}

func (r *ChoreRepository) DeleteTimeSession(c context.Context, sessionID int, choreID int) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// delete existing choreHistory linked to the time session:
		if err := tx.Where(
			"chore_id = ? and status = ?", choreID, chModel.ChoreHistoryStatusStarted,
		).Delete(&chModel.ChoreHistory{}).Error; err != nil {
			logging.FromContext(c).Errorf("Failed to delete chore history for session %d and chore %d: %v", sessionID, choreID, err)
			return err
		}
		// delete the time session itself:
		if err := tx.Where("id = ?", sessionID).Delete(&chModel.TimeSession{}).Error; err != nil {
			logging.FromContext(c).Errorf("Failed to delete time session %d for chore %d: %v", sessionID, choreID, err)
			return err
		}
		return nil

	})
	// delete where session ID matches and chore ID matches

}

// GetTombstonesSince returns global tombstones and tombstones scoped to userID.
func (r *ChoreRepository) GetTombstonesSince(c context.Context, circleID int, entityType syncModel.EntityType, userID int, since int64, limit int) ([]*syncModel.Tombstone, error) {
	var tombstones []*syncModel.Tombstone
	query := r.db.WithContext(c).
		Where("circle_id = ? AND entity_type = ? AND sync_version > ? AND (user_id IS NULL OR user_id = ?)", circleID, entityType, since, userID).
		Order("sync_version asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&tombstones).Error; err != nil {
		return nil, err
	}
	return tombstones, nil
}

// GetChoresHistoryByCircle retrieves chore histories for a circle, respecting privacy and optionally filtered by sync version.
// A history is visible if the user can see the associated chore (privacy rules apply).
// If syncOptions is nil or syncOptions.SyncVersion is nil, returns all visible histories ordered by performed_at DESC.
// If syncOptions.SyncVersion is set, returns only histories with sync_version > *SyncVersion, ordered by sync_version ASC.
func (r *ChoreRepository) GetChoresHistoryByCircle(c context.Context, circleID int, userID int, syncOptions *SyncOptions) ([]*chModel.ChoreHistory, error) {
	var histories []*chModel.ChoreHistory

	query := r.db.WithContext(c).
		Table("chore_histories").
		Select("chore_histories.*, time_sessions.duration").
		Joins("JOIN chores ON chores.id = chore_histories.chore_id").
		Joins("LEFT JOIN time_sessions ON time_sessions.chore_history_id = chore_histories.id").
		Joins("LEFT JOIN chore_assignees ON chores.id = chore_assignees.chore_id AND chore_assignees.user_id = ?", userID).
		Where("chores.circle_id = ?", circleID).
		Where(privacyPredicate(userID))

	// Sync-specific filtering if provided.
	if syncOptions != nil && syncOptions.SyncVersion != nil {
		query = query.Where("chore_histories.sync_version > ?", *syncOptions.SyncVersion).
			Order("chore_histories.sync_version asc").
			Limit(syncOptions.Limit + 1)
	} else {
		// Normal ordering when not syncing.
		query = query.Order("chore_histories.performed_at desc, chore_histories.updated_at desc")
	}

	if err := query.Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// GetUserLastChoreAction gets the user's most recent action on a chore (within time limit)
func (r *ChoreRepository) GetUserLastChoreAction(c context.Context, choreID int, userID int, timeLimit time.Duration) (*chModel.ChoreHistory, error) {
	var history chModel.ChoreHistory
	cutoffTime := time.Now().UTC().Add(-timeLimit)

	err := r.db.WithContext(c).
		Where("chore_id = ? AND completed_by = ? AND created_at > ?", choreID, userID, cutoffTime).
		Where("status IN (?)", []chModel.ChoreHistoryStatus{
			chModel.ChoreHistoryStatusCompleted,
			chModel.ChoreHistoryStatusSkipped,
			chModel.ChoreHistoryStatusPendingApproval,
			chModel.ChoreHistoryStatusRejected,
		}).
		Order("created_at desc").
		First(&history).Error

	if err != nil {
		return nil, err
	}
	return &history, nil
}

// UndoChoreAction undoes a chore action by restoring previous state and removing the history entry
func (r *ChoreRepository) UndoChoreAction(c context.Context, choreID int, historyID int, circleID int, previousAssignedTo *int, previousDueDate *time.Time) error {
	nextVersion, err := r.nextSyncVersion(c, circleID)
	if err != nil {
		return err
	}
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Get the history entry to undo
		var historyToUndo chModel.ChoreHistory
		if err := tx.First(&historyToUndo, historyID).Error; err != nil {
			return err
		}

		// Prepare chore updates
		choreUpdates := map[string]interface{}{
			"status":       chModel.ChoreStatusNoStatus,
			"sync_version": nextVersion,
		}

		// Build list of fields to explicitly update (including nullable fields)
		selectFields := []string{"status", "sync_version"}

		// Restore previous assignee
		if previousAssignedTo != nil {
			choreUpdates["assigned_to"] = *previousAssignedTo
		} else {
			choreUpdates["assigned_to"] = nil
		}
		selectFields = append(selectFields, "assigned_to")

		// Restore previous due date; always reactivate since the chore was
		// active before completion (e.g. "once" chores have no due date but
		// must be unarchived on undo).
		if previousDueDate != nil {
			choreUpdates["next_due_date"] = *previousDueDate
		} else {
			choreUpdates["next_due_date"] = nil
		}
		choreUpdates["is_active"] = true
		selectFields = append(selectFields, "next_due_date", "is_active")

		// Update the chore with explicit field selection to handle nil values
		if err := tx.Model(&chModel.Chore{}).Where("id = ?", choreID).Select(selectFields).Updates(choreUpdates).Error; err != nil {
			return err
		}

		// Remove points if they were added during completion/approval
		if historyToUndo.Points != nil && *historyToUndo.Points > 0 &&
			(historyToUndo.Status == chModel.ChoreHistoryStatusCompleted) {
			// Get the chore to find circle ID
			var chore chModel.Chore
			if err := tx.Select("circle_id").First(&chore, choreID).Error; err != nil {
				return err
			}
			// Subtract points from user
			if err := tx.Model(&cModel.UserCircle{}).
				Where("user_id = ? AND circle_id = ?", historyToUndo.CompletedBy, chore.CircleID).
				Update("points", gorm.Expr("points - ?", *historyToUndo.Points)).Error; err != nil {
				return err
			}
		}

		// Delete the history entry being undone
		if err := tx.Delete(&chModel.ChoreHistory{}, historyID).Error; err != nil {
			return err
		}

		// Delete associated time sessions for this history entry
		if err := tx.Where("chore_history_id = ?", historyID).Delete(&chModel.TimeSession{}).Error; err != nil {
			return err
		}

		return nil
	})
}
