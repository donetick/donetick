package notifier

import (
	"context"

	nModel "donetick.com/core/internal/notifier/model"
	mqtt "donetick.com/core/internal/notifier/service/mqtt"
	pushover "donetick.com/core/internal/notifier/service/pushover"
	telegram "donetick.com/core/internal/notifier/service/telegram"
	"donetick.com/core/logging"
)

type Notifier struct {
	Telegram *telegram.TelegramNotifier
	Pushover *pushover.Pushover
	Mqtt     *mqtt.MqttNotifier
}

func NewNotifier(t *telegram.TelegramNotifier, p *pushover.Pushover, m *mqtt.MqttNotifier) *Notifier {
	return &Notifier{
		Telegram: t,
		Pushover: p,
		Mqtt:     m,
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
	case nModel.NotificationTypeMqtt:
		if n.Mqtt == nil {
			log.Error("Mqtt is not initialized, Skipping sending message")
			return nil
		}
		return n.Mqtt.SendNotification(c, notification)
	}

	return nil
}
