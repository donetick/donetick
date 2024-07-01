package notifier

import (
	"context"
	"log"
	"time"

	"donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	nRepo "donetick.com/core/internal/notifier/repo"
	notifier "donetick.com/core/internal/notifier/telegram"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type keyType string

const (
	SchedulerKey keyType = "scheduler"
)

type Scheduler struct {
	choreRepo        *chRepo.ChoreRepository
	userRepo         *uRepo.UserRepository
	stopChan         chan bool
	notifier         *notifier.TelegramNotifier
	notificationRepo *nRepo.NotificationRepository
	SchedulerJobs    config.SchedulerConfig
}

func NewScheduler(cfg *config.Config, ur *uRepo.UserRepository, cr *chRepo.ChoreRepository, n *notifier.TelegramNotifier, nr *nRepo.NotificationRepository) *Scheduler {
	return &Scheduler{
		choreRepo:        cr,
		userRepo:         ur,
		stopChan:         make(chan bool),
		notifier:         n,
		notificationRepo: nr,
		SchedulerJobs:    cfg.SchedulerJobs,
	}
}

func (s *Scheduler) Start(c context.Context) {
	log := logging.FromContext(c)
	log.Debug("Scheduler started")
	go s.runScheduler(c, " NOTIFICATION_SCHEDULER ", s.loadAndSendNotificationJob, 3*time.Minute)
}

func (s *Scheduler) loadAndSendNotificationJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now()
	getAllPendingNotifications, err := s.notificationRepo.GetPendingNotificaiton(c, time.Minute*15)
	log.Debug("Getting pending notifications", " count ", len(getAllPendingNotifications))

	if err != nil {
		log.Error("Error getting pending notifications")
		return time.Since(startTime), err
	}

	for _, notification := range getAllPendingNotifications {
		s.notifier.SendNotification(c, notification)
		notification.IsSent = true
	}

	s.notificationRepo.MarkNotificationsAsSent(getAllPendingNotifications)
	return time.Since(startTime), nil
}
func (s *Scheduler) runScheduler(c context.Context, jobName string, job func(c context.Context) (time.Duration, error), interval time.Duration) {

	for {
		logging.FromContext(c).Debug("Scheduler running ", jobName, " time", time.Now().String())

		select {
		case <-s.stopChan:
			log.Println("Scheduler stopped")
			return
		default:
			elapsedTime, err := job(c)
			if err != nil {
				logging.FromContext(c).Error("Error running scheduler job", err)
			}
			logging.FromContext(c).Debug("Scheduler job completed", jobName, " time", elapsedTime.String())
		}
		time.Sleep(interval)
	}
}

func (s *Scheduler) Stop() {
	s.stopChan <- true
}
