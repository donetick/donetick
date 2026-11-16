package user

import (
	"context"
	"fmt"
	"testing"
	"time"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newNotificationPreferenceTestRepository(t *testing.T) *UserRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.UserNotificationPreference{}))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return NewUserRepository(db, cfg)
}

func TestGetNotificationPreferenceReturnsBalancedDefaultWhenUnset(t *testing.T) {
	repo := newNotificationPreferenceTestRepository(t)

	pref, err := repo.GetNotificationPreference(context.Background(), 42)
	require.NoError(t, err)

	if pref.Profile != uModel.NotificationProfileBalanced {
		t.Errorf("expected default profile %q, got %q", uModel.NotificationProfileBalanced, pref.Profile)
	}
	if pref.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", pref.UserID)
	}
}

func TestUpsertNotificationPreferenceRoundTrips(t *testing.T) {
	repo := newNotificationPreferenceTestRepository(t)
	c := context.Background()

	err := repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  7,
		Profile: uModel.NotificationProfileQuiet,
		Overrides: uModel.NotificationOverrides{
			uModel.NotificationEventOverdueFollowup: true,
		},
	})
	require.NoError(t, err)

	pref, err := repo.GetNotificationPreference(c, 7)
	require.NoError(t, err)

	if pref.Profile != uModel.NotificationProfileQuiet {
		t.Errorf("expected profile quiet, got %q", pref.Profile)
	}
	if !pref.Enabled(uModel.NotificationEventOverdueFollowup) {
		t.Error("expected the saved override (overdue_followup=true) to persist through the round trip")
	}

	// Saving again for the same user must replace, not duplicate, the row.
	require.NoError(t, repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  7,
		Profile: uModel.NotificationProfileBalanced,
	}))
	updated, err := repo.GetNotificationPreference(c, 7)
	require.NoError(t, err)
	if updated.Profile != uModel.NotificationProfileBalanced {
		t.Errorf("expected profile to be updated to balanced, got %q", updated.Profile)
	}
}

func TestGetNotificationPreferencesBatchFillsDefaultsForMissingUsers(t *testing.T) {
	repo := newNotificationPreferenceTestRepository(t)
	c := context.Background()

	require.NoError(t, repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  1,
		Profile: uModel.NotificationProfilePersistent,
	}))

	prefs, err := repo.GetNotificationPreferences(c, []int{1, 2})
	require.NoError(t, err)

	if len(prefs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(prefs))
	}
	if prefs[1].Profile != uModel.NotificationProfilePersistent {
		t.Errorf("expected user 1's saved profile to be returned, got %q", prefs[1].Profile)
	}
	if prefs[2].Profile != uModel.NotificationProfileBalanced {
		t.Errorf("expected user 2 (no saved row) to default to balanced, got %q", prefs[2].Profile)
	}
}

func TestGetAllUserDigestSettingsReturnsSavedAndDefaultPreferences(t *testing.T) {
	repo := newNotificationPreferenceTestRepository(t)
	c := context.Background()

	savedUser := &uModel.User{Username: "saved", Timezone: "America/New_York"}
	defaultUser := &uModel.User{Username: "default", Timezone: "UTC"}
	require.NoError(t, repo.db.WithContext(c).Create(savedUser).Error)
	require.NoError(t, repo.db.WithContext(c).Create(defaultUser).Error)
	require.NoError(t, repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  savedUser.ID,
		Profile: uModel.NotificationProfileCustom,
		Overrides: uModel.NotificationOverrides{
			uModel.NotificationEventDailyDigest:  false,
			uModel.NotificationEventWeeklyDigest: true,
		},
		DigestHour: 11,
	}))

	settings, err := repo.GetAllUserDigestSettings(c)
	require.NoError(t, err)
	require.Len(t, settings, 2)

	byID := make(map[int]uModel.UserDigestSettings, len(settings))
	for _, setting := range settings {
		byID[setting.ID] = setting
	}
	saved := byID[savedUser.ID]
	require.Equal(t, "America/New_York", saved.Timezone)
	require.Equal(t, uModel.NotificationProfileCustom, saved.Profile)
	require.Equal(t, 11, saved.DigestHour)
	require.False(t, saved.Enabled(uModel.NotificationEventDailyDigest))
	require.True(t, saved.Enabled(uModel.NotificationEventWeeklyDigest))

	defaults := byID[defaultUser.ID]
	require.Equal(t, "UTC", defaults.Timezone)
	require.Equal(t, uModel.NotificationProfileDefault, defaults.Profile)
	require.Equal(t, uModel.DefaultDigestHour, defaults.DigestHour)
	require.True(t, defaults.Enabled(uModel.NotificationEventDailyDigest))
	require.False(t, defaults.Enabled(uModel.NotificationEventWeeklyDigest))
}

