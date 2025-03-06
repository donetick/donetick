package model

import "time"

type SubTask struct {
	ID          int        `json:"id" gorm:"primary_key"`
	ChoreID     int        `json:"-" gorm:"column:chore_id;index"`
	OrderID     int8       `json:"orderId" gorm:"column:order_id"`
	Name        string     `json:"name" gorm:"column:name"`
	CompletedAt *time.Time `json:"completedAt" gorm:"column:completed_at"`
	CompletedBy int        `json:"completedBy" gorm:"column:completed_by"`
	ParentId    *int       `json:"parentId" gorm:"column:parent_id"`
}
