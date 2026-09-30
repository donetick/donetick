// Package bridgenotifier sends Core's FCM notifications through Donetick
// Bridge instead of Core's own direct Firebase Admin SDK config, for
// self-hosted instances connected to Bridge (plan §15 "Notifier
// interface"). It implements the same shape as
// internal/notifier/service/fcm.FCMNotifier.SendNotification so
// internal/notifier.Notifier can select between the two by config.
package bridgenotifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"donetick.com/core/internal/bridge"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
)

// payload mirrors fcm.FCMNotificationPayload's JSON shape (same RawEvent
// convention) without importing the fcm package, to avoid coupling this
// notifier to the direct-Firebase implementation.
type payload struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	ImageURL string            `json:"image_url,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

const defaultAndroidChannelID = "donetick_notifications"

type BridgeNotifier struct {
	bridgeSvc  *bridge.Service
	deviceRepo *dRepo.DeviceRepository
}

func NewBridgeNotifier(bs *bridge.Service, dr *dRepo.DeviceRepository) *BridgeNotifier {
	return &BridgeNotifier{bridgeSvc: bs, deviceRepo: dr}
}

// Enabled reports whether the currently configured Bridge client is usable.
func (b *BridgeNotifier) Enabled() bool {
	return b.bridgeSvc.Client().Enabled()
}

// SendNotification sends one notification through Bridge. notification.
// TargetID is the raw FCM token (same convention as fcm.FCMNotifier); it is
// looked up against the local device table to find the corresponding
// bridgeDeviceId, since Bridge's send API requires both. If the device was
// never successfully registered with Bridge (e.g. registration is still
// pending/retrying), this returns an error rather than silently skipping,
// so the caller's scheduler retry logic applies uniformly.
func (b *BridgeNotifier) SendNotification(ctx context.Context, notification *nModel.NotificationDetails) error {
	log := logging.FromContext(ctx)

	if notification.TargetID == "" {
		return fmt.Errorf("FCM token is required")
	}

	client := b.bridgeSvc.Client()
	if !client.Enabled() {
		return fmt.Errorf("bridge notifier is not enabled")
	}

	device, err := b.deviceRepo.GetActiveDeviceByToken(ctx, notification.UserID, notification.TargetID)
	if err != nil {
		return fmt.Errorf("look up bridge device: %w", err)
	}
	if device == nil || device.BridgeDeviceID == nil || *device.BridgeDeviceID == "" {
		return fmt.Errorf("device not yet registered with bridge")
	}

	var p payload
	if notification.RawEvent != nil {
		if payloadData, ok := notification.RawEvent["payload"]; ok {
			payloadBytes, err := json.Marshal(payloadData)
			if err != nil {
				return fmt.Errorf("marshal payload: %w", err)
			}
			if err := json.Unmarshal(payloadBytes, &p); err != nil {
				return fmt.Errorf("unmarshal payload: %w", err)
			}
		}
	}
	if p.Title == "" && p.Body == "" {
		p.Body = notification.Text
		p.Title = "DoneTick"
	}

	badge := 1
	// idempotencyKey is derived deterministically from the notification's
	// stable identity (DB ID + scheduled time), not a fresh random value,
	// so a scheduler retry of the same logical send reuses it and does not
	// consume additional Bridge quota (plan §15).
	idempotencyKey := bridge.IdempotencyKey("core_notification", strconv.Itoa(notification.ID), notification.ScheduledFor.UTC().Format(time.RFC3339))

	res, err := client.SendNotification(ctx, idempotencyKey,
		[]bridge.DeviceTarget{{BridgeDeviceID: *device.BridgeDeviceID, Token: notification.TargetID}},
		bridge.NotificationPayload{Title: p.Title, Body: p.Body, Data: p.Data},
		bridge.NotificationOptions{AndroidChannelID: defaultAndroidChannelID, Sound: "default", Badge: &badge},
	)
	if err != nil {
		return fmt.Errorf("bridge send: %w", err)
	}

	var sendErr error
	for _, r := range res.Results {
		if r.Status == bridge.StatusInvalidToken {
			// Bridge confirmed the token is permanently unregistered:
			// deactivate the local copy too (plan §15 "Deactivate local
			// invalid tokens when Bridge reports a permanent
			// invalid-token result").
			if err := b.deviceRepo.UnregisterDeviceTokenByToken(ctx, notification.UserID, notification.TargetID); err != nil {
				log.Error("Failed to deactivate locally invalid device token", "error", err)
			}
		}
		if r.Status != "sent" {
			sendErr = fmt.Errorf("bridge delivery status: %s", r.Status)
		}
	}
	return sendErr
}
