package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

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
	RawEvent     JSONB                `json:"raw_event" gorm:"column:raw_event;type:jsonb"`
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
	NotificationPlatformWebhook
	NotificationPlatformDiscord
	NotificationPlatformFCM
)

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	value, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(value), nil
}

func (j *JSONB) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return errors.New("type assertion to []byte or string failed")
	}
}
