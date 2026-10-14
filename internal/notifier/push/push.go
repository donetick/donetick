// Package push sends ad-hoc FCM push notifications to a user's registered
// devices. Unlike the notification planner (which schedules chore reminders
// through a user's configured notification target), this is for immediate,
// event-driven pushes to whatever devices a user has registered.
package push

import (
	"context"

	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/notifier"
	"donetick.com/core/internal/notifier/service/fcm"
	"donetick.com/core/logging"
)

type Sender struct {
	deviceRepo *dRepo.DeviceRepository
	notifier   *notifier.Notifier
}

func NewSender(dr *dRepo.DeviceRepository, n *notifier.Notifier) *Sender {
	return &Sender{
		deviceRepo: dr,
		notifier:   n,
	}
}

// SendToUser pushes a notification to every active device of a single user and
// returns the number of devices targeted. Users without registered devices are
// not an error.
func (s *Sender) SendToUser(c context.Context, userID int, payload fcm.FCMNotificationPayload) (int, error) {
	tokens, err := s.tokensForUser(c, userID)
	if err != nil {
		return 0, err
	}
	if len(tokens) == 0 {
		return 0, nil
	}
	if err := s.sendToTokens(c, tokens, payload); err != nil {
		return 0, err
	}
	return len(tokens), nil
}

// SendToUsers pushes a notification to every active device of each user. It is
// best-effort: a failure for one user is logged and the remaining users are
// still attempted. The returned count is the number of devices targeted.
func (s *Sender) SendToUsers(c context.Context, userIDs []int, payload fcm.FCMNotificationPayload) int {
	log := logging.FromContext(c)
	devices := 0
	for _, userID := range userIDs {
		sent, err := s.SendToUser(c, userID, payload)
		if err != nil {
			log.Warn("Failed to send push notification to user", "user_id", userID, "error", err)
			continue
		}
		devices += sent
	}
	return devices
}

func (s *Sender) tokensForUser(c context.Context, userID int) ([]string, error) {
	deviceTokens, err := s.deviceRepo.GetActiveDeviceTokens(c, userID)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0, len(deviceTokens))
	for _, deviceToken := range deviceTokens {
		if deviceToken.Token != "" {
			tokens = append(tokens, deviceToken.Token)
		}
	}
	return tokens, nil
}

func (s *Sender) sendToTokens(c context.Context, tokens []string, payload fcm.FCMNotificationPayload) error {
	log := logging.FromContext(c)

	if s.notifier == nil || s.notifier.FCM == nil {
		log.Debug("FCM is not configured, skipping push notification")
		return nil
	}

	response, err := s.notifier.FCM.SendMulticast(c, tokens, payload)
	if err != nil {
		return err
	}

	if response.FailureCount > 0 {
		for i, result := range response.Responses {
			if !result.Success {
				log.Warn("Failed to send push notification to token",
					"token_index", i,
					"error", result.Error)
			}
		}
	}

	return nil
}
