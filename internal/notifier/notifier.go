package notifier

import (
	"context"

	"donetick.com/core/internal/events"
	nModel "donetick.com/core/internal/notifier/model"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"

	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram       *telegram.TelegramNotifier
	Pushover       *pushover.Pushover
	eventsProducer *events.EventsProducer
}

func NewNotifier(t *telegram.TelegramNotifier, p *pushover.Pushover, ep *events.EventsProducer) *Notifier {
	return &Notifier{
		Telegram:       t,
		Pushover:       p,
		eventsProducer: ep,
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
	}
	if err != nil {
		log.Error("Failed to send notification", "err", err)
	}

	return nil
}
