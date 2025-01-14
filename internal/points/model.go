package model

import "time"

type PointsHistory struct {
	ID        int                 `json:"id" gorm:"primary_key"`                   // Unique identifier
	Action    PointsHistoryAction `json:"action" gorm:"column:action"`             // Action
	Points    int                 `json:"points" gorm:"column:points"`             // Points
	CreatedAt time.Time           `json:"created_at" gorm:"column:created_at"`     // Created at
	CreatedBy int                 `json:"created_by" gorm:"column:created_by"`     // Created by
	UserID    int                 `json:"user_id" gorm:"column:user_id;index"`     // User ID
	CircleID  int                 `json:"circle_id" gorm:"column:circle_id;index"` // Circle ID with index
}

type PointsHistoryAction int8

const (
	PointsHistoryActionAdd PointsHistoryAction = iota
	PointsHistoryActionRemove
	PointsHistoryActionRedeem
)
