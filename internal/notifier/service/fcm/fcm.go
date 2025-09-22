package fcm

import (
	"context"
	"encoding/json"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
)

type FCMNotifier struct {
	client *messaging.Client
	app    *firebase.App
}

type FCMNotificationPayload struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	ImageURL string            `json:"image_url,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

func NewFCMNotifier(config *config.Config) (*FCMNotifier, error) {
	if config == nil {
		return nil, nil // Return nil when config is nil - allows app to start without FCM
	}

	// Check if FCM is configured - if not, return nil to allow app to start without FCM
	if config.FCM.ProjectID == "" {
		return nil, nil
	}

	var app *firebase.App
	var err error

	if config.FCM.CredentialsPath != "" {
		// Initialize with service account key file
		opt := option.WithCredentialsFile(config.FCM.CredentialsPath)
		app, err = firebase.NewApp(context.Background(), &firebase.Config{
			ProjectID: config.FCM.ProjectID,
		}, opt)
	} else {
		// Initialize with default credentials (useful for GCP environments)
		app, err = firebase.NewApp(context.Background(), &firebase.Config{
			ProjectID: config.FCM.ProjectID,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %v", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize FCM client: %v", err)
	}

	return &FCMNotifier{
		client: client,
		app:    app,
	}, nil
}

func (f *FCMNotifier) SendNotification(ctx context.Context, notification *nModel.NotificationDetails) error {
	log := logging.FromContext(ctx)

	if notification.TargetID == "" {
		return fmt.Errorf("FCM token is required")
	}

	// Parse the notification payload
	var payload FCMNotificationPayload
	if notification.RawEvent != nil {
		if payloadData, ok := notification.RawEvent["payload"]; ok {
			payloadBytes, err := json.Marshal(payloadData)
			if err != nil {
				log.Error("Failed to marshal FCM payload", "error", err)
				return err
			}
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Error("Failed to unmarshal FCM payload", "error", err)
				return err
			}
		}
	}

	// Use notification text as fallback if payload is empty
	if payload.Title == "" && payload.Body == "" {
		payload.Body = notification.Text
		payload.Title = "DoneTick"
	}

	// Create FCM message
	message := &messaging.Message{
		Token: notification.TargetID,
		Notification: &messaging.Notification{
			Title:    payload.Title,
			Body:     payload.Body,
			ImageURL: payload.ImageURL,
		},
		Data: payload.Data,
		Android: &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				ChannelID: "donetick_notifications",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: payload.Title,
						Body:  payload.Body,
					},
					Badge: getBadgePointer(1),
					Sound: "default",
				},
			},
		},
	}

	// Send the message
	response, err := f.client.Send(ctx, message)
	if err != nil {
		log.Error("Failed to send FCM notification", "error", err, "token", notification.TargetID)
		return err
	}

	log.Info("FCM notification sent successfully", "message_id", response, "token", notification.TargetID)
	return nil
}

func (f *FCMNotifier) SendMulticast(ctx context.Context, tokens []string, payload FCMNotificationPayload) (*messaging.BatchResponse, error) {
	log := logging.FromContext(ctx)

	if len(tokens) == 0 {
		return nil, fmt.Errorf("at least one FCM token is required")
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title:    payload.Title,
			Body:     payload.Body,
			ImageURL: payload.ImageURL,
		},
		Data: payload.Data,
		Android: &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				ChannelID: "donetick_notifications",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: payload.Title,
						Body:  payload.Body,
					},
					Badge: getBadgePointer(1),
					Sound: "default",
				},
			},
		},
	}

	response, err := f.client.SendEachForMulticast(ctx, message)
	if err != nil {
		log.Error("Failed to send multicast FCM notification", "error", err)
		return nil, err
	}

	log.Info("FCM multicast notification sent",
		"success_count", response.SuccessCount,
		"failure_count", response.FailureCount,
		"total_tokens", len(tokens))

	return response, nil
}

func (f *FCMNotifier) ValidateToken(ctx context.Context, token string) error {
	// Create a test message without sending it
	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"test": "validation",
		},
	}

	// Use dry run to validate the token
	response, err := f.client.SendDryRun(ctx, message)
	if err != nil {
		return fmt.Errorf("invalid FCM token: %v", err)
	}

	logging.FromContext(ctx).Info("FCM token validated successfully", "message_id", response)
	return nil
}

func getBadgePointer(badge int) *int {
	return &badge
}
