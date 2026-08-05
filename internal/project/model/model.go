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
	IsPrivate   bool       `json:"isPrivate" gorm:"column:is_private;default:false"` // Whether the project is only visible to its creator
}

// CanView reports whether the user is allowed to see the project. Public projects
// are visible to every member of the circle, private ones only to their creator.
// Circle admins and managers do not bypass this, same as private chores today.
func (p *Project) CanView(userID int) bool {
	return !p.IsPrivate || p.CreatedBy == userID
}

type ProjectReq struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Icon        *string `json:"icon"`
	// IsPrivate is optional: when omitted on update the current value is kept, so
	// older clients that don't know about the field can't accidentally unset it.
	IsPrivate *bool `json:"isPrivate"`
}

type UpdateProjectReq struct {
	ID int `json:"id" binding:"required"`
	ProjectReq
}
