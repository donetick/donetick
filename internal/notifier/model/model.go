package model

import "time"

type Notification struct {
	ID           int              `json:"id" gorm:"primaryKey"`
	ChoreID      int              `json:"chore_id" gorm:"column:chore_id"`
	UserID       int              `json:"user_id" gorm:"column:user_id"`
	TargetID     string           `json:"target_id" gorm:"column:target_id"`
	Text         string           `json:"text" gorm:"column:text"`
	IsSent       bool             `json:"is_sent" gorm:"column:is_sent;index;default:false"`
	TypeID       NotificationType `json:"type" gorm:"column:type"`
	ScheduledFor time.Time        `json:"scheduled_for" gorm:"column:scheduled_for;index"`
	CreatedAt    time.Time        `json:"created_at" gorm:"column:created_at"`
}

func (n *Notification) IsValid() bool {
	switch n.TypeID {
	case NotificationTypeTelegram, NotificationTypePushover:
		if n.TargetID == "" {
			return false
		} else if n.Text == "0" {
			return false
		}
		return true
	case NotificationTypeMqtt:
		return true
	default:
		return false
	}
}

type NotificationType int8

const (
	NotificationTypeNone NotificationType = iota
	NotificationTypeTelegram
	NotificationTypePushover
	NotificationTypeMqtt
)
