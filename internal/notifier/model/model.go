package model

import "time"

type Notification struct {
	ID           int                  `json:"id" gorm:"primaryKey"`
	ChoreID      int                  `json:"chore_id" gorm:"column:chore_id"`
	CircleID     int                  `json:"circle_id" gorm:"column:circle_id"`
	UserID       int                  `json:"user_id" gorm:"column:user_id"`
	TargetID     string               `json:"target_id" gorm:"column:target_id"`
	Text         string               `json:"text" gorm:"column:text"`
	IsSent       bool                 `json:"is_sent" gorm:"column:is_sent;index;default:false"`
	TypeID       NotificationPlatform `json:"type" gorm:"column:type"`
	ScheduledFor time.Time            `json:"scheduled_for" gorm:"column:scheduled_for;index"`
	CreatedAt    time.Time            `json:"created_at" gorm:"column:created_at"`
	RawEvent     interface{}          `json:"raw_event" gorm:"column:raw_event;type:jsonb"`
}
type NotificationDetails struct {
	Notification
	WebhookURL *string `json:"webhook_url" gorm:"column:webhook_url;<-:null"` // read-only, will only be used if webhook enabled

}

func (n *Notification) IsValid() bool {
	return true
}

type NotificationPlatform int8

const (
	NotificationPlatformNone NotificationPlatform = iota
	NotificationPlatformTelegram
	NotificationPlatformPushover
)
