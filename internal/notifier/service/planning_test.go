package service

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	uModel "donetick.com/core/internal/user/model"
	"github.com/stretchr/testify/require"
)

func TestChorePlanningRejectsOutOfOrderAssignment(t *testing.T) {
	_, db, ur, dr := newDigestTestPlanner(t)
	ctx := context.Background()
	first := seedUserWithChores(t, db, ur, dr, time.Now().UTC())
	second := &uModel.User{Username: "bob", CircleID: first.CircleID}
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: second.ID, CircleID: first.CircleID, IsActive: true}).Error)
	require.NoError(t, dr.RegisterDeviceToken(ctx, &uModel.UserDeviceToken{
		UserID: second.ID, DeviceID: "bob-device", Token: "bob-token", Platform: "ios",
	}))
	planner := NewNotificationPlanner(nRepo.NewNotificationRepository(db), cRepo.NewCircleRepository(db), ur, dr)
	old := &chModel.Chore{Name: "Assigned task", CircleID: first.CircleID, AssignedTo: &first.ID, IsActive: true}
	require.NoError(t, db.Create(old).Error)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", old.ID).Updates(map[string]interface{}{
		"assigned_to": second.ID, "sync_version": old.SyncVersion + 1,
	}).Error)
	var current chModel.Chore
	require.NoError(t, db.First(&current, old.ID).Error)

	// The newer request finishes planning before the older background job runs.
	require.True(t, planner.GenerateChoreNotifications(ctx, &current, &first.ID))
	require.True(t, planner.GenerateChoreNotifications(ctx, old, nil))
	// Reminder-only callers must not delete the newer assignment either.
	require.True(t, planner.GenerateNotifications(ctx, old))
	var rows []nModel.Notification
	require.NoError(t, db.Where("chore_id = ?", old.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, second.ID, rows[0].UserID)
	require.Equal(t, "bob-token", rows[0].TargetID)
}

func TestChorePlanningSerializesMutationDuringPlanning(t *testing.T) {
	_, db, ur, dr := newDigestTestPlanner(t)
	ctx := context.Background()
	user := seedUserWithChores(t, db, ur, dr, time.Now().UTC())
	nr := nRepo.NewNotificationRepository(db)
	planner := NewNotificationPlanner(nr, cRepo.NewCircleRepository(db), ur, dr)
	old := &chModel.Chore{Name: "Assigned task", CircleID: user.CircleID, AssignedTo: &user.ID, IsActive: true}
	require.NoError(t, db.Create(old).Error)

	entered := make(chan struct{})
	release := make(chan struct{})
	oldDone := make(chan bool, 1)
	go func() {
		oldDone <- planner.planCurrentChore(ctx, old, func() bool {
			close(entered)
			<-release
			return planner.generateAssignmentNotification(ctx, old, nil)
		})
	}()
	<-entered
	// The task changes after the old job's revision check, but before insertion.
	err := db.Model(&chModel.Chore{}).Where("id = ?", old.ID).Updates(map[string]interface{}{
		"assigned_to": nil, "sync_version": old.SyncVersion + 1,
	}).Error
	var current chModel.Chore
	if err == nil {
		err = db.First(&current, old.ID).Error
	}
	newDone := make(chan bool, 1)
	go func() { newDone <- planner.GenerateChoreNotifications(ctx, &current, &user.ID) }()
	var finishedEarly bool
	select {
	case <-newDone:
		finishedEarly = true
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	require.True(t, <-oldDone)
	require.NoError(t, err)
	require.False(t, finishedEarly, "the newer job must wait for the older plan to finish")
	require.True(t, <-newDone)
	var rows []nModel.Notification
	require.NoError(t, db.Where("chore_id = ?", old.ID).Find(&rows).Error)
	require.Empty(t, rows, "the newer job must clean up the old assignment after it finishes")
}

func TestChorePlanningPreservesApprovalOnEditAndClearsOnResolution(t *testing.T) {
	_, db, ur, dr := newDigestTestPlanner(t)
	ctx := context.Background()
	user := seedUserWithChores(t, db, ur, dr, time.Now().UTC())
	nr := nRepo.NewNotificationRepository(db)
	planner := NewNotificationPlanner(nr, cRepo.NewCircleRepository(db), ur, dr)
	require.NoError(t, db.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", user.ID, user.CircleID).
		Update("role", cModel.UserRoleAdmin).Error)
	chore := &chModel.Chore{Name: "Awaiting approval", CircleID: user.CircleID, IsActive: true,
		Status: chModel.ChoreStatusPendingApproval}
	require.NoError(t, db.Create(chore).Error)
	require.NoError(t, nr.BatchInsertNotifications([]*nModel.Notification{{
		ChoreID: chore.ID, UserID: user.ID, TypeID: nModel.NotificationPlatformTelegram,
		RawEvent: nModel.JSONB{"type": "on_pending_approval"},
	}}))
	require.NoError(t, db.Model(chore).Update("name", "Edited while awaiting approval").Error)
	require.NoError(t, db.First(chore, chore.ID).Error)
	require.True(t, planner.GenerateChoreNotifications(ctx, chore, nil))
	var count int64
	require.NoError(t, db.Model(&nModel.Notification{}).Where("chore_id = ?", chore.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)

	require.NoError(t, db.Model(chore).Update("status", chModel.ChoreStatusNoStatus).Error)
	require.NoError(t, db.First(chore, chore.ID).Error)
	require.True(t, planner.GenerateChoreNotifications(ctx, chore, nil))
	require.NoError(t, db.Model(&nModel.Notification{}).Where("chore_id = ?", chore.ID).Count(&count).Error)
	require.Zero(t, count)
}
