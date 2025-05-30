package repo

import (
	"context"
	"fmt"
	"log"
	"time"

	stModel "donetick.com/core/internal/subtask/model"
	"gorm.io/gorm"
)

type SubTasksRepository struct {
	db *gorm.DB
}

func NewSubTasksRepository(db *gorm.DB) *SubTasksRepository {
	return &SubTasksRepository{db}
}

func (r *SubTasksRepository) UpdateSubtask(c context.Context, choreId int, toBeRemoved []stModel.SubTask, toBeAddedOrUpdated []stModel.SubTask) error {
	// Start a database transaction. All operations within this function will be atomic. so if something wrong will just rollback
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if len(toBeRemoved) > 0 {
			var idsToRemove []int
			for _, subtask := range toBeRemoved {
				if subtask.ID > 0 {
					idsToRemove = append(idsToRemove, subtask.ID)
				}

			}
			if len(idsToRemove) > 0 {
				if err := tx.Where("chore_id = ? AND id IN ?", choreId, idsToRemove).Delete(&stModel.SubTask{}).Error; err != nil {
					log.Printf("Error deleting subtasks for chore %d: %v", choreId, err)
					return fmt.Errorf("failed to delete subtasks: %w", err) // Rollback
				}
			}
		}

		if len(toBeAddedOrUpdated) > 0 {
			var insertions []*stModel.SubTask
			var updates []*stModel.SubTask

			// temporary frontend ID -> pointer to the SubTask object being inserted
			tempIdObjectMap := make(map[int]*stModel.SubTask)
			// pointer to inserted SubTask object -> its original temporary frontend ID
			objectToTempIdMap := make(map[*stModel.SubTask]int)
			//  original ID (temp or real) -> original ParentId pointer received from frontend
			originalParentIdMap := make(map[int]*int)

			for i := range toBeAddedOrUpdated {
				subtask := &toBeAddedOrUpdated[i]

				subtask.ChoreID = choreId

				if subtask.ID <= 0 {

					tempId := subtask.ID
					tempIdObjectMap[tempId] = subtask
					objectToTempIdMap[subtask] = tempId
					originalParentIdMap[tempId] = subtask.ParentId

					subtask.ID = 0
					insertions = append(insertions, subtask)
				} else {
					originalParentIdMap[subtask.ID] = subtask.ParentId
					updates = append(updates, subtask)
				}
			}

			if len(insertions) > 0 {
				if err := tx.Create(&insertions).Error; err != nil {
					log.Printf("Error creating new subtasks for chore %d: %v", choreId, err)
					return fmt.Errorf("failed to create new subtasks: %w", err)
				}
			}

			tempToRealIdMap := make(map[int]int) //temporary frontend ID -> real database ID
			for tempId, insertedSubtask := range tempIdObjectMap {

				tempToRealIdMap[tempId] = insertedSubtask.ID
			}

			// First, update the parentIds for newly inserted subtasks if needed
			for _, subtask := range insertions {
				tempId := objectToTempIdMap[subtask]
				originalParentIdPtr, hasOriginalParent := originalParentIdMap[tempId]

				// Check if the *original* parent ID was temporary (negative).
				if hasOriginalParent && originalParentIdPtr != nil && *originalParentIdPtr <= 0 {
					tempParentId := *originalParentIdPtr
					realParentId, found := tempToRealIdMap[tempParentId]
					if found {
						// resolved the temporary parent ID to a real ID...now update the subtask's ParentId field.
						subtask.ParentId = &realParentId

						// Update the newly inserted subtask's parent_id only
						if err := tx.Model(&stModel.SubTask{}).Where("id = ? AND chore_id = ?", subtask.ID, choreId).
							Update("parent_id", subtask.ParentId).Error; err != nil {
							log.Printf("Error updating parent_id for newly inserted subtask ID %d for chore %d: %v",
								subtask.ID, choreId, err)
							return fmt.Errorf("failed to update parent_id for subtask %d: %w", subtask.ID, err)
						}
					} else {
						// Error Case: Frontend sent a temporary parent ID not in this batch.
						log.Printf("Warning: Temporary parent ID %d for subtask '%s' (Original Key: %d, Real ID: %d) not found in the current batch insert for chore %d. Setting ParentId to NULL.",
							tempParentId, subtask.Name, tempId, subtask.ID, choreId)
						subtask.ParentId = nil
						return fmt.Errorf("invalid temporary parent ID %d provided for subtask with original key %d", tempParentId, tempId)
					}
				}
			}

			// Process only existing subtasks that need updating
			for _, subtask := range updates {
				// It must be an update, use its real ID
				originalMapKey := subtask.ID

				originalParentIdPtr, hasOriginalParent := originalParentIdMap[originalMapKey]

				// Check if the *original* parent ID was temporary (negative).
				if hasOriginalParent && originalParentIdPtr != nil && *originalParentIdPtr <= 0 {
					tempParentId := *originalParentIdPtr
					realParentId, found := tempToRealIdMap[tempParentId]
					if found {
						// resolved the temporary parent ID to a real ID...now update the subtask's ParentId field.
						subtask.ParentId = &realParentId
					} else {
						// Error Case: Frontend sent a temporary parent ID not in this batch.
						log.Printf("Warning: Temporary parent ID %d for subtask '%s' (Original Key: %d, Real ID: %d) not found in the current batch insert for chore %d. Setting ParentId to NULL.",
							tempParentId, subtask.Name, originalMapKey, subtask.ID, choreId)
						subtask.ParentId = nil
						return fmt.Errorf("invalid temporary parent ID %d provided for subtask with original key %d", tempParentId, originalMapKey) // Rollback
					}
				}

				updateValues := map[string]interface{}{
					"name":         subtask.Name,
					"order_id":     subtask.OrderID,
					"completed_at": subtask.CompletedAt,
					"completed_by": subtask.CompletedBy,
					"parent_id":    subtask.ParentId, // Use the potentially resolved ParentId
				}

				// Perform the update operation for this subtask using its real ID.
				if err := tx.Model(&stModel.SubTask{}).Where("id = ? and chore_id = ?", subtask.ID, choreId).Updates(updateValues).Error; err != nil {
					log.Printf("Error updating subtask ID %d (Original Key: %d) for chore %d: %v. Values: %+v", subtask.ID, originalMapKey, choreId, err, updateValues)
					return fmt.Errorf("failed to update subtask %d: %w", subtask.ID, err) // Rollback
				}
			}
		}

		// If all operations completed without error, the transaction is committed.
		return nil // Commit
	})
}
func (r *SubTasksRepository) UpdateSubTaskStatus(c context.Context, userID int, subtaskID int, completedAt *time.Time) error {
	return r.db.Model(&stModel.SubTask{}).Where("id = ?", subtaskID).Updates(map[string]interface{}{
		"completed_at": completedAt,
		"completed_by": userID,
	}).Error
}

func (r *SubTasksRepository) ResetSubtasksCompletion(c context.Context, choreID int) error {
	return r.db.Model(&stModel.SubTask{}).Where("chore_id = ?", choreID).Updates(map[string]interface{}{
		"completed_at": nil,
		"completed_by": nil,
	}).Error
}
