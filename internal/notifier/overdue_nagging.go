package notifier

import (
	"context"
	"errors"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	nService "donetick.com/core/internal/notifier/service"
	"gorm.io/gorm"
)

// overdueNagSlot returns the latest elapsed interval. The first slot is one
// full interval after the due date. Missed slots count toward the limit.
func overdueNagSlot(chore *chModel.Chore, now time.Time) (time.Time, time.Duration) {
	if !chore.IsActive || chore.NextDueDate == nil || chore.Status == chModel.ChoreStatusPendingApproval ||
		chore.FrequencyType == chModel.FrequencyTypeTrigger || chore.FrequencyType == chModel.FrequencyTypeAlways ||
		chore.NotificationMetadataV2 == nil || !chore.NotificationMetadataV2.OverdueNagging.Valid() {
		return time.Time{}, 0
	}
	followUps := chore.NotificationMetadataV2.OverdueNagging
	interval := time.Duration(followUps.IntervalMinutes) * time.Minute
	slot := now.Sub(*chore.NextDueDate) / interval
	if slot < 1 || (!followUps.ContinueUntilComplete && slot > time.Duration(followUps.MaxFollowUps)) {
		return time.Time{}, 0
	}
	return chore.NextDueDate.UTC().Add(slot * interval), interval
}

func (s *Scheduler) overdueNaggingJob(ctx context.Context) (time.Duration, error) {
	now := time.Now().UTC()
	err := errors.Join(s.queueOverdueNags(ctx, now), s.sendOverdueNags(ctx), s.notificationRepo.DeleteExpiredNags(ctx, now))
	return time.Since(now), err
}

func (s *Scheduler) queueOverdueNags(ctx context.Context, now time.Time) error {
	chores, err := s.notificationRepo.GetOverdueNaggingChores(ctx, now)
	if err != nil {
		return err
	}
	var jobErr error
	for _, candidate := range chores {
		slot, _ := overdueNagSlot(&candidate.Chore, now)
		if slot.IsZero() {
			continue
		}
		if candidate.LastNagDueDate != nil && candidate.LastNagDueDate.Equal(*candidate.NextDueDate) {
			maximumReached := !candidate.NotificationMetadataV2.OverdueNagging.ContinueUntilComplete &&
				candidate.NagQueuedCount >= candidate.NotificationMetadataV2.OverdueNagging.MaxFollowUps
			slotAlreadyQueued := candidate.LastNagSlot != nil && !slot.After(*candidate.LastNagSlot)
			if maximumReached || slotAlreadyQueued {
				continue
			}
		}
		if err := s.queueChoreNag(ctx, candidate.ID, now); err != nil {
			jobErr = errors.Join(jobErr, err)
		}
	}
	return jobErr
}

func (s *Scheduler) queueChoreNag(ctx context.Context, choreID int, now time.Time) error {
	chore, err := s.notificationRepo.GetChoreForNagging(ctx, choreID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	slot, interval := overdueNagSlot(chore, now)
	if slot.IsZero() {
		return nil
	}
	notifications, err := s.planner.OverdueNaggingDestinations(ctx, chore, s.deviceRepo, s.notifier.FCM != nil)
	if err != nil {
		return err
	}
	expiresAt := slot.Add(interval)
	for i := range notifications {
		notification := &notifications[i]
		notification.ScheduledFor = slot
		notification.NagExpiresAt = &expiresAt
		notification.NagDueDate = chore.NextDueDate
		nService.PopulateOverdueNaggingDelivery(notification, chore)
	}
	return s.notificationRepo.QueueOverdueNagging(ctx, chore, slot, notifications)
}

func (s *Scheduler) sendOverdueNags(ctx context.Context) error {
	notifications, err := s.notificationRepo.GetPendingNags(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	var jobErr error
	for _, notification := range notifications {
		if err := s.sendOverdueNag(ctx, notification); err != nil {
			jobErr = errors.Join(jobErr, err)
		}
	}
	return jobErr
}

func (s *Scheduler) sendOverdueNag(ctx context.Context, notification *nModel.NotificationDetails) error {
	now := time.Now().UTC()
	claimed, err := s.notificationRepo.ClaimNag(ctx, notification.ID, now)
	if err != nil || !claimed {
		return err
	}
	if err := s.dispatchOverdueNag(ctx, notification, now); err != nil {
		return errors.Join(err, s.notificationRepo.ReleaseNagClaim(ctx, notification.ID))
	}
	return s.notificationRepo.MarkNotificationsAsSent([]*nModel.NotificationDetails{notification})
}

func (s *Scheduler) dispatchOverdueNag(ctx context.Context, notification *nModel.NotificationDetails, now time.Time) error {
	chore, err := s.notificationRepo.GetChoreForNagging(ctx, notification.ChoreID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	slot, interval := overdueNagSlot(chore, now)
	if slot.IsZero() || !slot.Equal(notification.ScheduledFor) ||
		notification.NagDueDate == nil || !notification.NagDueDate.Equal(*chore.NextDueDate) ||
		notification.NagExpiresAt == nil || !notification.NagExpiresAt.Equal(slot.Add(interval)) {
		return nil
	}
	targets, err := s.planner.OverdueNaggingDestinations(ctx, chore, s.deviceRepo, s.notifier.FCM != nil)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.TypeID != notification.TypeID || target.TargetID != notification.TargetID {
			continue
		}
		nService.PopulateOverdueNaggingDelivery(&notification.Notification, chore)
		if notification.TypeID == nModel.NotificationPlatformWebhook {
			s.eventsProducer.OverdueNaggingEvent(notification.TargetID, notification.RawEvent)
			return nil
		}
		return s.notifier.SendNotification(ctx, notification)
	}
	return nil
}
