package dispatch

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/internal/notifier/push"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
)

const recipientID = 7

func newTestDispatcher(t *testing.T) (*Dispatcher, *gorm.DB, *uRepo.UserRepository, *dRepo.DeviceRepository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	ur := uRepo.NewUserRepository(db, cfg)
	dr := dRepo.NewDeviceRepository(db, cfg)

	// A nil notifier means no push provider is configured, so the push itself is
	// a no-op; these tests assert which rows get queued and for whom.
	return NewDispatcher(nRepo.NewNotificationRepository(db), ur, dr, push.NewSender(dr, nil)), db, ur, dr
}

func audience(platform nModel.NotificationPlatform, targetID string) []*cModel.UserCircleDetail {
	return []*cModel.UserCircleDetail{{
		UserCircle:       cModel.UserCircle{UserID: recipientID, CircleID: 1, Role: cModel.UserRoleAdmin, IsActive: true},
		NotificationType: platform, TargetID: targetID,
	}}
}

func testRequest() Request {
	return Request{
		EventType: nps.EventTypeOnPendingApproval,
		CircleID:  1,
		ChoreID:   3,
		Subject:   "Take out trash",
		PushTitle: "Approval Needed",
		PushBody:  "Sam is waiting for your approval",
		PushData:  map[string]string{"type": "chore_pending_approval"},
		Text:      "Sam needs your approval",
	}
}

func queuedRows(t *testing.T, db *gorm.DB) []nModel.Notification {
	t.Helper()
	var rows []nModel.Notification
	require.NoError(t, db.Find(&rows).Error)
	return rows
}

func registerDevice(t *testing.T, dr *dRepo.DeviceRepository, token string) {
	t.Helper()
	require.NoError(t, dr.RegisterDeviceToken(context.Background(), &uModel.UserDeviceToken{
		UserID: recipientID, DeviceID: "device-" + token, Token: token, Platform: "ios"}))
}

// A third-party channel is additive: it is the only way to reach that chat, so
// it gets a row whether or not the recipient also has devices.
func TestNotifyUsersQueuesThirdPartyChannels(t *testing.T) {
	d, db, _, dr := newTestDispatcher(t)
	registerDevice(t, dr, "fcm-token-1")

	result := d.NotifyUsers(context.Background(), testRequest(), audience(nModel.NotificationPlatformTelegram, "chat-1"))

	require.Equal(t, 1, result.Recipients)
	rows := queuedRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, recipientID, rows[0].UserID)
	require.Equal(t, nModel.NotificationPlatformTelegram, rows[0].TypeID)
	require.Equal(t, "chat-1", rows[0].TargetID)
	require.Equal(t, "Sam needs your approval", rows[0].Text)
	require.Equal(t, 3, rows[0].ChoreID)
	// The scheduler re-checks the preference by this type after a JSON round-trip.
	require.Equal(t, string(nps.EventTypeOnPendingApproval), rows[0].RawEvent["type"])
}

// A legacy FCM target is the same transport as the push that already went out,
// so it must not produce a second copy.
func TestNotifyUsersDoesNotDoublePushFCM(t *testing.T) {
	d, db, _, dr := newTestDispatcher(t)
	registerDevice(t, dr, "fcm-token-1")

	d.NotifyUsers(context.Background(), testRequest(), audience(nModel.NotificationPlatformFCM, "fcm-token-1"))

	require.Empty(t, queuedRows(t, db), "the device push already reached this recipient")
}

// Without devices the legacy FCM target is the only route left, so it stays.
func TestNotifyUsersFallsBackToLegacyFCMWithoutDevices(t *testing.T) {
	d, db, _, _ := newTestDispatcher(t)

	d.NotifyUsers(context.Background(), testRequest(), audience(nModel.NotificationPlatformFCM, "fcm-token-1"))

	rows := queuedRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, nModel.NotificationPlatformFCM, rows[0].TypeID)
}

// A mobile-only recipient has no legacy target at all; the push is the delivery,
// and queueing a row with an empty target would only create an undeliverable one.
func TestNotifyUsersReachesMobileOnlyRecipientsWithoutQueueingRows(t *testing.T) {
	d, db, _, dr := newTestDispatcher(t)
	registerDevice(t, dr, "fcm-token-1")

	result := d.NotifyUsers(context.Background(), testRequest(), audience(nModel.NotificationPlatformNone, ""))

	require.Equal(t, 1, result.Recipients)
	require.Empty(t, queuedRows(t, db))
}

// Muting the event type silences every channel, not just the push.
func TestNotifyUsersRespectsNotificationPreference(t *testing.T) {
	d, db, ur, dr := newTestDispatcher(t)
	ctx := context.Background()
	registerDevice(t, dr, "fcm-token-1")
	require.NoError(t, ur.UpsertNotificationPreference(ctx, &uModel.UserNotificationPreference{
		UserID: recipientID, Profile: uModel.NotificationProfileQuiet}))

	result := d.NotifyUsers(ctx, testRequest(), audience(nModel.NotificationPlatformTelegram, "chat-1"))

	require.Zero(t, result.Recipients)
	require.Empty(t, queuedRows(t, db))
}

// An event type nobody has a preference case for is still delivered: a new kind
// of notification must not be silently dropped.
func TestNotifyUsersDeliversUnrecognizedEventTypes(t *testing.T) {
	d, db, ur, _ := newTestDispatcher(t)
	ctx := context.Background()
	require.NoError(t, ur.UpsertNotificationPreference(ctx, &uModel.UserNotificationPreference{
		UserID: recipientID, Profile: uModel.NotificationProfileQuiet}))

	req := testRequest()
	req.EventType = "something_new"
	d.NotifyUsers(ctx, req, audience(nModel.NotificationPlatformTelegram, "chat-1"))

	require.Len(t, queuedRows(t, db), 1)
}
