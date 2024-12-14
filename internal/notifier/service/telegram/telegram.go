package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot *tgbotapi.BotAPI
}

func NewTelegramNotifier(config *config.Config) *TelegramNotifier {
	bot, err := tgbotapi.NewBotAPI(config.Telegram.Token)
	if err != nil {
		fmt.Println("Error creating bot: ", err)
		return nil
	}

	return &TelegramNotifier{
		bot: bot,
	}
}

func (tn *TelegramNotifier) SendChoreCompletion(c context.Context, chore *chModel.Chore, user *uModel.User) {

	log := logging.FromContext(c)
	if tn == nil {
		log.Error("Telegram bot is not initialized, Skipping sending message")
		return
	}
	var mt *chModel.NotificationMetadata
	if err := json.Unmarshal([]byte(*chore.NotificationMetadata), &mt); err != nil {
		log.Error("Error unmarshalling notification metadata", err)
	}

	targets := []int64{}
	if user.ChatID != 0 {
		targets = append(targets, user.ChatID)
	}
	if mt.CircleGroup && mt.CircleGroupID != nil {
		// attempt to parse it:

		if *mt.CircleGroupID != 0 {
			targets = append(targets, *mt.CircleGroupID)
		}

	}

	text := fmt.Sprintf("🎉 *%s* is completed! is off the list, %s! 🌟 ", chore.Name, user.DisplayName)
	for _, target := range targets {
		msg := tgbotapi.NewMessage(target, text)

		msg.ParseMode = "Markdown"
		_, err := tn.bot.Send(msg)
		if err != nil {
			log.Error("Error sending message to user: ", err)
			log.Debug("Error sending message, chore: ", chore.Name, " user: ", user.DisplayName, " chatID: ", user.ChatID, " user id: ", user.ID)
		}
	}

}

func (tn *TelegramNotifier) SendNotification(c context.Context, notification *nModel.Notification) error {

	log := logging.FromContext(c)
	if notification.TargetID == "" {
		log.Error("Notification target ID is empty")
		return errors.New("Notification target ID is empty")
	}
	chatID, err := strconv.ParseInt(notification.TargetID, 10, 64)
	if err != nil {
		log.Error("Error parsing chatID: ", err)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, notification.Text)
	msg.ParseMode = "Markdown"
	_, err = tn.bot.Send(msg)
	if err != nil {
		log.Error("Error sending message to user: ", err)
		log.Debug("Error sending message, notification: ", notification.Text, " chatID: ", chatID)
		return err
	}
	return nil
}
