package service

import (
	"context"
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

	if chore.NextDueDate == nil {
		return true
	}
	if chore.NotificationMetadataV2.DueDate {
		notifications = append(notifications, generateDueNotifications(chore, assignedUser))
	}
	if chore.NotificationMetadataV2.PreDue {
		notifications = append(notifications, generatePreDueNotifications(chore, assignedUser))
	}
	if chore.NotificationMetadataV2.Nagging {
		notifications = append(notifications, generateOverdueNotifications(chore, assignedUser)...)
	}
	if chore.NotificationMetadataV2.CircleGroup {
		notifications = append(notifications, generateCircleGroupNotifications(chore, chore.NotificationMetadataV2)...)
	}
	if len(chore.NotificationMetadataV2.Templates) > 0 {
		notifications = append(notifications, generateNotiticationFromTemplete(chore, assignedUser, EventTypeDue)...)
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

// calculateDuration calculates duration based on unit and value
func calculateDuration(value int, unit string) (time.Duration, error) {
	switch unit {
	case "minutes":
		return time.Duration(value) * time.Minute, nil
	case "hours":
		return time.Duration(value) * time.Hour, nil
	case "days":
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported time unit: %s", unit)
	}
}

// calculateScheduledTime calculates the scheduled time based on template configuration
func calculateScheduledTime(baseTime time.Time, template *chModel.NotificaitonTemplate) (time.Time, error) {
	if template == nil {
		return baseTime, fmt.Errorf("template is nil")
	}

	duration, err := calculateDuration(template.Value, template.Unit)
	if err != nil {
		return baseTime, err
	}

	if template.Type == "before" {
		duration = -duration
	}

	return baseTime.Add(duration), nil
}

func generateNotiticationFromTemplete(chore *chModel.Chore, assignedUser *cModel.UserCircleDetail, eventType EventType) []*nModel.Notification {
	if len(chore.NotificationMetadataV2.Templates) == 0 {
		return nil // No templates to process
	}

	notifications := make([]*nModel.Notification, 0)

	for _, template := range chore.NotificationMetadataV2.Templates {
		scheduledTime, err := calculateScheduledTime(*chore.NextDueDate, template)
		if err != nil {
			// Log error and fallback to due date
			scheduledTime = *chore.NextDueDate
		}
		// don't schedule if the time already pass :
		if scheduledTime.Before(time.Now().UTC()) {
			logging.FromContext(context.Background()).Debug("Skipping notification for template, scheduled time has passed", "scheduled_time", scheduledTime)
			continue
		}
		notifications = append(notifications, &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: scheduledTime,
			CreatedAt:    time.Now().UTC(),
			TypeID:       assignedUser.NotificationType,
			UserID:       assignedUser.UserID,
			CircleID:     assignedUser.CircleID,
			TargetID:     assignedUser.TargetID,
			Text:         fmt.Sprintf("📅 Reminder: *%s* is due today and assigned to %s.", chore.Name, assignedUser.DisplayName),
			RawEvent: map[string]interface{}{
				"id":                chore.ID,
				"type":              eventType,
				"name":              chore.Name,
				"due_date":          chore.NextDueDate,
				"assignee":          assignedUser.DisplayName,
				"assignee_username": assignedUser.Username,
			},
		})

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
