package chore

import (
	"context"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/events"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/realtime"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

const (
	defaultAutoSkipInterval   = 5 * time.Minute
	maxAutoSkipPerChorePerRun = 10
)

type AutoSkipService struct {
	choreRepo       *chRepo.ChoreRepository
	circleRepo      *cRepo.CircleRepository
	userRepo        *uRepo.UserRepository
	nPlanner        *nps.NotificationPlanner
	eventProducer   *events.EventsProducer
	realTimeService *realtime.RealTimeService
	interval        time.Duration
	gracePeriod     time.Duration
	done            chan struct{}
}

func NewAutoSkipService(cfg *config.Config, cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	ur *uRepo.UserRepository, np *nps.NotificationPlanner, ep *events.EventsProducer,
	rts *realtime.RealTimeService) *AutoSkipService {
	interval := cfg.SchedulerJobs.AutoSkipJob
	if interval <= 0 {
		interval = defaultAutoSkipInterval
	}
	gracePeriod := cfg.SchedulerJobs.AutoSkipGracePeriod
	if gracePeriod < 0 {
		gracePeriod = 0
	}
	return &AutoSkipService{
		choreRepo:       cr,
		circleRepo:      circleRepo,
		userRepo:        ur,
		nPlanner:        np,
		eventProducer:   ep,
		realTimeService: rts,
		interval:        interval,
		gracePeriod:     gracePeriod,
		done:            make(chan struct{}),
	}
}

func (s *AutoSkipService) Start(ctx context.Context) {
	logging.FromContext(ctx).Infow("Auto skip service started", "interval", s.interval.String(), "gracePeriod", s.gracePeriod.String())
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.run(ctx)
		for {
			select {
			case <-s.done:
				logging.FromContext(ctx).Info("Auto skip service stopped")
				return
			case <-ticker.C:
				s.run(ctx)
			}
		}
	}()
}

func (s *AutoSkipService) Stop() {
	close(s.done)
}

func (s *AutoSkipService) run(ctx context.Context) {
	log := logging.FromContext(ctx)
	cutoff := time.Now().UTC().Add(-s.gracePeriod)

	chores, err := s.choreRepo.GetChoresDueForAutoSkip(ctx, cutoff)
	if err != nil {
		log.Errorw("Failed to load chores due for auto skip", "error", err)
		return
	}
	if len(chores) == 0 {
		return
	}
	log.Debugw("Auto skipping late chores", "count", len(chores))
	for _, chore := range chores {
		if err := s.autoSkipChore(ctx, chore, cutoff); err != nil {
			log.Errorw("Failed to auto skip chore", "choreID", chore.ID, "error", err)
		}
	}
}

func (s *AutoSkipService) autoSkipChore(ctx context.Context, chore *chModel.Chore, cutoff time.Time) error {
	log := logging.FromContext(ctx)
	performerID := chore.CreatedBy
	if chore.AssignedTo != nil && *chore.AssignedTo > 0 {
		performerID = *chore.AssignedTo
	}

	skipped := 0
	for skipped < maxAutoSkipPerChorePerRun {
		if chore.NextDueDate == nil || !chore.NextDueDate.UTC().Before(cutoff) {
			break
		}
		previousDueDate := chore.NextDueDate.UTC()
		nextDueDate, err := scheduleNextDueDate(ctx, chore, previousDueDate)
		if err != nil {
			return err
		}
		if nextDueDate == nil || !nextDueDate.After(previousDueDate) {
			log.Warnw("Auto skip could not advance the due date, leaving chore untouched", "choreID", chore.ID, "frequencyType", chore.FrequencyType)
			break
		}
		if err := s.choreRepo.SkipChore(ctx, chore, performerID, nextDueDate, chore.AssignedTo, nil); err != nil {
			return err
		}
		chore.NextDueDate = nextDueDate
		skipped++
	}
	if skipped == 0 {
		return nil
	}

	updatedChore, err := s.choreRepo.GetChore(ctx, chore.ID, performerID, chore.CircleID)
	if err != nil {
		return err
	}
	log.Infow("Auto skipped late chore", "choreID", chore.ID, "occurrences", skipped, "nextDueDate", updatedChore.NextDueDate)

	s.nPlanner.GenerateNotifications(ctx, updatedChore)

	performer, err := s.userRepo.GetUserByID(ctx, performerID)
	if err != nil {
		log.Errorw("Failed to load the user an auto skip is attributed to", "userID", performerID, "error", err)
		return nil
	}

	if circle, err := s.circleRepo.GetCircleByID(ctx, chore.CircleID); err == nil && circle.WebhookURL != nil {
		s.eventProducer.ChoreSkipped(ctx, circle.WebhookURL, updatedChore, performer)
	}

	if s.realTimeService != nil {
		history, _ := s.choreRepo.GetChoreHistoryWithLimit(ctx, chore.ID, 1)
		var choreHistory *chModel.ChoreHistory
		if len(history) > 0 {
			choreHistory = history[0]
		}
		s.realTimeService.GetEventBroadcaster().BroadcastChoreSkipped(updatedChore, performer, choreHistory, nil)
	}
	return nil
}
