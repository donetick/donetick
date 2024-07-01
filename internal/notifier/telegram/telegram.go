package telegram

import (
	"context"
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

func (tn *TelegramNotifier) SendChoreReminder(c context.Context, chore *chModel.Chore, users []*uModel.User) {
	for _, user := range users {
		var assignee *uModel.User
		if user.ID == chore.AssignedTo {
			if user.ChatID == 0 {
				continue
			}
			assignee = user
			text := fmt.Sprintf("*%s* is due today and assigned to *%s*", chore.Name, assignee.DisplayName)
			msg := tgbotapi.NewMessage(user.ChatID, text)
			msg.ParseMode = "Markdown"
			_, err := tn.bot.Send(msg)
			if err != nil {
				fmt.Println("Error sending message to user: ", err)
			}
			break
		}
	}
}

func (tn *TelegramNotifier) SendChoreCompletion(c context.Context, chore *chModel.Chore, users []*uModel.User) {
	log := logging.FromContext(c)
	for _, user := range users {
		if user.ChatID == 0 {
			continue
		}
		text := fmt.Sprintf("🎉 '%s' is completed! is off the list, %s! 🌟 ", chore.Name, user.DisplayName)
		msg := tgbotapi.NewMessage(user.ChatID, text)
		msg.ParseMode = "Markdown"
		_, err := tn.bot.Send(msg)
		if err != nil {
			log.Error("Error sending message to user: ", err)
			log.Debug("Error sending message, chore: ", chore.Name, " user: ", user.DisplayName, " chatID: ", user.ChatID, " user id: ", user.ID)
		}

	}
}

func (tn *TelegramNotifier) SendChoreOverdue(c context.Context, chore *chModel.Chore, users []*uModel.User) {
	log := logging.FromContext(c)
	for _, user := range users {
		if user.ChatID == 0 {
			continue
		}
		text := fmt.Sprintf("*%s* is overdue and assigned to *%s*", chore.Name, user.DisplayName)
		msg := tgbotapi.NewMessage(user.ChatID, text)
		msg.ParseMode = "Markdown"
		_, err := tn.bot.Send(msg)
		if err != nil {
			log.Error("Error sending message to user: ", err)
			log.Debug("Error sending message, chore: ", chore.Name, " user: ", user.DisplayName, " chatID: ", user.ChatID, " user id: ", user.ID)
		}
	}
}

func (tn *TelegramNotifier) SendChorePreDue(c context.Context, chore *chModel.Chore, users []*uModel.User) {
	log := logging.FromContext(c)
	for _, user := range users {
		if user.ID != chore.AssignedTo {
			continue
		}
		if user.ChatID == 0 {
			continue
		}
		text := fmt.Sprintf("*%s* is due tomorrow and assigned to *%s*", chore.Name, user.DisplayName)
		msg := tgbotapi.NewMessage(user.ChatID, text)
		msg.ParseMode = "Markdown"
		_, err := tn.bot.Send(msg)
		if err != nil {
			log.Error("Error sending message to user: ", err)
			log.Debug("Error sending message, chore: ", chore.Name, " user: ", user.DisplayName, " chatID: ", user.ChatID, " user id: ", user.ID)
		}
	}
}

func (tn *TelegramNotifier) SendNotification(c context.Context, notification *nModel.Notification) {

	log := logging.FromContext(c)
	if notification.TargetID == "" {
		log.Error("Notification target ID is empty")
		return
	}
	chatID, err := strconv.ParseInt(notification.TargetID, 10, 64)
	if err != nil {
		log.Error("Error parsing chatID: ", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, notification.Text)
	msg.ParseMode = "Markdown"
	_, err = tn.bot.Send(msg)
	if err != nil {
		log.Error("Error sending message to user: ", err)
		log.Debug("Error sending message, notification: ", notification.Text, " chatID: ", chatID)
	}
}
