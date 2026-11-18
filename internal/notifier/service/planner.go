package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	dRepo "donetick.com/core/internal/device/repo"
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
	if err != nil {
		log.Error("Error getting circle members", err)
		return false
	}

	var assignedUser *cModel.UserCircleDetail
	for _, member := range circleMembers {
		if chore.AssignedTo != nil && *chore.AssignedTo == member.UserID {
			assignedUser = member
			break
		}
	}

	n.nRepo.DeleteChoreReminders(chore.ID)
	notifications := make([]*nModel.Notification, 0)
	if !chore.Notification || chore.NotificationMetadataV2 == nil || chore.FrequencyType == "trigger" {

		return true
	}

	if chore.NextDueDate == nil {
		return true
	}

	if len(chore.NotificationMetadataV2.Templates) > 0 {
		if assignedUser != nil && canReceiveChore(chore, assignedUser.UserID, circleMembers) {
			notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, assignedUser, nil)...)
		} else if assignedUser == nil {
			// A chore assigned to "Anyone" has no assigned user to resolve, which previously
			// meant no notifications were generated at all. Remind every circle member who
			// can actually receive one instead.
			for _, member := range anyoneChoreRecipients(chore, circleMembers) {
				notifications = append(notifications, generateNotificationsFromTemplate(chore, member, nil, nil)...)
			}
		}
	}

	if chore.NotificationMetadataV2.CircleGroup && assignedUser != nil && canReceiveChore(chore, assignedUser.UserID, circleMembers) {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, assignedUser, chore.NotificationMetadataV2.CircleGroupID)...)
	}

	log.Debug("Generated notifications", "count", len(notifications))
	n.nRepo.BatchInsertNotifications(notifications)
	return true
}

// OverdueNaggingDestinations resolves the current delivery targets, including
// mobile devices that receive push instead of scheduling an offline repeat.
func (n *NotificationPlanner) OverdueNaggingDestinations(c context.Context, chore *chModel.Chore, devices *dRepo.DeviceRepository, pushEnabled bool) ([]nModel.Notification, error) {
	members, err := n.cRepo.GetCircleUsers(c, chore.CircleID)
	if err != nil {
		return nil, err
	}
	type destination struct {
		platform nModel.NotificationPlatform
		target   string
	}
	seen := make(map[destination]bool)
	var notifications []nModel.Notification
	add := func(platform nModel.NotificationPlatform, target string, userID int) {
		key := destination{platform, target}
		if seen[key] {
			return
		}
		seen[key] = true
		notifications = append(notifications, nModel.Notification{
			ChoreID: chore.ID, CircleID: chore.CircleID, UserID: userID, TypeID: platform, TargetID: target,
		})
	}
	checkedDevices := make(map[int]bool)
	for _, member := range members {
		if !member.IsActive || (chore.AssignedTo != nil && *chore.AssignedTo != 0 && *chore.AssignedTo != member.UserID) || !canReceiveChore(chore, member.UserID, members) {
			continue
		}
		if member.TargetID != "" && member.NotificationType != nModel.NotificationPlatformNone && member.NotificationType != nModel.NotificationPlatformWebhook {
			add(member.NotificationType, member.TargetID, member.UserID)
		}
		if pushEnabled && !checkedDevices[member.UserID] {
			checkedDevices[member.UserID] = true
			tokens, err := devices.GetActiveDeviceTokens(c, member.UserID)
			if err != nil {
				return nil, err
			}
			for _, token := range tokens {
				if token.Token != "" {
					add(nModel.NotificationPlatformFCM, token.Token, member.UserID)
				}
			}
		}
	}
	if !chore.IsPrivate && (chore.ProjectID == nil || (chore.Project != nil && !chore.Project.IsPrivate)) {
		circle, err := n.cRepo.GetCircleByID(c, chore.CircleID)
		if err != nil {
			return nil, err
		}
		if circle.WebhookURL != nil && *circle.WebhookURL != "" {
			add(nModel.NotificationPlatformWebhook, *circle.WebhookURL, 0)
		}
	}
	return notifications, nil
}

