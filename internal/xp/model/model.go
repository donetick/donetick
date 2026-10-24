package model

// Category IDs are stable integration keys. Archiving never deletes earned XP.
type Category struct {
	ID             int    `json:"id" gorm:"primaryKey"`
	CircleID       int    `json:"-" gorm:"not null;uniqueIndex:idx_xp_category_name"`
	Name           string `json:"name" gorm:"size:80;not null"`
	NormalizedName string `json:"-" gorm:"size:80;not null;uniqueIndex:idx_xp_category_name"`
	Archived       bool   `json:"archived" gorm:"not null;default:false"`
}

func (Category) TableName() string { return "xp_categories" }

// Award is an immutable attribution snapshot. It intentionally has no cascading
// foreign keys: task/history deletion must retain lifetime points attribution.
// Undo explicitly deletes the matching award in its points transaction.
type Award struct {
	MembershipID int `gorm:"not null;index"`
	HistoryID    int `gorm:"primaryKey;autoIncrement:false"`
	CircleID     int `gorm:"not null;index:idx_xp_awards_member"`
	UserID       int `gorm:"not null;index:idx_xp_awards_member"`
	CategoryID   int `gorm:"not null"`
	Points       int `gorm:"not null"`
}

func (Award) TableName() string { return "xp_awards" }
