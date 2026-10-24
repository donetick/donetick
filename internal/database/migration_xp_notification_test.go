package database

import (
	"path/filepath"
	"testing"

	user "donetick.com/core/internal/user/model"
	xp "donetick.com/core/internal/xp/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A one-sided resolution of the migration-list conflict drops either the
// notification preference table or the XP tables. Exercise the real migration
// and persisted rows so all three registrations must remain present.
func TestXPMigrationPreservesNotificationPreferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "xp-notifications.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, Migration(db))
	category := xp.Category{CircleID: 1, Name: "Household", NormalizedName: "household"}
	require.NoError(t, db.Create(&category).Error)
	award := xp.Award{MembershipID: 2, HistoryID: 3, CircleID: 1, UserID: 4, CategoryID: category.ID, Points: 25}
	require.NoError(t, db.Create(&award).Error)
	preference := user.UserNotificationPreference{
		UserID: 4, Profile: user.NotificationProfileQuiet, DigestHour: 0,
		Overrides: user.NotificationOverrides{user.NotificationEventDailyDigest: true},
	}
	require.NoError(t, db.Create(&preference).Error)

	// Reapplying the migration must preserve both independently added features.
	require.NoError(t, Migration(db))
	var savedCategory xp.Category
	require.NoError(t, db.First(&savedCategory, category.ID).Error)
	require.Equal(t, category, savedCategory)
	var savedAward xp.Award
	require.NoError(t, db.First(&savedAward, "history_id = ?", 3).Error)
	require.Equal(t, award, savedAward)
	var savedPreference user.UserNotificationPreference
	require.NoError(t, db.First(&savedPreference, "user_id = ?", 4).Error)
	require.Equal(t, user.NotificationProfileQuiet, savedPreference.Profile)
	require.Equal(t, 0, savedPreference.DigestHour)
	require.Equal(t, user.NotificationOverrides{user.NotificationEventDailyDigest: true}, savedPreference.Overrides)
}
