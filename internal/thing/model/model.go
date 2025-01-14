package model

import "time"

type Thing struct {
	ID          int          `json:"id" gorm:"primary_key"`
	UserID      int          `json:"userID" gorm:"column:user_id"`
	CircleID    int          `json:"circleId" gorm:"column:circle_id"`
	Name        string       `json:"name" gorm:"column:name"`
	State       string       `json:"state" gorm:"column:state"`
	Type        string       `json:"type" gorm:"column:type"`
	ThingChores []ThingChore `json:"thingChores" gorm:"foreignkey:ThingID;references:ID"`
	UpdatedAt   *time.Time   `json:"updatedAt" gorm:"column:updated_at"`
	CreatedAt   *time.Time   `json:"createdAt" gorm:"column:created_at"`
}

type ThingHistory struct {
	ID        int        `json:"id" gorm:"primary_key"`
	ThingID   int        `json:"thingId" gorm:"column:thing_id"`
	State     string     `json:"state" gorm:"column:state"`
	UpdatedAt *time.Time `json:"updatedAt" gorm:"column:updated_at"`
	CreatedAt *time.Time `json:"createdAt" gorm:"column:created_at"`
}

type ThingChore struct {
	ThingID      int    `json:"thingId" gorm:"column:thing_id;primaryKey;uniqueIndex:idx_thing_user"`
	ChoreID      int    `json:"choreId" gorm:"column:chore_id;primaryKey;uniqueIndex:idx_thing_user"`
	TriggerState string `json:"triggerState" gorm:"column:trigger_state"`
	Condition    string `json:"condition" gorm:"column:condition"`
}

type ThingTrigger struct {
	ID           int    `json:"thingID" binding:"required"`
	TriggerState string `json:"triggerState" binding:"required"`
	Condition    string `json:"condition"`
}
