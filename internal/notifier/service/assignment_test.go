package service

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cRepo "donetick.com/core/internal/circle/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	uModel "donetick.com/core/internal/user/model"
	"github.com/stretchr/testify/require"
)

func TestAssignmentNotificationLifecycle(t *testing.T) {
	_, db, ur, dr := newDigestTestPlanner(t)
	ctx := context.Background()
	user := seedUserWithChores(t, db, ur, dr, time.Now().UTC())
	// Mobile-only account: no legacy user_notification_targets row.
	planner := NewNotificationPlanner(nRepo.NewNotificationRepository(db), cRepo.NewCircleRepository(db), ur, dr)
	chore := &chModel.Chore{Name: "No due date", CircleID: user.CircleID, AssignedTo: &user.ID, CreatedBy: user.ID, IsActive: true}
	require.NoError(t, db.Create(chore).Error)
	// Assignment is independent of the per-task due-date reminder switch/templates.
	require.True(t, planner.GenerateChoreNotifications(ctx, chore, nil, 0))
	var rows []nModel.Notification
	require.NoError(t, db.Where("chore_id = ?", chore.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, user.ID, rows[0].UserID)
	require.Equal(t, nModel.NotificationPlatformFCM, rows[0].TypeID)
	require.Equal(t, "fcm-token-1", rows[0].TargetID)
	require.Equal(t, "on_assignment", rows[0].RawEvent["type"])
	payload := rows[0].RawEvent["payload"].(map[string]interface{})
	require.Equal(t, "on_assignment", payload["data"].(map[string]interface{})["type"])
	// Unrelated edits must neither duplicate nor delete a pending assignment event.
	require.True(t, planner.GenerateChoreNotifications(ctx, chore, &user.ID, 0))
	rows = nil
	require.NoError(t, db.Where("chore_id = ?", chore.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	// Removing the assignment cancels the pending event to the previous assignee.
	require.NoError(t, db.Model(chore).Update("assigned_to", nil).Error)
	require.NoError(t, db.First(chore, chore.ID).Error)
	require.True(t, planner.GenerateChoreNotifications(ctx, chore, &user.ID, 0))
	rows = nil
	require.NoError(t, db.Where("chore_id = ?", chore.ID).Find(&rows).Error)
	require.Empty(t, rows)
}

func TestAssignmentNotificationPreferenceAndVisibility(t *testing.T) {
	for _, scenario := range []string{"opted_out", "quiet", "inactive_task", "restricted_project", "unloaded_project", "inactive_member", "no_delivery_target", "unchanged", "new_assignee", "mobile_only", "multiple_devices", "legacy_only", "third_party_and_push", "inactive_device", "duplicate_legacy_token", "self_assigned_on_create", "self_assigned_on_edit", "assigned_by_other"} {
		t.Run(scenario, func(t *testing.T) {
			_, db, ur, dr := newDigestTestPlanner(t)
			ctx := context.Background()
			user := seedUserWithChores(t, db, ur, dr, time.Now().UTC())
			require.NoError(t, db.Create(&uModel.UserNotificationTarget{UserID: user.ID, Type: nModel.NotificationPlatformFCM, TargetID: "target"}).Error)
			planner := NewNotificationPlanner(nRepo.NewNotificationRepository(db), cRepo.NewCircleRepository(db), ur, dr)
			chore := &chModel.Chore{ID: 100, Name: "Assigned task", CircleID: user.CircleID, AssignedTo: &user.ID, IsActive: true}
			oldID := user.ID + 1
			previous := &oldID
			actor := 0
			switch scenario {
			case "opted_out":
				require.NoError(t, ur.UpsertNotificationPreference(ctx, &uModel.UserNotificationPreference{UserID: user.ID, Profile: uModel.NotificationProfileBalanced, Overrides: uModel.NotificationOverrides{uModel.NotificationEventOnAssignment: false}}))
			case "quiet":
				require.NoError(t, ur.UpsertNotificationPreference(ctx, &uModel.UserNotificationPreference{UserID: user.ID, Profile: uModel.NotificationProfileQuiet}))
			case "inactive_task":
				chore.IsActive = false
			case "restricted_project":
				chore.Project = sharedProject(user.ID + 1)
				chore.ProjectID = &chore.Project.ID
			case "unloaded_project":
				id := 40
				chore.ProjectID = &id
			case "inactive_member":
				require.NoError(t, db.Table("user_circles").Where("user_id = ?", user.ID).Update("is_active", false).Error)
			case "no_delivery_target":
				require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&uModel.UserNotificationTarget{}).Error)
				require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&uModel.UserDeviceToken{}).Error)
			case "mobile_only", "multiple_devices", "inactive_device":
				require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&uModel.UserNotificationTarget{}).Error)
				if scenario == "multiple_devices" {
					require.NoError(t, dr.RegisterDeviceToken(ctx, &uModel.UserDeviceToken{UserID: user.ID, DeviceID: "device-2", Token: "fcm-token-2", Platform: "android"}))
				}
				if scenario == "inactive_device" {
					require.NoError(t, db.Model(&uModel.UserDeviceToken{}).Where("user_id = ?", user.ID).Update("is_active", false).Error)
				}
			case "legacy_only":
				require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&uModel.UserDeviceToken{}).Error)
			case "third_party_and_push":
				require.NoError(t, db.Model(&uModel.UserNotificationTarget{}).Where("user_id = ?", user.ID).Update("type", nModel.NotificationPlatformTelegram).Error)
			case "duplicate_legacy_token":
				require.NoError(t, db.Model(&uModel.UserNotificationTarget{}).Where("user_id = ?", user.ID).Update("target_id", "fcm-token-1").Error)
			case "unchanged":
				previous = &user.ID
			case "self_assigned_on_create":
				// Creating a task and taking it yourself: no previous assignee, but
				// the actor is the assignee, so there is nothing to announce.
				previous = nil
				actor = user.ID
			case "self_assigned_on_edit":
				actor = user.ID
			case "assigned_by_other":
				actor = user.ID + 1
			}
			require.True(t, planner.generateAssignmentNotification(ctx, chore, previous, actor))
			var count int64
			require.NoError(t, db.Model(&nModel.Notification{}).Count(&count).Error)
			expected := map[string]int64{
				"new_assignee": 1, "mobile_only": 1, "multiple_devices": 2,
				"legacy_only": 1, "third_party_and_push": 2, "duplicate_legacy_token": 1,
				"assigned_by_other": 1,
			}
			require.Equal(t, expected[scenario], count)
		})
	}
}

func TestAssignmentNotificationProfileDefaultsAndSendTimeGating(t *testing.T) {
	for _, profile := range []uModel.NotificationProfile{uModel.NotificationProfileBalanced, uModel.NotificationProfilePlanner, uModel.NotificationProfileJustInTime, uModel.NotificationProfilePersistent, uModel.NotificationProfileQuiet, uModel.NotificationProfileCustom} {
		pref := &uModel.UserNotificationPreference{Profile: profile}
		expected := profile != uModel.NotificationProfileQuiet && profile != uModel.NotificationProfileCustom
		require.Equal(t, expected, AllowedByPreference(pref, "on_assignment"))
		pref.Overrides = uModel.NotificationOverrides{uModel.NotificationEventOnAssignment: !expected}
		require.Equal(t, !expected, AllowedByPreference(pref, "on_assignment"))
	}
}
