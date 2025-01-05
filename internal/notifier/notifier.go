package notifier

import (
	"context"

	nModel "donetick.com/core/internal/notifier/model"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	webhook "donetick.com/core/internal/notifier/service/webhook"
	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram *telegram.TelegramNotifier
	Pushover *pushover.Pushover
	Webhook  *webhook.WebhookNotifier
}

func NewNotifier(t *telegram.TelegramNotifier, p *pushover.Pushover, w *webhook.WebhookNotifier) *Notifier {
	return &Notifier{
		Telegram: t,
		Pushover: p,
		Webhook:  w,
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
	case nModel.NotificationTypeWebhook:
		if notification.WebhookURL == "" {
			log.Error("Webhook URL is empty, Skipping sending message")
			return nil
		}
		if notification.WebhookMethod != nModel.GET && notification.WebhookMethod != nModel.POST {
			log.Error("Webhook method is not valid, Skipping sending message")
			return nil
		}
		return n.Webhook.SendNotification(c, notification)
	}
	return nil
}
