package mqtt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type MqttNotifier struct {
	client MQTT.Client
	topic  string
}

func NewMqttNotifier(config *config.Config) *MqttNotifier {
	client := MQTT.NewClient(
		MQTT.NewClientOptions().AddBroker(config.Mqtt.Host).SetClientID(config.Mqtt.ClientId).SetUsername(config.Mqtt.Username).SetPassword(config.Mqtt.Password))

	if token := client.Connect(); token.WaitTimeout(time.Second*4) && token.Error() != nil {
		fmt.Printf("Failed to connect to MQTT broker: %v", token.Error())
	} else {
		fmt.Println("Connected to MQTT broker")
	}

	return &MqttNotifier{
		client,
		config.Mqtt.Topic,
	}
}

func (m *MqttNotifier) SendNotification(c context.Context, notification *nModel.Notification) error {
	log := logging.FromContext(c)

	token := m.client.Publish(m.topic, 0, false, notification.Text).WaitTimeout(time.Second * 4)
	if token {
		log.Info("Mqtt notification delivered")
		return nil
	} else {
		log.Error("Mqtt notification failed")
		return errors.New("mqtt notification failed")
	}
}