func PopulateOverdueNaggingDelivery(notification *nModel.Notification, chore *chModel.Chore) {
	notification.Text = fmt.Sprintf("%s is overdue.", chore.Name)
	notification.RawEvent = nModel.JSONB{
		"id": chore.ID, "type": EventTypeOverdueNagging, "name": chore.Name,
		"due_date": chore.NextDueDate, "scheduled_for": notification.ScheduledFor,
	}
	if notification.TypeID == nModel.NotificationPlatformFCM {
		notification.RawEvent["payload"] = map[string]any{
			"title": "Overdue task", "body": notification.Text,
			"data": map[string]string{"type": string(EventTypeOverdueNagging), "chore_id": strconv.Itoa(chore.ID)},
		}
	}
}

// notifiableMembers returns the circle members that can actually receive a notification.
// Pending join requests (is_active=false, see CircleRepository.GetPendingJoinRequests) are
// not members yet, and members without a delivery target would be dropped by the senders,
// so neither should have notification rows created for them.
func notifiableMembers(circleMembers []*cModel.UserCircleDetail) []*cModel.UserCircleDetail {
	members := make([]*cModel.UserCircleDetail, 0, len(circleMembers))
	for _, member := range circleMembers {
		if !member.IsActive {
			continue
		}
		if member.NotificationType == nModel.NotificationPlatformNone || member.TargetID == "" {
			continue
		}
		members = append(members, member)
	}
	return members
}

// anyoneChoreRecipients returns the circle members to remind about a chore assigned to
// "Anyone": those that can actually receive a notification and are allowed to see the
// chore. Without the visibility check a private chore with no assignee would leak its
// name to the whole circle, which project-level privacy makes easy to hit (every chore
// in a private project is private).
func canReceiveChore(chore *chModel.Chore, userID int, circleMembers []*cModel.UserCircleDetail) bool {
	if chore.ProjectID != nil && chore.Project == nil {
		return false
	}
	if chore.Project != nil && chore.Project.IsPrivate {
		return chore.Project.CreatedBy == userID
	}
	return chore.CanView(userID, circleMembers)
}

func anyoneChoreRecipients(chore *chModel.Chore, circleMembers []*cModel.UserCircleDetail) []*cModel.UserCircleDetail {
	members := notifiableMembers(circleMembers)
	recipients := make([]*cModel.UserCircleDetail, 0, len(members))
	for _, member := range members {
		if !canReceiveChore(chore, member.UserID, circleMembers) {
			continue
		}
		recipients = append(recipients, member)
	}
	return recipients
}

func getEventTypeFromTemplate(template *chModel.NotificationTemplate) EventType {
	switch {
	case template == nil:
		return EventTypeUnknown
	case template.Value < 0:
		return EventTypePreDue
	case template.Value == 0:
		return EventTypeDue
	default:
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

// generateNotificationsFromTemplate builds the scheduled notifications for one recipient.
// assignedUser is who the chore is assigned to and is only used for the message wording; it
// is nil for a chore assigned to "Anyone", where recipient is each circle member in turn.
func generateNotificationsFromTemplate(chore *chModel.Chore, recipient *cModel.UserCircleDetail, assignedUser *cModel.UserCircleDetail, overrideTarget *int64) []*nModel.Notification {
	if len(chore.NotificationMetadataV2.Templates) == 0 {
		return nil // No templates to process
	}
	targetID := recipient.TargetID
	if overrideTarget != nil {
		targetID = fmt.Sprint(*overrideTarget)
	}

	text := fmt.Sprintf("📅 Reminder: *%s* is due today and can be completed by anyone.", chore.Name)
	var assigneeName, assigneeUsername string
	if assignedUser != nil {
		assigneeName = assignedUser.DisplayName
		assigneeUsername = assignedUser.Username
		text = fmt.Sprintf("📅 Reminder: *%s* is due today and assigned to %s.", chore.Name, assignedUser.DisplayName)
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
			TypeID:       recipient.NotificationType,
			UserID:       recipient.UserID,
			CircleID:     recipient.CircleID,
			TargetID:     targetID,
			Text:         text,
			RawEvent: map[string]interface{}{
				"id":                chore.ID,
				"type":              eventType,
				"name":              chore.Name,
				"due_date":          chore.NextDueDate,
				"assignee":          assigneeName,
				"assignee_username": assigneeUsername,
			},
		})

	}

	return notifications
}

type EventType string

const (
	EventTypeUnknown        EventType = "unknown"
	EventTypeDue            EventType = "due"
	EventTypePreDue         EventType = "pre_due"
	EventTypeOverdue        EventType = "overdue"
	EventTypeOverdueNagging EventType = "overdue_nagging"
)
