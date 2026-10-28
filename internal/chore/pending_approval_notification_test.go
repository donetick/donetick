package chore

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/notifier/dispatch"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/internal/notifier/push"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
)

const (
	approvalSubmitterID = 1
	approvalApproverID  = 2
)

func newPendingApprovalTestHandler(t *testing.T) (*Handler, *gorm.DB, *uRepo.UserRepository, *dRepo.DeviceRepository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	ur := uRepo.NewUserRepository(db, cfg)
	dr := dRepo.NewDeviceRepository(db, cfg)
	nr := nRepo.NewNotificationRepository(db)

	// A nil notifier means no push provider is configured, so the immediate push
	// is a no-op; what the queued rows do is what these tests assert.
	return &Handler{dispatcher: dispatch.NewDispatcher(nr, ur, dr, push.NewSender(dr, nil))}, db, ur, dr
}

// approvalCircle returns a submitter (plain member) and one admin approver whose
// legacy delivery target is platform with targetID.
func approvalCircle(platform nModel.NotificationPlatform, targetID string) []*cModel.UserCircleDetail {
	return []*cModel.UserCircleDetail{
		{UserCircle: cModel.UserCircle{UserID: approvalSubmitterID, CircleID: 1, Role: cModel.UserRoleMember, IsActive: true}},
		{UserCircle: cModel.UserCircle{UserID: approvalApproverID, CircleID: 1, Role: cModel.UserRoleAdmin, IsActive: true},
			NotificationType: platform, TargetID: targetID},
	}
}

func approvalChore() *chModel.Chore {
	return &chModel.Chore{ID: 1, Name: "Take out trash", CircleID: 1, CreatedBy: approvalSubmitterID,
		AssignedTo: intPtr(approvalSubmitterID), Assignees: []chModel.ChoreAssignees{{UserID: approvalSubmitterID}}}
}

func queuedApprovalNotifications(t *testing.T, db *gorm.DB) []nModel.Notification {
	t.Helper()
	var rows []nModel.Notification
	require.NoError(t, db.Find(&rows).Error)
	return rows
}

// A third-party channel is the approver's only delivery target, so it gets a
// queued row the scheduler can send.
// The delivery rules themselves (one message per approver across devices and
// legacy channels) belong to the dispatcher and are tested there; these cover
// this path's wiring: wording, event type, and that gating actually applies.
func TestPendingApprovalNotifiesThirdPartyTarget(t *testing.T) {
	h, db, _, _ := newPendingApprovalTestHandler(t)
	submitter := &uModel.User{ID: approvalSubmitterID, DisplayName: "Sam"}

	h.notifyApproversOfPendingApproval(context.Background(), approvalChore(), submitter,
		approvalCircle(nModel.NotificationPlatformTelegram, "chat-1"))

	rows := queuedApprovalNotifications(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, approvalApproverID, rows[0].UserID)
	require.Equal(t, nModel.NotificationPlatformTelegram, rows[0].TypeID)
	require.Equal(t, "chat-1", rows[0].TargetID)
	require.Equal(t, "Sam marked \"Take out trash\" done — needs your approval", rows[0].Text)
	// The scheduler re-checks preferences by this type after the JSON round-trip,
	// so the stored event type has to be one AllowedByPreference recognizes.
	require.Equal(t, string(nps.EventTypeOnPendingApproval), rows[0].RawEvent["type"])
	require.False(t, nps.AllowedByPreference(
		&uModel.UserNotificationPreference{Profile: uModel.NotificationProfileQuiet},
		rows[0].RawEvent["type"].(string), chModel.PriorityNone))
}

// An approver who muted approval requests hears nothing on any channel.
func TestPendingApprovalRespectsNotificationPreference(t *testing.T) {
	h, db, ur, dr := newPendingApprovalTestHandler(t)
	ctx := context.Background()
	require.NoError(t, dr.RegisterDeviceToken(ctx, &uModel.UserDeviceToken{
		UserID: approvalApproverID, DeviceID: "device-1", Token: "fcm-token-1", Platform: "ios"}))
	require.NoError(t, ur.UpsertNotificationPreference(ctx, &uModel.UserNotificationPreference{
		UserID: approvalApproverID, Profile: uModel.NotificationProfileQuiet}))

	h.notifyApproversOfPendingApproval(ctx, approvalChore(), &uModel.User{ID: approvalSubmitterID, DisplayName: "Sam"},
		approvalCircle(nModel.NotificationPlatformTelegram, "chat-1"))

	require.Empty(t, queuedApprovalNotifications(t, db))
}
