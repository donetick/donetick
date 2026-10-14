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
		// the circle-wide webhook.
		Select("notifications.*, CASE WHEN projects.is_private = true THEN NULL ELSE circles.webhook_url END as webhook_url").
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
