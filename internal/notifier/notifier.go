package notifier

import (
	"context"

	"donetick.com/core/internal/events"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/internal/notifier/service/discord"
	"donetick.com/core/internal/notifier/service/fcm"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	shoutrrrNotif "donetick.com/core/internal/notifier/service/shoutrrr"
	telegram "donetick.com/core/internal/notifier/service/telegram"

	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram       *telegram.TelegramNotifier
	Pushover       *pushover.Pushover
	discord        *discord.DiscordNotifier
	FCM            *fcm.FCMNotifier
	eventsProducer *events.EventsProducer
	shoutrrr       *shoutrrrNotif.ShoutrrrNotifier
}

type NotifierParams struct {
	Telegram       *telegram.TelegramNotifier
	Pushover       *pushover.Pushover
	EventsProducer *events.EventsProducer
	Discord        *discord.DiscordNotifier
	FCM            *fcm.FCMNotifier
	Shoutrrr       *shoutrrrNotif.ShoutrrrNotifier
}

func NewNotifier(params *NotifierParams) *Notifier {
	return &Notifier{
		Telegram:       params.Telegram,
		Pushover:       params.Pushover,
		eventsProducer: params.EventsProducer,
		discord:        params.Discord,
		FCM:            params.FCM,
		shoutrrr:       params.Shoutrrr,
	}
}

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
		if n.FCM == nil {
			log.Error("FCM is not initialized, Skipping sending message")
			return nil
		}
		err = n.FCM.SendNotification(c, notification)

	case nModel.NotificationPlatformWebhook:
		// TODO: Implement webhook notification
		// currently we have eventProducer to send events always as a webhook
		// if NotificationPlatform is selected. this a case to catch
		// when we only want to send a webhook
	case nModel.NotificationPlatformShoutrrr:
		if n.FCM == nil {
			log.Error("Shoutrrr is not initialized, Skipping sending message")

			return nil
		}
		err = n.shoutrrr.SendNotification(c, notification)

	default:
		log.Error("Unknown notification type", "type", notification.TypeID)
		return nil
	}
	if err != nil {
		log.Error("Failed to send notification", "err", err)
	}

	return nil
}
