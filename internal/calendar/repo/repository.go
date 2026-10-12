package repo

import (
	"context"
	"time"

	calModel "donetick.com/core/internal/calendar/model"
	"gorm.io/gorm"
)

// TouchInterval is the minimum gap between last_used_at writes for a token.
const TouchInterval = time.Hour

type CalendarRepository struct {
	db *gorm.DB
}

func NewCalendarRepository(db *gorm.DB) *CalendarRepository {
	return &CalendarRepository{db: db}
}

// GetByUserID returns the user's active calendar token. Returns
// gorm.ErrRecordNotFound when the user has never generated one.
func (r *CalendarRepository) GetByUserID(c context.Context, userID int) (*calModel.CalendarToken, error) {
	var token calModel.CalendarToken
	if err := r.db.WithContext(c).Where("user_id = ?", userID).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// GetByToken looks up a token by its secret value. Returns
// gorm.ErrRecordNotFound for unknown or revoked tokens.
func (r *CalendarRepository) GetByToken(c context.Context, token string) (*calModel.CalendarToken, error) {
	var found calModel.CalendarToken
	if err := r.db.WithContext(c).Where("token = ?", token).First(&found).Error; err != nil {
		return nil, err
	}
	return &found, nil
}

// Replace stores a new token for the user, deleting any existing one so that a
// user has exactly one active calendar URL and rotation revokes the old URL
// atomically.
func (r *CalendarRepository) Replace(c context.Context, userID int, token string) (*calModel.CalendarToken, error) {
	stored := &calModel.CalendarToken{UserID: userID, Token: token}

	if err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&calModel.CalendarToken{}).Error; err != nil {
			return err
		}
		return tx.Create(stored).Error
	}); err != nil {
		return nil, err
	}

	return stored, nil
}

// DeleteByUserID revokes the user's calendar URL.
func (r *CalendarRepository) DeleteByUserID(c context.Context, userID int) error {
	return r.db.WithContext(c).Where("user_id = ?", userID).Delete(&calModel.CalendarToken{}).Error
}

// TouchLastUsed records that the token served a feed, writing at most once per
// TouchInterval. The WHERE clause does the staleness check so this stays a
// single unconditional UPDATE on an unauthenticated code path.
func (r *CalendarRepository) TouchLastUsed(c context.Context, tokenID int) error {
	now := time.Now().UTC()
	return r.db.WithContext(c).Model(&calModel.CalendarToken{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", tokenID, now.Add(-TouchInterval)).
		Update("last_used_at", now).Error
}
