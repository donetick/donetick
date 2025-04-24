package service

import (
	"context"

	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
)

type Notifier interface {
	SendChoreCompletion(c context.Context, chore *chModel.Chore, user *uModel.User)
	SendNotification(c context.Context, notification *nModel.NotificationDetails) error
}
