package user

import (
	"context"
	"time"

	nModel "donetick.com/core/internal/notifier/model"
	"gorm.io/gorm"
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

// DeleteChoreReminders resets due-date reminders without losing a pending assignment
// event during a subsequent edit. Events for a superseded assignee are discarded.
// Inspect JSON in Go to keep this portable across SQLite and Postgres.
func (r *NotificationRepository) DeleteChoreReminders(c context.Context, choreID int, assignedTo *int) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var notifications []nModel.Notification
		if err := tx.Where("chore_id = ?", choreID).Find(&notifications).Error; err != nil {
			return err
		}
		var ids []int
		for _, notification := range notifications {
			if notification.RawEvent["type"] == "on_assignment" && !notification.IsSent &&
				assignedTo != nil && notification.UserID == *assignedTo {
				continue
			}
			ids = append(ids, notification.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Where("id IN ?", ids).Delete(&nModel.Notification{}).Error
	})
}

func (r *NotificationRepository) BatchInsertNotifications(notifications []*nModel.Notification) error {
	return r.db.Create(&notifications).Error
}
func (r *NotificationRepository) MarkNotificationsAsSent(notifications []*nModel.NotificationDetails) error {
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
		// Reminders of chores in a project hidden from the circle are not mirrored to
		// the circle-wide webhook. Digest notifications (chore_id is a negative
		// sentinel, see service.DigestChoreIDDaily/Weekly -- they aren't about one
		// real chore) never are either: they're a push-only summary, not an event
		// about a specific chore a webhook consumer would expect.
		Select("notifications.*, CASE WHEN notifications.chore_id < 0 THEN NULL WHEN projects.is_private = true THEN NULL ELSE circles.webhook_url END as webhook_url, COALESCE(chores.priority, 0) as chore_priority").
		Joins("left join circles on circles.id = notifications.circle_id").
		Joins("left join chores on chores.id = notifications.chore_id").
		Joins("left join projects on projects.id = chores.project_id").
		Where("notifications.is_sent = ? AND notifications.scheduled_for < ? AND notifications.scheduled_for > ?", false, end, start).
		Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *NotificationRepository) DeleteSentNotifications(c context.Context, since time.Time) error {
	return r.db.WithContext(c).Where("is_sent = ? AND scheduled_for < ?", true, since).Delete(&nModel.Notification{}).Error
}

// ExistsNotification reports whether a notification with this sentinel choreID
// (a digest run uses a negative sentinel, see service.DigestChoreIDDaily /
// DigestChoreIDWeekly, since it isn't about one real chore) was already created
// for userID since the given time. Used to avoid sending the same digest twice
// if the digest scheduler job runs more than once inside a user's send hour.
func (r *NotificationRepository) ExistsNotification(c context.Context, userID int, choreID int, since time.Time) (bool, error) {
	var count int64
	if err := r.db.WithContext(c).Model(&nModel.Notification{}).
		Where("user_id = ? AND chore_id = ? AND created_at >= ?", userID, choreID, since).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
