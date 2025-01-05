package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
)

type WebhookNotifier struct {
}

func NewWebhookNotifier(config *config.Config) *WebhookNotifier {
	return &WebhookNotifier{}
}

func (m *WebhookNotifier) SendNotification(c context.Context, notification *nModel.Notification) error {
	log := logging.FromContext(c)

	notificationData := map[string]string{
		"choreId": strconv.Itoa(notification.ChoreID),
		"userId":  strconv.Itoa(notification.UserID),
		"text":    notification.Text,
	}

	notificationJSON, err := json.Marshal(notificationData)
	if err != nil {
		log.Error("Error marshalling notification data to JSON", "error", err)
		return err
	}

	log.Debug("Sending webhook notification", "notification", notification)
	if notification.WebhookMethod == "GET" {
		req, err := http.NewRequest("GET", notification.WebhookURL, nil)
		if err != nil {
			log.Error("Error creating GET request", "error", err)
			return err
		}
		q := req.URL.Query()
		for key, value := range notificationData {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Error("Error making GET request", "error", err)
			return err
		}
		defer resp.Body.Close()
	} else if notification.WebhookMethod == "POST" {
		resp, err := http.Post(notification.WebhookURL, "application/json", bytes.NewBuffer(notificationJSON))
		if err != nil {
			log.Error("Error making POST request", "error", err)
			return err
		}
		defer resp.Body.Close()
	}

	return nil
}
