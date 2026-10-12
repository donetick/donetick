package notifier

import (
	"context"

	"donetick.com/core/internal/events"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/internal/notifier/service/bridgenotifier"
	"donetick.com/core/internal/notifier/service/discord"
	"donetick.com/core/internal/notifier/service/fcm"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"

	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram       *telegram.TelegramNotifier
	Pushover       *pushover.Pushover
	discord        *discord.DiscordNotifier
	FCM            *fcm.FCMNotifier
	Bridge         *bridgenotifier.BridgeNotifier
	eventsProducer *events.EventsProducer
}

func NewNotifier(t *telegram.TelegramNotifier, p *pushover.Pushover, ep *events.EventsProducer, d *discord.DiscordNotifier, f *fcm.FCMNotifier, b *bridgenotifier.BridgeNotifier) *Notifier {
	return &Notifier{
		Telegram:       t,
		Pushover:       p,
		eventsProducer: ep,
		discord:        d,
		FCM:            f,
		Bridge:         b,
	}
}

// SendNotification dispatches notification to the right provider and
// returns its real error. Previously this always returned nil even after a
// provider failure (only logging it), which caused the scheduler
// (internal/notifier.Scheduler.loadAndSendNotificationJob) to mark every
// attempted notification as sent regardless of outcome. That silently
// dropped failed sends -- including, critically, Bridge quota/availability
// errors -- instead of leaving them pending for retry. Fixed here for every
// provider, not just Bridge, since the bug was general.
func (n *Notifier) SendNotification(c context.Context, notification *nModel.NotificationDetails) error {
	log := logging.FromContext(c)
	var err error
	switch notification.TypeID {
	case nModel.NotificationPlatformTelegram:
		if n.Telegram == nil {
			log.Error("Telegram bot is not initialized, Skipping sending message")
			return nil
		}
		err = n.Telegram.SendNotification(c, notification)
	case nModel.NotificationPlatformPushover:
		if n.Pushover == nil {
			log.Error("Pushover is not initialized, Skipping sending message")
			return nil
		}
		err = n.Pushover.SendNotification(c, notification)
	case nModel.NotificationPlatformDiscord:
		if n.discord == nil {
			log.Error("Discord is not initialized, Skipping sending message")
			return nil
		}
		err = n.discord.SendNotification(c, notification)
	case nModel.NotificationPlatformFCM:
		// Prefer Bridge when it is connected and enabled (self-hosted
		// instance relaying through Donetick Bridge); otherwise
		// fall back to Core's own direct Firebase config, preserving
		// existing behavior for Donetick Cloud / self-managed-FCM
		// installs.
		if n.Bridge != nil && n.Bridge.Enabled() {
			err = n.Bridge.SendNotification(c, notification)
		} else if n.FCM != nil {
			err = n.FCM.SendNotification(c, notification)
		} else {
			log.Error("Neither Bridge nor FCM is initialized, Skipping sending message")
			return nil
		}

	case nModel.NotificationPlatformWebhook:
		// TODO: Implement webhook notification
		// currently we have eventProducer to send events always as a webhook
		// if NotificationPlatform is selected. this a case to catch
		// when we only want to send a webhook

	default:
		log.Error("Unknown notification type", "type", notification.TypeID)
		return nil
	}
	if err != nil {
		log.Error("Failed to send notification", "err", err)
	}

	return err
}
