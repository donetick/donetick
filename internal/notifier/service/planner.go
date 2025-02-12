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
	var assignedUser *cModel.UserCircleDetail
	for _, member := range circleMembers {
		if member.UserID == chore.AssignedTo {
			assignedUser = member
			break
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
		return false
	}
	if chore.NextDueDate == nil {
		return true
	}
	if mt.DueDate {
		notifications = append(notifications, generateDueNotifications(chore, assignedUser))
	}
	if mt.PreDue {
		notifications = append(notifications, generatePreDueNotifications(chore, assignedUser))
	}
	if mt.Nagging {
		notifications = append(notifications, generateOverdueNotifications(chore, assignedUser)...)
	}
	if mt.CircleGroup {
		notifications = append(notifications, generateCircleGroupNotifications(chore, mt)...)
	}
	log.Debug("Generated notifications", "count", len(notifications))
	n.nRepo.BatchInsertNotifications(notifications)
	return true
}

func generateDueNotifications(chore *chModel.Chore, assignedUser *cModel.UserCircleDetail) *nModel.Notification {

	notification := &nModel.Notification{
		ChoreID:      chore.ID,
		IsSent:       false,
		ScheduledFor: *chore.NextDueDate,
		CreatedAt:    time.Now().UTC(),
		TypeID:       assignedUser.NotificationType,

		UserID:   assignedUser.UserID,
		TargetID: assignedUser.TargetID,
		Text:     fmt.Sprintf("📅 Reminder: *%s* is due today and assigned to %s.", chore.Name, assignedUser.DisplayName),
		RawEvent: map[string]interface{}{
			"id":                chore.ID,
			"name":              chore.Name,
			"due_date":          chore.NextDueDate,
			"assignee":          assignedUser.DisplayName,
			"assignee_username": assignedUser.Username,
		},
	}

	return notification
}

func generatePreDueNotifications(chore *chModel.Chore, assignedUser *cModel.UserCircleDetail) *nModel.Notification {

	notification := &nModel.Notification{
		ChoreID:      chore.ID,
		IsSent:       false,
		ScheduledFor: *chore.NextDueDate,
		CreatedAt:    time.Now().UTC().Add(-time.Hour * 3),
		TypeID:       assignedUser.NotificationType,
		UserID:       assignedUser.UserID,
		CircleID:     assignedUser.CircleID,
		TargetID:     assignedUser.TargetID,

		Text: fmt.Sprintf("📢 Heads up! *%s* is due soon (on %s) and assigned to %s.", chore.Name, chore.NextDueDate.Format("January 2nd"), assignedUser.DisplayName),

		RawEvent: map[string]interface{}{
			"id":                chore.ID,
			"name":              chore.Name,
			"due_date":          chore.NextDueDate,
			"assignee":          assignedUser.DisplayName,
			"assignee_username": assignedUser.Username,
		},
	}

	return notification

}

func generateOverdueNotifications(chore *chModel.Chore, assignedUser *cModel.UserCircleDetail) []*nModel.Notification {
	var notifications []*nModel.Notification
	for _, hours := range []int{24, 48, 72} {
		scheduleTime := chore.NextDueDate.Add(time.Hour * time.Duration(hours))
		notification := &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: scheduleTime,
			CreatedAt:    time.Now().UTC(),
			TypeID:       assignedUser.NotificationType,
			UserID:       assignedUser.UserID,
			CircleID:     assignedUser.CircleID,
			TargetID:     fmt.Sprint(assignedUser.TargetID),
			Text:         fmt.Sprintf("🚨 *%s* is now %d hours overdue. Please complete it as soon as possible. (Assigned to %s)", chore.Name, hours, assignedUser.DisplayName),
			RawEvent: map[string]interface{}{
				"id":                chore.ID,
				"type":              EventTypeOverdue,
				"name":              chore.Name,
				"due_date":          chore.NextDueDate,
				"assignee":          assignedUser.DisplayName,
				"assignee_username": assignedUser.Username,
			},
		}
		notifications = append(notifications, notification)
	}

	return notifications

}

func generateCircleGroupNotifications(chore *chModel.Chore, mt *chModel.NotificationMetadata) []*nModel.Notification {
	var notifications []*nModel.Notification
	if !mt.CircleGroup || mt.CircleGroupID == nil || *mt.CircleGroupID == 0 {
		return notifications
	}
	if mt.DueDate {
		notification := &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: *chore.NextDueDate,
			CreatedAt:    time.Now().UTC(),
			TypeID:       1,
			TargetID:     fmt.Sprint(*mt.CircleGroupID),
			Text:         fmt.Sprintf("📅 Reminder: *%s* is due today.", chore.Name),
			RawEvent: map[string]interface{}{
				"id":       chore.ID,
				"type":     EventTypeDue,
				"name":     chore.Name,
				"due_date": chore.NextDueDate.Format("January 2nd"),
			},
		}
		if notification.IsValid() {
			notifications = append(notifications, notification)
		}

	}
	if mt.PreDue {
		notification := &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: *chore.NextDueDate,
			CreatedAt:    time.Now().UTC().Add(-time.Hour * 3),
			TypeID:       3,
			TargetID:     fmt.Sprint(*mt.CircleGroupID),
			Text:         fmt.Sprintf("📢 Heads up! *%s* is due soon (on %s).", chore.Name, chore.NextDueDate.Format("January 2nd")),
			RawEvent: map[string]interface{}{
				"id":       chore.ID,
				"type":     EventTypePreDue,
				"name":     chore.Name,
				"due_date": chore.NextDueDate.Format("January 2nd"),
			},
		}
		if notification.IsValid() {
			notifications = append(notifications, notification)
		}

	}
	if mt.Nagging {
		for _, hours := range []int{24, 48, 72} {
			scheduleTime := chore.NextDueDate.Add(time.Hour * time.Duration(hours))
			notification := &nModel.Notification{
				ChoreID:      chore.ID,
				IsSent:       false,
				ScheduledFor: scheduleTime,
				CreatedAt:    time.Now().UTC(),
				TypeID:       2,
				TargetID:     fmt.Sprint(*mt.CircleGroupID),
				Text:         fmt.Sprintf("🚨 *%s* is now %d hours overdue. Please complete it as soon as possible.", chore.Name, hours),
				RawEvent: map[string]interface{}{
					"id":       chore.ID,
					"type":     EventTypeOverdue,
					"name":     chore.Name,
					"due_date": chore.NextDueDate.Format("January 2nd"),
				},
			}
			if notification.IsValid() {
				notifications = append(notifications, notification)
			}
		}
	}

	return notifications
}

type EventType string

const (
	EventTypeUnknown EventType = "unknown"
	EventTypeDue     EventType = "due"
	EventTypePreDue  EventType = "pre_due"
	EventTypeOverdue EventType = "overdue"
)
