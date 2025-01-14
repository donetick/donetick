package model

type Label struct {
	ID        int    `json:"id" gorm:"primary_key"`
	Name      string `json:"name" gorm:"column:name"`
	Color     string `json:"color" gorm:"column:color"`
	CircleID  *int   `json:"-" gorm:"column:circle_id"`
	CreatedBy int    `json:"created_by" gorm:"column:created_by"`
}

type LabelReq struct {
	LabelID int `json:"id" binding:"required"`
}
