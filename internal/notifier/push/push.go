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
	uModel "donetick.com/core/internal/user/model"
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
	deviceTokens, err := s.deviceRepo.GetActiveDeviceTokens(c, userID)
	if err != nil {
		return 0, err
	}
	return s.SendToDeviceTokens(c, "push", deviceTokens, payload)
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

// SendToDeviceTokens sends payload to a set of already-resolved device
// tokens, preferring Bridge (self-hosted instances relaying through Donetick
// Bridge) when it is connected and enabled, and falling back to Core's own
// direct Firebase config otherwise -- the same provider selection
// Notifier.SendNotification uses for scheduled chore reminders. This is the
// single place that selection lives, so callers that already hold device
// rows (e.g. chore nudges) don't need to duplicate it.
// idempotencyScope distinguishes callers for Bridge's idempotency key (e.g.
// "push", "nudge") so unrelated sends never collide on the same key.
func (s *Sender) SendToDeviceTokens(c context.Context, idempotencyScope string, deviceTokens []*uModel.UserDeviceToken, payload fcm.FCMNotificationPayload) (int, error) {
	log := logging.FromContext(c)

	if len(deviceTokens) == 0 {
		return 0, nil
	}

	if s.notifier != nil && s.notifier.Bridge != nil && s.notifier.Bridge.Enabled() {
		return s.notifier.Bridge.SendMulticast(c, idempotencyScope, deviceTokens, payload.Title, payload.Body, payload.Data)
	}

	if s.notifier == nil || s.notifier.FCM == nil {
		log.Debug("Neither Bridge nor FCM is configured, skipping push notification")
		return 0, nil
	}

	tokens := make([]string, 0, len(deviceTokens))
	for _, deviceToken := range deviceTokens {
		if deviceToken.Token != "" {
			tokens = append(tokens, deviceToken.Token)
		}
	}
	if len(tokens) == 0 {
		return 0, nil
	}

	response, err := s.notifier.FCM.SendMulticast(c, tokens, payload)
	if err != nil {
		return 0, err
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

	return response.SuccessCount, nil
}
