package repo

import (
	"context"
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

func (r *SubTasksRepository) CreateSubtasks(c context.Context, tx *gorm.DB, subtasks *[]stModel.SubTask, choreID int) error {
	if tx != nil {
		return tx.Model(&stModel.SubTask{}).Save(subtasks).Error
	}
	return r.db.WithContext(c).Save(subtasks).Error
}
func (r *SubTasksRepository) UpdateSubtask(c context.Context, choreId int, toBeRemove []stModel.SubTask, toBeAdd []stModel.SubTask) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if len(toBeRemove) == 0 && len(toBeAdd) == 0 {
			return nil
		}

		if len(toBeRemove) > 0 {
			if err := tx.Delete(toBeRemove).Error; err != nil {
				return err
			}
		}
		if len(toBeAdd) > 0 {
			var insertions []stModel.SubTask
			var updates []stModel.SubTask
			for _, subtask := range toBeAdd {
				if subtask.ID <= 0 {
					// we interpret this as a new subtask
					subtask.ID = 0
					insertions = append(insertions, subtask)
				} else {
					updates = append(updates, subtask)
				}
			}

			if len(insertions) > 0 {
				if err := tx.Create(&insertions).Error; err != nil {
					return err
				}
			}
			if len(updates) > 0 {
				for _, subtask := range updates {
					values := map[string]interface{}{
						"name":         subtask.Name,
						"order_id":     subtask.OrderID,
						"completed_at": subtask.CompletedAt,
						"completed_by": subtask.CompletedBy,
						"parent_id":    subtask.ParentId,
					}
					if err := tx.Model(&stModel.SubTask{}).Where("chore_id = ? AND id = ?", choreId, subtask.ID).Updates(values).Error; err != nil {
						return err
					}
				}
			}

		}
		return nil
	})
}
func (r *SubTasksRepository) DeleteSubtask(c context.Context, tx *gorm.DB, subtaskID int) error {
	if tx != nil {
		return r.deleteSubtaskWithChildren(c, tx, subtaskID)
	}
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		return r.deleteSubtaskWithChildren(c, tx, subtaskID)
	})
}

func (r *SubTasksRepository) deleteSubtaskWithChildren(c context.Context, tx *gorm.DB, subtaskID int) error {
	var childSubtasks []stModel.SubTask
	if err := tx.Where("parent_id = ?", subtaskID).Find(&childSubtasks).Error; err != nil {
		return err
	}

	for _, child := range childSubtasks {
		if err := r.deleteSubtaskWithChildren(c, tx, child.ID); err != nil {
			return err
		}
	}

	return tx.Delete(&stModel.SubTask{}, subtaskID).Error
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
