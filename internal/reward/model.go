package model

import "time"

type Reward struct {
	ID          int       `json:"id" gorm:"primary_key"`
	CircleID    int       `json:"circleId" gorm:"column:circle_id;index"`
	Name        string    `json:"name" gorm:"column:name"`
	Description string    `json:"description" gorm:"column:description"`
	Cost        int       `json:"cost" gorm:"column:cost"`
	CreatedBy   int       `json:"createdBy" gorm:"column:created_by"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}
