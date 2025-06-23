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

	if len(chore.NotificationMetadataV2.Templates) > 0 {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, nil)...)
	}

	if chore.NotificationMetadataV2.CircleGroup {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, chore.NotificationMetadataV2.CircleGroupID)...)
	}

	log.Debug("Generated notifications", "count", len(notifications))
	n.nRepo.BatchInsertNotifications(notifications)
	return true
}

func getEventTypeFromTemplate(template *chModel.NotificationTemplate) EventType {
	if template == nil {
		return EventTypeUnknown
	}
	if template.Value < 0 {
		return EventTypePreDue
	} else if template.Value == 0 {
		return EventTypeDue
	} else {
		return EventTypeOverdue
	}
}

// calculateDuration calculates duration based on unit and value
func calculateDuration(value int, unit chModel.NotificationTemplateUnit) (time.Duration, error) {
	switch unit {
	case chModel.NotificationTemplateUnitMinute:
		return time.Duration(value) * time.Minute, nil
	case chModel.NotificationTemplateUnitHour:
		return time.Duration(value) * time.Hour, nil
	case chModel.NotificationTemplateUnitDay:
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported time unit: %s", unit)
	}
}

// calculateScheduledTime calculates the scheduled time based on template configuration
func calculateScheduledTime(baseTime time.Time, template *chModel.NotificationTemplate) (time.Time, error) {
	if template == nil {
		return baseTime, fmt.Errorf("template is nil")
	}

	duration, err := calculateDuration(template.Value, template.Unit)
	if err != nil {
		return baseTime, err
	}

	return baseTime.Add(duration), nil
}

func generateNotificationsFromTemplate(chore *chModel.Chore, assignedUser *cModel.UserCircleDetail, overrideTarget *int64) []*nModel.Notification {
	if len(chore.NotificationMetadataV2.Templates) == 0 {
		return nil // No templates to process
	}
	targetID := assignedUser.TargetID
	if overrideTarget != nil {
		targetID = fmt.Sprint(*overrideTarget)
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
		eventType := getEventTypeFromTemplate(template)
		notifications = append(notifications, &nModel.Notification{
			ChoreID:      chore.ID,
			IsSent:       false,
			ScheduledFor: scheduledTime,
			CreatedAt:    time.Now().UTC(),
			TypeID:       assignedUser.NotificationType,
			UserID:       assignedUser.UserID,
			CircleID:     assignedUser.CircleID,
			TargetID:     targetID,
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
