package model

import "time"

// XPCategory belongs to a circle. Archiving retains its identity and past awards.
type XPCategory struct {
	ID        int       `json:"id" gorm:"primary_key"`
	CircleID  int       `json:"circleId" gorm:"not null;uniqueIndex:idx_xp_category_name"`
	Name      string    `json:"name"`
	NameKey   string    `json:"-" gorm:"not null;uniqueIndex:idx_xp_category_name"`
	Archived  bool      `json:"archived" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"createdAt"`
}

// XPEntry supplements the existing earned/redeemed counters with immutable
// category deltas. It survives deletion of chore history, just as points do.
type XPEntry struct {
	ID           int       `json:"id" gorm:"primary_key"`
	MembershipID int       `json:"-" gorm:"not null;index"`
	CircleID     int       `json:"circleId" gorm:"not null;index:idx_xp_member"`
	UserID       int       `json:"userId" gorm:"not null;index:idx_xp_member"`
	CategoryID   *int      `json:"categoryId" gorm:"index"`
	Earned       int       `json:"earned"`
	Redeemed     int       `json:"redeemed"`
	EventKey     string    `json:"-" gorm:"not null;uniqueIndex"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (XPCategory) TableName() string { return "xp_categories" }
func (XPEntry) TableName() string    { return "xp_entries" }

type XPCategoryTotal struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Balance  int64  `json:"balance"`
	Earned   int64  `json:"earned"`
	Redeemed int64  `json:"redeemed"`
}

type XPStats struct {
	UserID          int               `json:"user_id"`
	CircleID        int               `json:"circle_id"`
	TotalXP         int64             `json:"total_xp"`
	LifetimeXP      int64             `json:"lifetime_xp"`
	RedeemedXP      int64             `json:"redeemed_xp"`
	Categories      map[string]int64  `json:"categories"`
	CategoryDetails []XPCategoryTotal `json:"category_details"`
}
