// Package dispatch delivers an immediate, event-driven notification to a set of
// users -- "somebody is waiting on you" messages such as a chore completion
// pending approval or a reward redemption request.
//
// It exists because every such caller has to get three things right, and each
// one that hand-rolled the fan-out got a different subset of them wrong:
//
//   - Deliver once. A user with registered devices and a legacy FCM target
//     would otherwise be pushed to twice, through two providers, with two
//     different wordings.
//   - Reach mobile-only users. Queueing rows against user_notification_targets
//     alone silently skips everybody whose only channel is a registered device.
//   - Respect the recipient's notification profile, both now and again at send
//     time (see Scheduler.loadAndSendNotificationJob).
//
// Deciding *who* should hear about something stays with the caller: that needs
// the domain's own visibility rules. Dispatcher takes that audience and handles
// how the message reaches each of them.
//
// It does not schedule anything. For reminders tied to a chore's due date, use
// NotificationPlanner instead.
package dispatch

import (
	"context"
	"time"

	cModel "donetick.com/core/internal/circle/model"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/internal/notifier/push"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/notifier/service/fcm"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type Dispatcher struct {
	nRepo      *nRepo.NotificationRepository
	uRepo      *uRepo.UserRepository
	deviceRepo *dRepo.DeviceRepository
	pushSender *push.Sender
}

func NewDispatcher(nr *nRepo.NotificationRepository, ur *uRepo.UserRepository, dr *dRepo.DeviceRepository, ps *push.Sender) *Dispatcher {
	return &Dispatcher{nRepo: nr, uRepo: ur, deviceRepo: dr, pushSender: ps}
}

// Request is one notification to deliver, independent of who receives it.
type Request struct {
	// EventType gates delivery against each recipient's notification profile and
	// is stored on the queued row so the scheduler can re-check it at send time.
	// An event type AllowedByPreference doesn't recognize is always allowed
	// through, so a new kind of notification is never silently dropped -- but it
	// is also not something a user can turn off, so add a case there for it.
	EventType nps.EventType
	// Priority is the chore priority escalation is gated on; leave it at
	// PriorityNone for notifications that aren't about a chore.
	Priority int
	CircleID int
	// ChoreID links the notification to a chore, and is 0 for subjects that
	// aren't one (a reward redemption, say).
	ChoreID int
	// PushTitle, PushBody and PushData are the payload delivered to registered
	// devices, and are carried on the queued row for FCM fallback delivery.
	PushTitle string
	PushBody  string
	PushData  map[string]string
	// Text is the wording for channels that render a single line of text
	// (Telegram, Discord, Pushover).
	Text string
	// Subject names what the notification is about in the raw event forwarded to
	// webhooks -- a chore or reward name.
	Subject string
	// IdempotencyScope distinguishes callers in Bridge's idempotency key so two
	// unrelated sends never collide on one key.
	IdempotencyScope string
}

// Result reports what delivery was attempted, for logging.
type Result struct {
	Recipients int // how many of the audience wanted this notification
	Devices    int // registered devices pushed to
	Queued     int // rows left for the scheduler to send
}

// NotifyUsers delivers req to each member of audience exactly once: an immediate
// push to their registered devices, plus a queued row for a configured legacy
// target -- a third-party channel always, or FCM only when they have no devices
// for the push above to reach. Recipients who muted req.EventType are dropped.
//
// Best-effort: failures are logged and the remaining recipients still attempted,
// since the action that triggered the notification has already happened.
func (d *Dispatcher) NotifyUsers(c context.Context, req Request, audience []*cModel.UserCircleDetail) Result {
	log := logging.FromContext(c)
	var result Result

	recipients := d.wantNotification(c, req, audience)
	result.Recipients = len(recipients)
	if len(recipients) == 0 {
		return result
	}

	payload := fcm.FCMNotificationPayload{Title: req.PushTitle, Body: req.PushBody, Data: req.PushData}
	queued := make([]*nModel.Notification, 0, len(recipients))

	for _, recipient := range recipients {
		tokens, err := d.deviceRepo.GetActiveDeviceTokens(c, recipient.UserID)
		if err != nil {
			log.Errorw("Failed to load push devices", "error", err, "event", req.EventType, "user_id", recipient.UserID)
		}
		if len(tokens) > 0 {
			sent, err := d.pushSender.SendToDeviceTokens(c, req.IdempotencyScope, tokens, payload)
			if err != nil {
				log.Warnw("Failed to push notification", "error", err, "event", req.EventType, "user_id", recipient.UserID)
			}
			result.Devices += sent
		}

		if recipient.NotificationType == nModel.NotificationPlatformNone || recipient.TargetID == "" {
			continue
		}
		// A legacy FCM target would repeat the push above, so it is only a
		// fallback for recipients with no registered device -- the same rule
		// NotificationPlanner.GenerateAssignmentNotification applies.
		if recipient.NotificationType == nModel.NotificationPlatformFCM && len(tokens) > 0 {
			continue
		}
		queued = append(queued, req.notification(recipient))
	}

	if len(queued) > 0 {
		if err := d.nRepo.BatchInsertNotifications(queued); err != nil {
			log.Errorw("Failed to queue notifications", "error", err, "event", req.EventType)
		} else {
			result.Queued = len(queued)
		}
	}

	log.Debugw("Dispatched notification", "event", req.EventType, "chore_id", req.ChoreID,
		"recipients", result.Recipients, "devices", result.Devices, "queued", result.Queued)
	return result
}

// wantNotification drops the members of audience who muted req.EventType. A
// failure to load preferences leaves everybody in rather than dropping a
// notification somebody may be waiting on, matching the same choice in
// Scheduler.pendingNotificationPreferences.
func (d *Dispatcher) wantNotification(c context.Context, req Request, audience []*cModel.UserCircleDetail) []*cModel.UserCircleDetail {
	userIDs := make([]int, 0, len(audience))
	for _, member := range audience {
		userIDs = append(userIDs, member.UserID)
	}
	prefs, err := d.uRepo.GetNotificationPreferences(c, userIDs)
	if err != nil {
		logging.FromContext(c).Errorw("Error loading notification preferences, skipping preference gating",
			"error", err, "event", req.EventType)
		return audience
	}

	recipients := make([]*cModel.UserCircleDetail, 0, len(audience))
	for _, member := range audience {
		if !nps.AllowedByPreference(prefs[member.UserID], string(req.EventType), req.Priority) {
			continue
		}
		recipients = append(recipients, member)
	}
	return recipients
}

func (r Request) notification(recipient *cModel.UserCircleDetail) *nModel.Notification {
	now := time.Now().UTC()
	rawEvent := map[string]interface{}{
		"type":    r.EventType,
		"name":    r.Subject,
		"payload": map[string]interface{}{"title": r.PushTitle, "body": r.PushBody, "data": r.PushData},
	}
	// Webhook consumers read "id" as a chore id, so a subject that isn't a chore
	// leaves it out rather than reporting chore 0.
	if r.ChoreID != 0 {
		rawEvent["id"] = r.ChoreID
	}
	return &nModel.Notification{
		ChoreID:      r.ChoreID,
		CircleID:     r.CircleID,
		UserID:       recipient.UserID,
		TargetID:     recipient.TargetID,
		TypeID:       recipient.NotificationType,
		Text:         r.Text,
		ScheduledFor: now,
		CreatedAt:    now,
		RawEvent:     rawEvent,
	}
}
