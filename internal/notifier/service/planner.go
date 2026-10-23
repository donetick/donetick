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
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type NotificationPlanner struct {
	nRepo *nRepo.NotificationRepository
	cRepo *cRepo.CircleRepository
	uRepo *uRepo.UserRepository
}

func NewNotificationPlanner(nr *nRepo.NotificationRepository, cr *cRepo.CircleRepository, ur *uRepo.UserRepository) *NotificationPlanner {
	return &NotificationPlanner{nRepo: nr,
		cRepo: cr,
		uRepo: ur,
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

	n.nRepo.DeleteAllChoreNotifications(chore.ID)
	notifications := make([]*nModel.Notification, 0)
	if !chore.Notification || chore.FrequencyType == "trigger" {

		return true
	}

	if chore.NextDueDate == nil {
		return true
	}

	var directNotifications []*nModel.Notification
	if len(chore.NotificationMetadataV2.Templates) > 0 {
		if assignedUser != nil && canReceiveChore(chore, assignedUser.UserID, circleMembers) {
			directNotifications = append(directNotifications, generateNotificationsFromTemplate(chore, assignedUser, assignedUser, nil)...)
		} else if assignedUser == nil {
			// A chore assigned to "Anyone" has no assigned user to resolve, which previously
			// meant no notifications were generated at all. Remind every circle member who
			// can actually receive one instead.
			for _, member := range anyoneChoreRecipients(chore, circleMembers) {
				directNotifications = append(directNotifications, generateNotificationsFromTemplate(chore, member, nil, nil)...)
			}
		}
	}
	// Gate individually-targeted reminders by each recipient's own notification
	// preference. The circle group chat message below is excluded from this: it's a
	// shared message other circle members in that chat may still want, so it
	// shouldn't be suppressed by the assigned user's personal profile.
	notifications = append(notifications, n.filterByPreference(c, chore, directNotifications)...)

	// A circle group chat reaches everybody in it, so chores of a restricted project
	// (only me / selected members) are never posted there.
	if chore.NotificationMetadataV2.CircleGroup && !inRestrictedProject(chore) && assignedUser != nil && canReceiveChore(chore, assignedUser.UserID, circleMembers) {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, assignedUser, chore.NotificationMetadataV2.CircleGroupID)...)
	}

	log.Debug("Generated notifications", "count", len(notifications))
	n.nRepo.BatchInsertNotifications(notifications)
	return true
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
		// Owner, plus members of a shared project once its members are loaded.
		return chore.Project.CanView(userID)
	}
	return chore.CanView(userID, circleMembers)
}

// inRestrictedProject reports whether the chore lives in a project hidden from the
// circle. An unloaded project counts as restricted, failing closed.
func inRestrictedProject(chore *chModel.Chore) bool {
	if chore.ProjectID != nil && chore.Project == nil {
		return true
	}
	return chore.Project != nil && chore.Project.IsPrivate
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

// filterByPreference drops notifications the recipient's notification profile has
// turned off. Fails open (returns notifications unfiltered) if preferences can't be
// loaded, since silently dropping reminders is worse than a profile being briefly
// ignored.
func (n *NotificationPlanner) filterByPreference(c context.Context, chore *chModel.Chore, notifications []*nModel.Notification) []*nModel.Notification {
	if len(notifications) == 0 {
		return notifications
	}

	seen := make(map[int]bool, len(notifications))
	userIDs := make([]int, 0, len(notifications))
	for _, notif := range notifications {
		if !seen[notif.UserID] {
			seen[notif.UserID] = true
			userIDs = append(userIDs, notif.UserID)
		}
	}

	prefs, err := n.uRepo.GetNotificationPreferences(c, userIDs)
	if err != nil {
		logging.FromContext(c).Error("Error loading notification preferences, skipping preference gating", "err", err)
		return notifications
	}

	filtered := make([]*nModel.Notification, 0, len(notifications))
	for _, notif := range notifications {
		eventType, _ := notif.RawEvent["type"].(EventType)
		if allowedByPreference(prefs[notif.UserID], eventType, chore) {
			filtered = append(filtered, notif)
		}
	}
	return filtered
}

// allowedByPreference resolves whether a single notification should survive the
// recipient's notification profile. EventTypeOverdue covers both "overdue follow-up"
// (any priority) and "escalation" (high/urgent priority only, for profiles like Quiet
// that otherwise mute due/overdue reminders).
func allowedByPreference(pref *uModel.UserNotificationPreference, eventType EventType, chore *chModel.Chore) bool {
	switch eventType {
	case EventTypePreDue:
		return pref.Enabled(uModel.NotificationEventPreDue)
	case EventTypeDue:
		return pref.Enabled(uModel.NotificationEventDue)
	case EventTypeOverdue:
		if pref.Enabled(uModel.NotificationEventOverdueFollowup) {
			return true
		}
		return pref.Enabled(uModel.NotificationEventEscalation) && chore.Priority >= chModel.PriorityHigh
	default:
		return true
	}
}

type EventType string

const (
	EventTypeUnknown EventType = "unknown"
	EventTypeDue     EventType = "due"
	EventTypePreDue  EventType = "pre_due"
	EventTypeOverdue EventType = "overdue"
)
