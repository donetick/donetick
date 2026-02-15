package model

import "time"

type Project struct {
	ID          int        `json:"id" gorm:"primary_key"`
	Name        string     `json:"name" gorm:"column:name;not null"`
	Description *string    `json:"description" gorm:"column:description"`
	Color       *string    `json:"color" gorm:"column:color"`
	Icon        *string    `json:"icon" gorm:"column:icon"`
	CircleID    int        `json:"circleId" gorm:"column:circle_id;index;not null"`
	CreatedBy   int        `json:"created_by" gorm:"column:created_by;not null"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty" gorm:"column:updated_at;autoUpdateTime"`
	IsDefault   bool       `json:"isDefault" gorm:"column:is_default;default:false"`
}

type ProjectReq struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Icon        *string `json:"icon"`
}

type UpdateProjectReq struct {
	ID int `json:"id" binding:"required"`
	ProjectReq
}
