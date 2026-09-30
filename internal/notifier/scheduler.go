package notifier

import (
	"context"
	"log"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/bridge"
	chRepo "donetick.com/core/internal/chore/repo"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/events"
	nRepo "donetick.com/core/internal/notifier/repo"
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
	deviceRepo       *dRepo.DeviceRepository
	bridgeSvc        *bridge.Service
	stopChan         chan bool
	notifier         *Notifier
	eventsProducer   *events.EventsProducer
	notificationRepo *nRepo.NotificationRepository
	SchedulerJobs    config.SchedulerConfig
}

func NewScheduler(cfg *config.Config, ur *uRepo.UserRepository, cr *chRepo.ChoreRepository, n *Notifier, nr *nRepo.NotificationRepository, ep *events.EventsProducer, dr *dRepo.DeviceRepository, bs *bridge.Service) *Scheduler {
	return &Scheduler{
		choreRepo:        cr,
		userRepo:         ur,
		deviceRepo:       dr,
		bridgeSvc:        bs,
		stopChan:         make(chan bool),
		notifier:         n,
		notificationRepo: nr,
		eventsProducer:   ep,
		SchedulerJobs:    cfg.SchedulerJobs,
	}
}

func (s *Scheduler) Start(c context.Context) {
	log := logging.FromContext(c)
	log.Debug("Scheduler started")
	go s.runScheduler(c, " NOTIFICATION_SCHEDULER ", s.loadAndSendNotificationJob, 3*time.Minute)
	go s.runScheduler(c, " NOTIFICATION_CLEANUP ", s.cleanupSentNotifications, 24*time.Hour*30)
	go s.runScheduler(c, " BRIDGE_DEVICE_SYNC_RETRY ", s.retryPendingBridgeDeviceSyncJob, 10*time.Minute)
}

// retryPendingBridgeDeviceSyncJob opportunistically retries Bridge device
// registration for local devices that were saved locally but never
// successfully synced to Bridge (e.g. Bridge was temporarily unreachable
// at registration time, plan §15 "retry registration later without
// blocking normal login"). Content-free: only re-sends the device's own
// FCM token/platform/app version, never notification content, and never
// logs the token. A no-op (nothing queried) when Bridge is disabled.
func (s *Scheduler) retryPendingBridgeDeviceSyncJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now().UTC()

	client := s.bridgeSvc.Client()
	if !client.Enabled() {
		return time.Since(startTime), nil
	}

	pending, err := s.deviceRepo.GetDevicesPendingBridgeSync(c, 50)
	if err != nil {
		log.Error("Error getting devices pending bridge sync", "error", err)
		return time.Since(startTime), err
	}

	for _, d := range pending {
		res, err := client.RegisterDevice(c, bridge.RegisterDeviceInput{
			LocalDeviceID: d.DeviceID,
			Token:         d.Token,
			Platform:      d.Platform,
			AppVersion:    d.AppVersion,
			DeviceModel:   d.DeviceModel,
		})
		if err != nil {
			category := bridge.Classify(err)
			log.Debug("Bridge device sync retry still failing", "category", string(category), "device_id", d.DeviceID)
			if updErr := s.deviceRepo.UpdateBridgeSyncStatus(c, d.ID, nil, string(category)); updErr != nil {
				log.Error("Failed to record bridge sync retry status", "error", updErr)
			}
			continue
		}
		if updErr := s.deviceRepo.UpdateBridgeSyncStatus(c, d.ID, &res.BridgeDeviceID, "synced"); updErr != nil {
			log.Error("Failed to persist bridge device id from retry", "error", updErr)
		}
	}

	return time.Since(startTime), nil
}
func (s *Scheduler) cleanupSentNotifications(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	deleteBefore := time.Now().UTC().Add(-time.Hour * 24 * 30)
	err := s.notificationRepo.DeleteSentNotifications(c, deleteBefore)
	if err != nil {
		log.Error("Error deleting sent notifications", err)
		return time.Duration(0), err
	}
	return time.Duration(0), nil
}

func (s *Scheduler) loadAndSendNotificationJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now().UTC()
	getAllPendingNotifications, err := s.notificationRepo.GetPendingNotification(c, time.Minute*900)
	log.Debug("Getting pending notifications", " count ", len(getAllPendingNotifications))

	if err != nil {
		log.Error("Error getting pending notifications")
		return time.Since(startTime), err
	}

	for _, notification := range getAllPendingNotifications {
		err := s.notifier.SendNotification(c, notification)
		if err != nil {
			log.Error("Error sending notification", err)
			continue
		}
		if notification.RawEvent != nil && notification.WebhookURL != nil {
			// if we have a webhook url, we should send the event to the webhook
			s.eventsProducer.NotificationEvent(c, *notification.WebhookURL, notification.RawEvent)
		}

		notification.IsSent = true
	}

	s.notificationRepo.MarkNotificationsAsSent(getAllPendingNotifications)
	return time.Since(startTime), nil
}
func (s *Scheduler) runScheduler(c context.Context, jobName string, job func(c context.Context) (time.Duration, error), interval time.Duration) {

	for {
		logging.FromContext(c).Debug("Scheduler running ", jobName, " time", time.Now().UTC().String())

		select {
		case <-s.stopChan:
			log.Println("Scheduler stopped")
			return
		default:
			elapsedTime, err := job(c)
			if err != nil {
				logging.FromContext(c).Error("Error running scheduler job", err)
			}
			logging.FromContext(c).Debug("Scheduler job completed", jobName, " time: ", elapsedTime.String())
		}
		time.Sleep(interval)
	}
}

func (s *Scheduler) Stop() {
	s.stopChan <- true
}
