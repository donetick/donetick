package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	"donetick.com/core/logging"
)

type NotificationPlanner struct {
	nRepo *nRepo.NotificationRepository
	cRepo *cRepo.CircleRepository
}

func NewNotificationPlanner(nr *nRepo.NotificationRepository, cr *cRepo.CircleRepository) *NotificationPlanner {
	return &NotificationPlanner{nRepo: nr,
		cRepo: cr,
	}
}

func (n *NotificationPlanner) GenerateNotifications(c context.Context, chore *chModel.Chore) bool {
	log := logging.FromContext(c)
	circleMembers, err := n.cRepo.GetCircleUsers(c, chore.CircleID)
	assignees := make([]*cModel.UserCircleDetail, 0)
	for _, member := range circleMembers {
		if member.ID == chore.AssignedTo {
			assignees = append(assignees, member)
		}
	}

	if err != nil {
		log.Error("Error getting circle members", err)
		return false
	}
	n.nRepo.DeleteAllChoreNotifications(chore.ID)
	notifications := make([]*nModel.Notification, 0)
	if !chore.Notification || chore.FrequencyType == "trigger" {

		return true
	}
	var mt *chModel.NotificationMetadata
	if err := json.Unmarshal([]byte(*chore.NotificationMetadata), &mt); err != nil {
		log.Error("Error unmarshalling notification metadata", err)
		return true
	}
	if mt.DueDate {
		notifications = append(notifications, generateDueNotifications(chore, assignees)...)
	}
	if mt.PreDue {
		notifications = append(notifications, generatePreDueNotifications(chore, assignees)...)
	}
	if mt.Nagging {
		notifications = append(notifications, generateOverdueNotifications(chore, assignees)...)
	}

	n.nRepo.BatchInsertNotifications(notifications)
	return true
}

func generateDueNotifications(chore *chModel.Chore, users []*cModel.UserCircleDetail) []*nModel.Notification {
	var assignee *cModel.UserCircleDetail
	notifications := make([]*nModel.Notification, 0)
	for _, user := range users {
		if user.ID == chore.AssignedTo {
			assignee = user
			break
		}
	}
	for _, user := range users {

		notification := &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: *chore.NextDueDate,
			CreatedAt:    time.Now().UTC(),
			TypeID:       1,
			UserID:       user.ID,
			TargetID:     fmt.Sprint(user.ChatID),
			Text:         fmt.Sprintf("📅 Reminder: '%s' is due today and assigned to %s.", chore.Name, assignee.DisplayName),
		}
		notifications = append(notifications, notification)
	}

	return notifications
}

func generatePreDueNotifications(chore *chModel.Chore, users []*cModel.UserCircleDetail) []*nModel.Notification {
	var assignee *cModel.UserCircleDetail
	for _, user := range users {
		if user.ID == chore.AssignedTo {
			assignee = user
			break
		}
	}
	notifications := make([]*nModel.Notification, 0)
	for _, user := range users {
		notification := &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: *chore.NextDueDate,
			CreatedAt:    time.Now().UTC().Add(-time.Hour * 3),
			TypeID:       3,
			UserID:       user.ID,
			TargetID:     fmt.Sprint(user.ChatID),
			Text:         fmt.Sprintf("📢 Heads up! Chore '%s' is due soon (on %s) and assigned to %s.", chore.Name, chore.NextDueDate.Format("January 2nd"), assignee.DisplayName),
		}
		notifications = append(notifications, notification)

	}
	return notifications

}

func generateOverdueNotifications(chore *chModel.Chore, users []*cModel.UserCircleDetail) []*nModel.Notification {
	var assignee *cModel.UserCircleDetail
	for _, user := range users {
		if user.ID == chore.AssignedTo {
			assignee = user
			break
		}
	}
	notifications := make([]*nModel.Notification, 0)
	for _, hours := range []int{24, 48, 72} {
		scheduleTime := chore.NextDueDate.Add(time.Hour * time.Duration(hours))
		for _, user := range users {
			notification := &nModel.Notification{
				ChoreID:      chore.ID,
				IsSent:       false,
				ScheduledFor: scheduleTime,
				CreatedAt:    time.Now().UTC(),
				TypeID:       2,
				UserID:       user.ID,
				TargetID:     fmt.Sprint(user.ChatID),
				Text:         fmt.Sprintf("🚨  '%s' is now %d hours overdue. Please complete it as soon as possible. (Assigned to %s)", chore.Name, hours, assignee.DisplayName),
			}
			notifications = append(notifications, notification)
		}
	}

	return notifications

}
