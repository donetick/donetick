package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/internal/webhook"
	"donetick.com/core/logging"
)

// discordPolicy is deliberately strict: Discord is on the public internet, so
// a target that resolves anywhere private is either a mistake or an attempt
// to use the notifier as a proxy.
var discordPolicy = webhook.Policy{
	BlockPrivateNetworks: true,
	RequireHTTPS:         true,
}

var discordClient = webhook.NewClient(discordPolicy, 30*time.Second, 0)

type DiscordNotifier struct {
}

func NewDiscordNotifier(config *config.Config) *DiscordNotifier {
	return &DiscordNotifier{}
}

func (dn *DiscordNotifier) SendChoreCompletion(c context.Context, chore *chModel.Chore, user *uModel.User) {
	log := logging.FromContext(c)
	if dn == nil {
		log.Error("Discord notifier is not initialized, skipping message sending")
		return
	}

	message := fmt.Sprintf("🎉 **%s** is completed! Great job, %s! 🌟", chore.Name, user.DisplayName)
	err := dn.sendMessage(c, user.UserNotificationTargets.TargetID, message)
	if err != nil {
		log.Error("Error sending Discord message:", err)
	}
}

func (dn *DiscordNotifier) SendNotification(c context.Context, notification *nModel.NotificationDetails) error {

	if dn == nil {
		return errors.New("Discord notifier is not initialized")
	}

	if notification.Text == "" {
		return errors.New("unable to send notification, text is empty")
	}

	return dn.sendMessage(c, notification.TargetID, notification.Text)
}

func (dn *DiscordNotifier) sendMessage(c context.Context, webhookURL string, message string) error {
	log := logging.FromContext(c)

	if webhookURL == "" {
		return errors.New("unable to send notification, webhook URL is empty")
	}

	// Targets stored before this check existed, or edited straight in the
	// database, are checked here as well as on the way in.
	if err := webhook.ValidateDiscordWebhookURL(webhookURL); err != nil {
		log.Errorw("Refusing to send Discord message, target is not a Discord webhook", "error", err)
		return err
	}

	payload := map[string]string{"content": message}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error("Error marshaling JSON:", err)
		return err
	}

	resp, err := discordClient.PostJSON(c, webhookURL, jsonData, nil)
	if err != nil {
		log.Error("Error sending message to Discord:", err)
		return err
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		log.Error("Discord webhook returned unexpected status:", resp.Status)
		return errors.New("failed to send Discord message")
	}

	return nil
}
