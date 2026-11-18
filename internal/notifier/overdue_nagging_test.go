package notifier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	nService "donetick.com/core/internal/notifier/service"
	pModel "donetick.com/core/internal/project/model"
	uModel "donetick.com/core/internal/user/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOverdueNaggingDefaultAndExplicitClear(t *testing.T) {
	var metadata chModel.NotificationMetadata
	require.NoError(t, json.Unmarshal([]byte(`{"templates":[{"value":0,"unit":"m"}],"overdueNagging":{}}`), &metadata))
	require.Equal(t, 60, metadata.OverdueNagging.IntervalMinutes)
	require.Equal(t, 3, metadata.OverdueNagging.MaxFollowUps)
	require.Len(t, metadata.Templates, 1)
	require.True(t, metadata.HasOverdueNaggingUpdate())

	require.NoError(t, json.Unmarshal([]byte(`{"overdueNagging":null}`), &metadata))
	require.Nil(t, metadata.OverdueNagging)
	require.True(t, metadata.HasOverdueNaggingUpdate())

	require.NoError(t, json.Unmarshal([]byte(`{"nagging":true}`), &metadata))
	require.False(t, metadata.HasOverdueNaggingUpdate())
}

func TestOverdueNagSlot(t *testing.T) {
	due := time.Date(2026, 11, 17, 17, 0, 0, 0, time.UTC)
	chore := chModel.Chore{
		IsActive: true, NextDueDate: &due,
		NotificationMetadataV2: &chModel.NotificationMetadata{
			OverdueNagging: &chModel.OverdueNagging{IntervalMinutes: 60, MaxFollowUps: 3},
		},
	}
	for _, elapsed := range []time.Duration{-time.Second, 0, time.Hour - time.Second} {
		slot, _ := overdueNagSlot(&chore, due.Add(elapsed))
		require.True(t, slot.IsZero())
	}
	slot, _ := overdueNagSlot(&chore, due.Add(time.Hour))
	require.Equal(t, due.Add(time.Hour), slot)

	chore.NotificationMetadataV2.OverdueNagging.IntervalMinutes = 15
	slot, interval := overdueNagSlot(&chore, due.Add(47*time.Minute))
	require.Equal(t, 15*time.Minute, interval)
	require.Equal(t, due.Add(45*time.Minute), slot)

	slot, _ = overdueNagSlot(&chore, due.Add(time.Hour))
	require.True(t, slot.IsZero())
	chore.NotificationMetadataV2.OverdueNagging.MaxFollowUps = 5
	slot, _ = overdueNagSlot(&chore, due.Add(time.Hour))
	require.Equal(t, due.Add(time.Hour), slot)

	chore.NotificationMetadataV2.OverdueNagging.MaxFollowUps = 1
	chore.NotificationMetadataV2.OverdueNagging.ContinueUntilComplete = true
	slot, _ = overdueNagSlot(&chore, due.Add(4*time.Hour))
	require.Equal(t, due.Add(4*time.Hour), slot)

	chore.Status = chModel.ChoreStatusPendingApproval
	slot, _ = overdueNagSlot(&chore, due.Add(time.Hour))
	require.True(t, slot.IsZero())
}

func TestOverdueNaggingQueuesOneSlotPerOccurrence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&chModel.Chore{}, &chModel.ChoreAssignees{}, &pModel.Project{}, &cModel.Circle{},
		&cModel.UserCircle{}, &uModel.User{}, &uModel.UserNotificationTarget{}, &nModel.Notification{}, &nModel.OverdueNagSlot{}))
	webhook := "https://example.com/notifications"
	circle := cModel.Circle{WebhookURL: &webhook}
	require.NoError(t, db.Create(&circle).Error)

	ctx := context.Background()
	due := time.Now().UTC().Truncate(time.Minute).Add(-2 * time.Hour)
	chore := chModel.Chore{
		Name: "Take out the trash", CircleID: circle.ID, IsActive: true, NextDueDate: &due,
		NotificationMetadataV2: &chModel.NotificationMetadata{
			OverdueNagging: &chModel.OverdueNagging{IntervalMinutes: 15, MaxFollowUps: 3},
		},
	}
	require.NoError(t, db.Create(&chore).Error)
	repo := nRepo.NewNotificationRepository(db)
	scheduler := Scheduler{
		notificationRepo: repo, notifier: &Notifier{},
		planner: nService.NewNotificationPlanner(repo, cRepo.NewCircleRepository(db)),
	}

	now := due.Add(47 * time.Minute)
	require.NoError(t, scheduler.queueOverdueNags(ctx, now))
	require.NoError(t, scheduler.queueOverdueNags(ctx, now))
	pending, err := repo.GetPendingNags(ctx, now)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.True(t, due.Add(45*time.Minute).Equal(pending[0].ScheduledFor))
	require.False(t, chore.Notification)

	// Changing the interval cannot create another follow-up beyond the limit
	// for this occurrence, even when the new cadence has only reached slot one.
	chore.NotificationMetadataV2.OverdueNagging.IntervalMinutes = 60
	chore.NotificationMetadataV2.OverdueNagging.MaxFollowUps = 1
	require.NoError(t, db.Model(&chore).Update("notification_meta_v2", chore.NotificationMetadataV2).Error)
	require.NoError(t, scheduler.queueOverdueNags(ctx, due.Add(time.Hour)))
	blocked, err := repo.GetPendingNags(ctx, due.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, blocked)

	// Advancing the task makes its previously queued nag obsolete.
	nextDue := due.Add(24 * time.Hour)
	require.NoError(t, db.Model(&chore).Update("next_due_date", nextDue).Error)
	require.NoError(t, scheduler.dispatchOverdueNag(ctx, pending[0], now))

	now = nextDue.Add(time.Hour)
	require.NoError(t, scheduler.queueOverdueNags(ctx, now))
	pending, err = repo.GetPendingNags(ctx, now)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.True(t, now.Equal(pending[0].ScheduledFor))
	claimed, err := repo.ClaimNag(ctx, pending[0].ID, now)
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = repo.ClaimNag(ctx, pending[0].ID, now)
	require.NoError(t, err)
	require.False(t, claimed)
}
