package model

import "time"

// CalendarToken is a revocable, user-scoped bearer token granting read-only
// access to a user's iCal feed. Calendar clients cannot send auth headers, so
// the token travels in the URL path. Persisting it here (rather than deriving
// it deterministically from the JWT secret) is what makes per-user rotation and
// revocation possible without invalidating every session on the instance.
type CalendarToken struct {
	ID        int       `json:"id" gorm:"primary_key"`
	UserID    int       `json:"userId" gorm:"column:user_id;index"`
	Token     string    `json:"-" gorm:"column:token;size:64;uniqueIndex"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
	// LastUsedAt is refreshed at most once per TouchInterval so that polling
	// calendar clients don't turn every feed read into a write.
	LastUsedAt *time.Time `json:"lastUsedAt" gorm:"column:last_used_at"`
}
