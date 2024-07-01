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
func (r *NotificationRepository) MarkNotificationsAsSent(notifications []*nModel.Notification) error {
	// Extract IDs from notifications
	var ids []int
	for _, notification := range notifications {
		ids = append(ids, notification.ID)
	}
	// Use the extracted IDs in the Where clause
	return r.db.Model(&nModel.Notification{}).Where("id IN (?)", ids).Update("is_sent", true).Error
}
func (r *NotificationRepository) GetPendingNotificaiton(c context.Context, lookback time.Duration) ([]*nModel.Notification, error) {
	var notifications []*nModel.Notification
	start := time.Now().UTC().Add(-lookback)
	end := time.Now().UTC()
	if err := r.db.Debug().Where("is_sent = ? AND scheduled_for < ? AND scheduled_for > ?", false, end, start).Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}
