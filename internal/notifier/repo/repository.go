package user

import (
	"context"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db}
}

func (r *NotificationRepository) DeleteAllChoreNotifications(choreID int) error {
	return r.db.Where("chore_id = ?", choreID).Delete(&nModel.Notification{}).Error
}

func (r *NotificationRepository) DeleteChoreReminders(choreID int) error {
	return r.db.Where("chore_id = ? AND nag_expires_at IS NULL", choreID).Delete(&nModel.Notification{}).Error
}

type OverdueNaggingChore struct {
	chModel.Chore
	LastNagDueDate *time.Time
	LastNagSlot    *time.Time
	NagQueuedCount int
}

func (r *NotificationRepository) GetOverdueNaggingChores(c context.Context, now time.Time) ([]OverdueNaggingChore, error) {
	var chores []OverdueNaggingChore
	configured := "json_extract(chores.notification_meta_v2, '$.overdueNagging') IS NOT NULL"
	if r.db.Dialector.Name() == "postgres" {
		configured = "chores.notification_meta_v2->>'overdueNagging' IS NOT NULL"
	}
	err := r.db.WithContext(c).Table("chores").
		Select("chores.*, overdue_nag_slots.due_date AS last_nag_due_date, overdue_nag_slots.scheduled_for AS last_nag_slot, overdue_nag_slots.queued_count AS nag_queued_count").
		Joins("LEFT JOIN overdue_nag_slots ON overdue_nag_slots.chore_id = chores.id").
		Where("chores.is_active = ? AND chores.next_due_date < ?", true, now).
		Where(configured).Find(&chores).Error
	return chores, err
}

func (r *NotificationRepository) GetChoreForNagging(c context.Context, choreID int) (*chModel.Chore, error) {
	var chore chModel.Chore
	err := r.db.WithContext(c).Preload("Project").Preload("Assignees").First(&chore, choreID).Error
	return &chore, err
}

// QueueOverdueNagging commits the slot and its deliveries together. Only one
// scheduler can advance a task to a given slot.
func (r *NotificationRepository) QueueOverdueNagging(c context.Context, chore *chModel.Chore, scheduledFor time.Time, notifications []nModel.Notification) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var current chModel.Chore
		err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sync_version = ? AND is_active = ? AND next_due_date = ?", chore.SyncVersion, true, chore.NextDueDate).
			Where("status IS NULL OR status <> ?", chModel.ChoreStatusPendingApproval).
			First(&current, chore.ID).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		slot := nModel.OverdueNagSlot{ChoreID: chore.ID, DueDate: *chore.NextDueDate, ScheduledFor: scheduledFor, QueuedCount: 1}
		updates := clause.AssignmentColumns([]string{"due_date", "scheduled_for"})
		updates = append(updates, clause.Assignment{
			Column: clause.Column{Name: "queued_count"},
			Value:  gorm.Expr("CASE WHEN overdue_nag_slots.due_date = excluded.due_date THEN overdue_nag_slots.queued_count + 1 ELSE 1 END"),
		})
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chore_id"}},
			DoUpdates: updates,
			Where: clause.Where{Exprs: []clause.Expression{gorm.Expr(
				"overdue_nag_slots.due_date <> excluded.due_date OR (overdue_nag_slots.scheduled_for < excluded.scheduled_for AND (? OR overdue_nag_slots.queued_count < ?))",
				chore.NotificationMetadataV2.OverdueNagging.ContinueUntilComplete,
				chore.NotificationMetadataV2.OverdueNagging.MaxFollowUps,
			)}},
		}).Create(&slot)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}

		for _, notification := range notifications {
			var matchingReminder int64
			if err := tx.Model(&nModel.Notification{}).
				Where("chore_id = ? AND scheduled_for = ? AND type = ? AND target_id = ? AND nag_expires_at IS NULL", chore.ID, scheduledFor, notification.TypeID, notification.TargetID).
				Count(&matchingReminder).Error; err != nil {
				return err
			}
			if matchingReminder == 0 {
				if err := tx.Create(&notification).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *NotificationRepository) ClaimNag(c context.Context, id int, now time.Time) (bool, error) {
	result := r.db.WithContext(c).Model(&nModel.Notification{}).
		Where("id = ? AND is_sent = ? AND nag_expires_at > ?", id, false, now).
		Where("(nag_claimed_until IS NULL OR nag_claimed_until <= ?)", now).
		Update("nag_claimed_until", now.Add(3*time.Minute))
	return result.RowsAffected == 1, result.Error
}

func (r *NotificationRepository) DeleteExpiredNags(c context.Context, now time.Time) error {
	return r.db.WithContext(c).Where("is_sent = ? AND nag_expires_at <= ?", false, now).Delete(&nModel.Notification{}).Error
}

func (r *NotificationRepository) DeleteOrphanedNagSlots(c context.Context) error {
	return r.db.WithContext(c).
		Where("NOT EXISTS (SELECT 1 FROM chores WHERE chores.id = overdue_nag_slots.chore_id)").
		Delete(&nModel.OverdueNagSlot{}).Error
}

func (r *NotificationRepository) ReleaseNagClaim(c context.Context, id int) error {
	return r.db.WithContext(c).Model(&nModel.Notification{}).Where("id = ?", id).Update("nag_claimed_until", nil).Error
}

func (r *NotificationRepository) GetPendingNags(c context.Context, now time.Time) ([]*nModel.NotificationDetails, error) {
	var notifications []*nModel.NotificationDetails
	err := r.db.WithContext(c).Table("notifications").
		Where("is_sent = ? AND scheduled_for <= ? AND nag_expires_at > ?", false, now, now).
		Where("(nag_claimed_until IS NULL OR nag_claimed_until <= ?)", now).
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepository) BatchInsertNotifications(notifications []*nModel.Notification) error {
	return r.db.Create(&notifications).Error
}
func (r *NotificationRepository) MarkNotificationsAsSent(notifications []*nModel.NotificationDetails) error {
	if len(notifications) == 0 {
		return nil
	}
	// Extract IDs from notifications
	var ids []int
	for _, notification := range notifications {
		ids = append(ids, notification.ID)
	}
	// Use the extracted IDs in the Where clause
	return r.db.Model(&nModel.Notification{}).Where("id IN (?)", ids).Update("is_sent", true).Error
}
func (r *NotificationRepository) GetPendingNotification(c context.Context, lookback time.Duration) ([]*nModel.NotificationDetails, error) {
	var notifications []*nModel.NotificationDetails
	start := time.Now().UTC().Add(-lookback)
	end := time.Now().UTC()
	if err := r.db.Table("notifications").
		Select("notifications.*, circles.webhook_url as webhook_url").
		Joins("left join circles on circles.id = notifications.circle_id").
		Where("notifications.nag_expires_at IS NULL AND notifications.is_sent = ? AND notifications.scheduled_for < ? AND notifications.scheduled_for > ?", false, end, start).
		Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *NotificationRepository) DeleteSentNotifications(c context.Context, since time.Time) error {
	return r.db.WithContext(c).Where("is_sent = ? AND scheduled_for < ?", true, since).Delete(&nModel.Notification{}).Error
}