// Both digest types are plus-only, so GetAllUserDigestSettings resolves
// subscription state itself and drops the users who can't receive one: the
// saved profile outlives the subscription, and the digest planner has nothing
// else to go on.
func TestGetAllUserDigestSettingsGatesDigestsOnSubscription(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.UserNotificationPreference{}))
	cfg := &config.Config{IsDoneTickDotCom: true}
	cfg.Database.Type = "sqlite"
	repo := NewUserRepository(db, cfg)
	c := context.Background()

	// Both ask for the digests; only one is still paying.
	for id, isPlus := range map[int]bool{30: true, 31: false} {
		require.NoError(t, db.Create(&uModel.User{
			ID: id, Username: fmt.Sprintf("u%d", id), DisplayName: "U", Timezone: "UTC",
		}).Error)
		expiration := time.Now().UTC().Add(-time.Hour)
		if isPlus {
			expiration = time.Now().UTC().Add(time.Hour)
		}
		require.NoError(t, db.Exec("UPDATE users SET expiration = ? WHERE id = ?", expiration, id).Error)
		require.NoError(t, repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
			UserID: id, Profile: uModel.NotificationProfilePlanner, DigestHour: 9,
		}))
	}
	// A third user never configured anything and has no subscription.
	require.NoError(t, db.Create(&uModel.User{ID: 32, Username: "u32", DisplayName: "U", Timezone: "UTC"}).Error)

	settings, err := repo.GetAllUserDigestSettings(c)
	require.NoError(t, err)
	byID := make(map[int]uModel.UserDigestSettings, len(settings))
	for _, s := range settings {
		byID[s.ID] = s
	}
	// Only the paying user is a digest candidate at all.
	require.Len(t, byID, 1)
	require.Contains(t, byID, 30)
	require.True(t, byID[30].Enabled(uModel.NotificationEventDailyDigest))
	require.True(t, byID[30].Enabled(uModel.NotificationEventWeeklyDigest))

	// 31 still has Planner saved from when they were paying, and 32 never
	// configured anything -- neither can be digested.
	require.NotContains(t, byID, 31, "a lapsed subscription stops the digests")
	require.NotContains(t, byID, 32)

	// The row itself is untouched, so resubscribing restores the posture.
	lapsed, err := repo.GetNotificationPreference(c, 31)
	require.NoError(t, err)
	require.Equal(t, uModel.NotificationProfilePlanner, lapsed.Profile)
}

// Midnight is a legitimate digest hour, and a column default on DigestHour
// would make gorm drop the zero value from the insert and store 8 instead.
func TestUpsertNotificationPreferencePersistsMidnightDigestHour(t *testing.T) {
	repo := newNotificationPreferenceTestRepository(t)
	c := context.Background()

	// GetAllUserDigestSettings drives off the users table, so the row has to
	// exist for the join below to see the preference at all.
	user := &uModel.User{ID: 11, Username: "midnight", DisplayName: "Midnight", Timezone: "UTC"}
	require.NoError(t, repo.db.Create(user).Error)

	require.NoError(t, repo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:     11,
		Profile:    uModel.NotificationProfilePlanner,
		DigestHour: 0,
	}))

	pref, err := repo.GetNotificationPreference(c, 11)
	require.NoError(t, err)
	if pref.DigestHour != 0 {
		t.Errorf("expected the saved digest hour 0, got %d", pref.DigestHour)
	}

	settings, err := repo.GetAllUserDigestSettings(c)
	require.NoError(t, err)
	for _, s := range settings {
		if s.ID == 11 && s.DigestHour != 0 {
			t.Errorf("expected digest settings to report hour 0, got %d", s.DigestHour)
		}
	}
}
