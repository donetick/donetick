package bridge

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const settingsRowID = 1

// SettingsRepository persists the single Settings row. A dedicated small
// repository (rather than reusing a generic key-value store) keeps the
// instance-token column's access pattern explicit and auditable.
type SettingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get returns the current settings row, or (nil, nil) if Bridge has never
// been configured through Core's settings API (i.e. config-only so far).
func (r *SettingsRepository) Get(c context.Context) (*Settings, error) {
	var s Settings
	err := r.db.WithContext(c).First(&s, settingsRowID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Upsert saves s as the single settings row.
func (r *SettingsRepository) Upsert(c context.Context, s *Settings) error {
	s.ID = settingsRowID
	return r.db.WithContext(c).Save(s).Error
}

// Clear removes the settings row (disconnect). Core falls back to
// config.BridgeConfig (likely disabled) after this.
func (r *SettingsRepository) Clear(c context.Context) error {
	return r.db.WithContext(c).Delete(&Settings{}, settingsRowID).Error
}
