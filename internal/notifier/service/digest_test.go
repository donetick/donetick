package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/database"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
)

func newDigestTestPlanner(t *testing.T) (*DigestPlanner, *gorm.DB, *uRepo.UserRepository, *dRepo.DeviceRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "digest.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"

	cr := chRepo.NewChoreRepository(db, cfg)
	cir := cRepo.NewCircleRepository(db)
	ur := uRepo.NewUserRepository(db, cfg)
	dr := dRepo.NewDeviceRepository(db, cfg)
	nr := nRepo.NewNotificationRepository(db)

	return NewDigestPlanner(cr, cir, ur, dr, nr), db, ur, dr
}

// seedUserWithChores creates a circle, a user in it with an active FCM device
// token (digests are push-only, so that's the only delivery channel they can
// use), one overdue chore and one chore due in 3 days, both assigned to the user.
func seedUserWithChores(t *testing.T, db *gorm.DB, ur *uRepo.UserRepository, dr *dRepo.DeviceRepository, now time.Time) *uModel.User {
	t.Helper()
	circle := &cModel.Circle{Name: "Test Circle"}
	require.NoError(t, db.Create(circle).Error)

	user := &uModel.User{Username: "alice", DisplayName: "Alice", CircleID: circle.ID, Timezone: "UTC"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: user.ID, CircleID: circle.ID, Role: cModel.UserRoleMember, IsActive: true}).Error)
	require.NoError(t, dr.RegisterDeviceToken(context.Background(), &uModel.UserDeviceToken{
		UserID: user.ID, DeviceID: "device-1", Token: "fcm-token-1", Platform: "ios",
	}))

	overdue := now.Add(-24 * time.Hour)
	dueSoon := now.Add(72 * time.Hour)
	require.NoError(t, db.Create(&chModel.Chore{
		Name: "Overdue chore", CircleID: circle.ID, AssignedTo: &user.ID,
		IsActive: true, NextDueDate: &overdue,
	}).Error)
	require.NoError(t, db.Create(&chModel.Chore{
		Name: "Upcoming chore", CircleID: circle.ID, AssignedTo: &user.ID,
		IsActive: true, NextDueDate: &dueSoon,
	}).Error)

	return user
}

func TestGenerateDueDigestsSendsDailyDigestAtConfiguredHour(t *testing.T) {
	planner, db, ur, dr := newDigestTestPlanner(t)
	now := time.Now().UTC()
	user := seedUserWithChores(t, db, ur, dr, now)

	// Custom profile, weekly off, so the test doesn't depend on today being Monday.
	require.NoError(t, ur.UpsertNotificationPreference(context.Background(), &uModel.UserNotificationPreference{
		UserID:  user.ID,
		Profile: uModel.NotificationProfileCustom,
		Overrides: uModel.NotificationOverrides{
			uModel.NotificationEventDailyDigest:  true,
			uModel.NotificationEventWeeklyDigest: false,
		},
		DigestHour: now.Hour(),
	}))

	sent, err := planner.GenerateDueDigests(context.Background(), now)
	require.NoError(t, err)
	if sent != 1 {
		t.Fatalf("expected 1 digest sent, got %d", sent)
	}

	var notifications []*nModel.Notification
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&notifications).Error)
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification row, got %d", len(notifications))
	}
	got := notifications[0]
	if got.ChoreID != DigestChoreIDDaily {
		t.Errorf("expected sentinel chore ID %d, got %d", DigestChoreIDDaily, got.ChoreID)
	}
	if got.TargetID != "fcm-token-1" || got.TypeID != nModel.NotificationPlatformFCM {
		t.Errorf("expected delivery to fcm-token-1/FCM, got %q/%d", got.TargetID, got.TypeID)
	}
	if got.RawEvent["overdue_count"] != float64(1) && got.RawEvent["overdue_count"] != 1 {
		// RawEvent round-trips through JSON in the db, so numeric types can come back as float64.
		t.Errorf("expected overdue_count 1, got %v (%T)", got.RawEvent["overdue_count"], got.RawEvent["overdue_count"])
	}
	payload, ok := got.RawEvent["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected RawEvent[\"payload\"] to be an object, got %v (%T)", got.RawEvent["payload"], got.RawEvent["payload"])
	}
	if payload["title"] == "" || payload["body"] == "" {
		t.Errorf("expected a non-empty title and body in the payload, got %+v", payload)
	}

	// Running again right away must not duplicate today's digest.
	sentAgain, err := planner.GenerateDueDigests(context.Background(), now)
	require.NoError(t, err)
	if sentAgain != 0 {
		t.Errorf("expected no duplicate digest on a second run, got %d sent", sentAgain)
	}
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&notifications).Error)
	if len(notifications) != 1 {
		t.Fatalf("expected still only 1 notification row after re-running, got %d", len(notifications))
	}
}

