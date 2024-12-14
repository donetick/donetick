package notifier

import (
	"context"

	nModel "donetick.com/core/internal/notifier/model"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram *telegram.TelegramNotifier
	Pushover *pushover.Pushover
}

func NewNotifier(t *telegram.TelegramNotifier, p *pushover.Pushover) *Notifier {
	return &Notifier{
		Telegram: t,
		Pushover: p,
	}
}

func (n *Notifier) SendNotification(c context.Context, notification *nModel.Notification) error {
	log := logging.FromContext(c)
	switch notification.TypeID {
	case nModel.NotificationTypeTelegram:
		if n.Telegram == nil {
			log.Error("Telegram bot is not initialized, Skipping sending message")
			return nil
		}
		return n.Telegram.SendNotification(c, notification)
	case nModel.NotificationTypePushover:
		if n.Pushover == nil {
			log.Error("Pushover is not initialized, Skipping sending message")
			return nil
		}
		return n.Pushover.SendNotification(c, notification)
	}
	return nil
}
