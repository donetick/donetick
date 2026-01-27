package pushbullet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"donetick.com/core/logging"
)

const pushbulletAPIURL = "https://api.pushbullet.com/v2/pushes"

// HTTPClient is an interface for making HTTP requests, allowing for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type PushbulletNotifier struct {
	apiToken string
	client   HTTPClient
}

func NewPushbulletNotifier(cfg *config.Config) *PushbulletNotifier {
	if cfg.Pushbullet.APIToken == "" {
		return nil
	}
	return &PushbulletNotifier{
		apiToken: cfg.Pushbullet.APIToken,
		client:   &http.Client{},
	}
}

// NewPushbulletNotifierWithClient creates a notifier with a custom HTTP client (for testing).
func NewPushbulletNotifierWithClient(apiToken string, client HTTPClient) *PushbulletNotifier {
	return &PushbulletNotifier{
		apiToken: apiToken,
		client:   client,
	}
}

type pushbulletPush struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (p *PushbulletNotifier) SendNotification(c context.Context, notification *nModel.NotificationDetails) error {
	if notification.TargetID == "" {
		return errors.New("unable to send notification, targetID is empty")
	}
	log := logging.FromContext(c)

	payload := pushbulletPush{
		Type:  "note",
		Title: "Donetick",
		Body:  notification.Text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal pushbullet payload: %w", err)
	}

	req, err := http.NewRequestWithContext(c, http.MethodPost, pushbulletAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create pushbullet request: %w", err)
	}
	req.Header.Set("Access-Token", p.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		log.Debug("Error sending pushbullet notification", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Debug("Pushbullet API error", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("pushbullet API returned status %d", resp.StatusCode)
	}

	return nil
}