func TestGenerateDueDigestsSkipsUsersOutsideTheirDigestHour(t *testing.T) {
	planner, db, ur, dr := newDigestTestPlanner(t)
	now := time.Now().UTC()
	user := seedUserWithChores(t, db, ur, dr, now)

	wrongHour := (now.Hour() + 2) % 24
	require.NoError(t, ur.UpsertNotificationPreference(context.Background(), &uModel.UserNotificationPreference{
		UserID:     user.ID,
		Profile:    uModel.NotificationProfileBalanced,
		DigestHour: wrongHour,
	}))

	sent, err := planner.GenerateDueDigests(context.Background(), now)
	require.NoError(t, err)
	if sent != 0 {
		t.Errorf("expected no digest outside the configured hour, got %d", sent)
	}
}

func TestGenerateDueDigestsSkipsUsersWithNoDeliveryTarget(t *testing.T) {
	planner, db, ur, _ := newDigestTestPlanner(t)
	now := time.Now().UTC()

	circle := &cModel.Circle{Name: "Test Circle"}
	require.NoError(t, db.Create(circle).Error)
	user := &uModel.User{Username: "bob", DisplayName: "Bob", CircleID: circle.ID, Timezone: "UTC"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: user.ID, CircleID: circle.ID, Role: cModel.UserRoleMember, IsActive: true}).Error)
	// No device token registered: this user has nowhere to deliver a push digest.

	require.NoError(t, ur.UpsertNotificationPreference(context.Background(), &uModel.UserNotificationPreference{
		UserID:     user.ID,
		Profile:    uModel.NotificationProfileBalanced,
		DigestHour: now.Hour(),
	}))

	sent, err := planner.GenerateDueDigests(context.Background(), now)
	require.NoError(t, err)
	if sent != 0 {
		t.Errorf("expected no digest for a user without a delivery target, got %d", sent)
	}
}

func TestFormatDailyDigestPayloadLeadsTitleWithCount(t *testing.T) {
	overdue := []*chModel.Chore{{Name: "Trash"}, {Name: "Dishes"}}
	upcoming := []*chModel.Chore{{Name: "Vacuum"}}

	title, body := formatDailyDigestPayload(overdue, upcoming)

	if title != "🗓️ 3 tasks today" {
		t.Errorf("unexpected title: %q", title)
	}
	if body != "Overdue: Trash, Dishes · Due today: Vacuum" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFormatDailyDigestPayloadSingularCount(t *testing.T) {
	title, _ := formatDailyDigestPayload(nil, []*chModel.Chore{{Name: "Vacuum"}})
	if title != "🗓️ 1 task today" {
		t.Errorf("expected singular 'task', got %q", title)
	}
}

func TestFormatDailyDigestPayloadTruncatesLongLists(t *testing.T) {
	upcoming := []*chModel.Chore{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
	_, body := formatDailyDigestPayload(nil, upcoming)
	if body != "Due today: A, B, C, +2 more" {
		t.Errorf("expected a truncated list with a '+N more' tail, got %q", body)
	}
}

func TestFormatWeeklyDigestPayloadHooksOnSoonestTask(t *testing.T) {
	monday := time.Date(2026, time.October, 26, 9, 0, 0, 0, time.UTC) // a Monday
	upcoming := []*chModel.Chore{
		{Name: "Groceries", NextDueDate: &monday},
		{Name: "Laundry", NextDueDate: ptrTime(monday.AddDate(0, 0, 2))},
	}

	title, body := formatWeeklyDigestPayload(nil, upcoming, time.UTC)

	if title != "📅 Your week ahead" {
		t.Errorf("unexpected title: %q", title)
	}
	if body != "2 tasks this week, starting with Groceries (Mon)." {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFormatWeeklyDigestPayloadMentionsOverdueAlongsideUpcoming(t *testing.T) {
	due := time.Date(2026, time.October, 28, 9, 0, 0, 0, time.UTC)
	overdue := []*chModel.Chore{{Name: "Old task"}}
	upcoming := []*chModel.Chore{{Name: "Groceries", NextDueDate: &due}}

	_, body := formatWeeklyDigestPayload(overdue, upcoming, time.UTC)
	if body != "1 task overdue. 1 task this week: Groceries (Wed)." {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFormatWeeklyDigestPayloadNoUpcomingTasks(t *testing.T) {
	overdue := []*chModel.Chore{{Name: "Old task"}}
	_, body := formatWeeklyDigestPayload(overdue, nil, time.UTC)
	if body != "1 task overdue. Nothing new scheduled this week." {
		t.Errorf("unexpected body: %q", body)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
