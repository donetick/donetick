package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type NotificationPlanner struct {
	nRepo      *nRepo.NotificationRepository
	cRepo      *cRepo.CircleRepository
	uRepo      *uRepo.UserRepository
	deviceRepo *dRepo.DeviceRepository
	// Bounded, per-task serialization for this planner (shared by all handlers).
	planningLocks [64]sync.Mutex
}

func NewNotificationPlanner(nr *nRepo.NotificationRepository, cr *cRepo.CircleRepository, ur *uRepo.UserRepository, dr *dRepo.DeviceRepository) *NotificationPlanner {
	return &NotificationPlanner{nRepo: nr,
		cRepo:      cr,
		uRepo:      ur,
		deviceRepo: dr,
	}
}

// GenerateChoreNotifications plans reminders and assignment events as one ordered
// operation. Never split these across jobs: an older job could otherwise delete
// the newer assignment or insert a notification for a superseded assignee.
func (n *NotificationPlanner) GenerateChoreNotifications(c context.Context, chore *chModel.Chore, previousAssignee *int) bool {
	return n.planCurrentChore(c, chore, func() bool {
		return n.generateNotifications(c, chore) && n.generateAssignmentNotification(c, chore, previousAssignee)
	})
}

func (n *NotificationPlanner) GenerateNotifications(c context.Context, chore *chModel.Chore) bool {
	return n.planCurrentChore(c, chore, func() bool { return n.generateNotifications(c, chore) })
}

func (n *NotificationPlanner) planCurrentChore(c context.Context, chore *chModel.Chore, plan func() bool) bool {
	lock := &n.planningLocks[uint(chore.ID)%uint(len(n.planningLocks))]
	lock.Lock()
	defer lock.Unlock()

	// Check after acquiring the lock, since another job may have planned a newer
	// revision while this one was waiting. If a mutation happens during planning,
	// its job must wait for this lock and will replace this plan afterwards.
	current, err := n.nRepo.IsCurrentChore(c, chore)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to check chore notification revision", "error", err, "chore_id", chore.ID)
		return false
	}
	if !current {
		logging.FromContext(c).Debugw("Skipping stale chore notification plan", "chore_id", chore.ID)
		return true
	}
	return plan()
}

func (n *NotificationPlanner) generateNotifications(c context.Context, chore *chModel.Chore) bool {
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

	// Preserve a pending assignment across ordinary edits, but not after the task
	// is completed/deactivated or the recipient loses access or membership.
	var assignmentRecipient *int
	if chore.IsActive && assignedUser != nil && assignedUser.IsActive && CanReceiveChore(chore, assignedUser.UserID, circleMembers) {
		assignmentRecipient = chore.AssignedTo
	}
	var approvalRecipients []int
	if chore.Status == chModel.ChoreStatusPendingApproval {
		for _, member := range circleMembers {
			if member.IsActive && member.IsManagerOrAdmin() && CanReceiveChore(chore, member.UserID, circleMembers) {
				approvalRecipients = append(approvalRecipients, member.UserID)
			}
		}
	}
	if err := n.nRepo.DeleteChoreReminders(c, chore.ID, assignmentRecipient, approvalRecipients); err != nil {
		log.Error("Error clearing chore reminders", "error", err)
		return false
	}
	notifications := make([]*nModel.Notification, 0)
	if !chore.Notification || chore.FrequencyType == "trigger" {

		return true
	}

	if chore.NextDueDate == nil {
		return true
	}

	var directNotifications []*nModel.Notification
	if len(chore.NotificationMetadataV2.Templates) > 0 {
		if assignedUser != nil && CanReceiveChore(chore, assignedUser.UserID, circleMembers) {
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
	if chore.NotificationMetadataV2.CircleGroup && !inRestrictedProject(chore) && assignedUser != nil && CanReceiveChore(chore, assignedUser.UserID, circleMembers) {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, assignedUser, assignedUser, chore.NotificationMetadataV2.CircleGroupID)...)
	}

	log.Debug("Generated notifications", "count", len(notifications))
	n.nRepo.BatchInsertNotifications(notifications)
	return true
}

// generateAssignmentNotification queues an event independently of due-date reminders.
// Production callers use GenerateChoreNotifications to serialize it with reminder cleanup.
// Only the current assignee is notified, and ordinary edits / unchanged rotations
// do not produce another event. A nil previous assignee represents task creation.
func (n *NotificationPlanner) generateAssignmentNotification(c context.Context, chore *chModel.Chore, previousAssignee *int) bool {
	log := logging.FromContext(c)
	if chore.AssignedTo == nil || *chore.AssignedTo <= 0 || !chore.IsActive ||
		(previousAssignee != nil && *previousAssignee == *chore.AssignedTo) {
		log.Debugw("Skipping assignment notification: no new active assignment", "chore_id", chore.ID)
		return true
	}
	members, err := n.cRepo.GetCircleUsers(c, chore.CircleID)
	if err != nil {
		logging.FromContext(c).Error("Error loading assignment recipient", "error", err)
		return false
	}
	for _, recipient := range members {
		if recipient.UserID != *chore.AssignedTo || !recipient.IsActive || !CanReceiveChore(chore, recipient.UserID, members) {
			continue
		}
		pref, err := n.uRepo.GetNotificationPreference(c, recipient.UserID)
		if err != nil {
			log.Errorw("Error loading assignment preference", "error", err, "user_id", recipient.UserID)
			return false
		}
		if !pref.Enabled(uModel.NotificationEventOnAssignment) {
			log.Debugw("Skipping assignment notification: muted by preference", "chore_id", chore.ID, "user_id", recipient.UserID)
			return true
		}
		tokens, err := n.deviceRepo.GetActiveDeviceTokens(c, recipient.UserID)
		if err != nil {
			log.Errorw("Error loading assignment push devices", "error", err, "user_id", recipient.UserID)
			return false
		}
		now := time.Now().UTC()
		text := fmt.Sprintf("📋 Assigned to you: %s", chore.Name)
		base := nModel.Notification{
			ChoreID: chore.ID, CircleID: chore.CircleID, UserID: recipient.UserID,
			TypeID: recipient.NotificationType, TargetID: recipient.TargetID,
			ScheduledFor: now, CreatedAt: now, Text: text,
			RawEvent: map[string]interface{}{
				"id": chore.ID, "type": EventTypeOnAssignment, "name": chore.Name,
				"due_date": chore.NextDueDate, "assignee": recipient.DisplayName,
				"assignee_username": recipient.Username,
				"payload": map[string]interface{}{
					"title": "Task assigned to you", "body": chore.Name,
					"data": map[string]string{"choreId": fmt.Sprint(chore.ID), "type": string(EventTypeOnAssignment)},
				},
			},
		}
		// Mobile registrations are independent of the legacy notification target.
		// Queue one row per active device, as digests do, for FCM/Bridge delivery.
		notifications := make([]*nModel.Notification, 0, len(tokens)+1)
		seen := make(map[string]bool, len(tokens))
		for _, token := range tokens {
			if token.Token == "" || seen[token.Token] {
				continue
			}
			seen[token.Token] = true
			push := base
			push.TypeID = nModel.NotificationPlatformFCM
			push.TargetID = token.Token
			notifications = append(notifications, &push)
		}
		// Preserve third-party channels. A legacy FCM target is only a fallback
		// when there are no registered devices, never a second push to an old token.
		if recipient.NotificationType != nModel.NotificationPlatformNone && recipient.TargetID != "" &&
			(recipient.NotificationType != nModel.NotificationPlatformFCM || len(notifications) == 0) {
			legacy := base
			notifications = append(notifications, &legacy)
		}
		if len(notifications) == 0 {
			log.Infow("Skipping assignment notification: no delivery target or active push devices", "chore_id", chore.ID, "user_id", recipient.UserID)
			return true
		}
		if err := n.nRepo.BatchInsertNotifications(notifications); err != nil {
			logging.FromContext(c).Error("Error queuing assignment notification", "error", err)
			return false
		}
		log.Infow("Queued assignment notifications", "chore_id", chore.ID, "user_id", recipient.UserID, "count", len(notifications))
		return true
	}
	log.Debugw("Skipping assignment notification: assignee is not an active member or cannot view task", "chore_id", chore.ID, "user_id", *chore.AssignedTo)
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

// CanReceiveChore reports whether userID may be told about chore -- the single
// visibility rule for anything that puts a chore's name in front of somebody.
// Stricter than Chore.CanView: a chore whose project was not loaded fails
// closed, since project privacy cannot be evaluated without it.
func CanReceiveChore(chore *chModel.Chore, userID int, circleMembers []*cModel.UserCircleDetail) bool {
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

// anyoneChoreRecipients returns the circle members to remind about a chore assigned to
// "Anyone": those that can actually receive a notification and are allowed to see the
// chore. Without the visibility check a private chore with no assignee would leak its
// name to the whole circle, which project-level privacy makes easy to hit (every chore
// in a private project is private).
func anyoneChoreRecipients(chore *chModel.Chore, circleMembers []*cModel.UserCircleDetail) []*cModel.UserCircleDetail {
	members := notifiableMembers(circleMembers)
	recipients := make([]*cModel.UserCircleDetail, 0, len(members))
	for _, member := range members {
		if !CanReceiveChore(chore, member.UserID, circleMembers) {
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
		if AllowedByPreference(prefs[notif.UserID], string(eventType)) {
			filtered = append(filtered, notif)
		}
	}
	return filtered
}

// AllowedByPreference resolves whether a single notification should survive the
// recipient's notification profile.
//
// eventType is the raw string from Notification.RawEvent["type"] rather than a
// typed value: the scheduler re-checks this at send time, where the event has
// been through a JSON round-trip and typed constants no longer survive. An
// unrecognized type is allowed through, so a notification kind that predates
// preferences is never silently dropped.
func AllowedByPreference(pref *uModel.UserNotificationPreference, eventType string) bool {
	switch eventType {
	case string(EventTypeOnAssignment):
		return pref.Enabled(uModel.NotificationEventOnAssignment)
	case string(EventTypeOnPendingApproval):
		return pref.Enabled(uModel.NotificationEventOnPendingApproval)
	case string(EventTypeOnRedemptionRequest):
		return pref.Enabled(uModel.NotificationEventOnRedemptionRequest)
	case string(EventTypePreDue):
		return pref.Enabled(uModel.NotificationEventPreDue)
	case string(EventTypeDue):
		return pref.Enabled(uModel.NotificationEventDue)
	case string(EventTypeOverdue):
		return pref.Enabled(uModel.NotificationEventOverdueFollowup)
	case string(uModel.NotificationEventDailyDigest):
		return pref.Enabled(uModel.NotificationEventDailyDigest)
	case string(uModel.NotificationEventWeeklyDigest):
		return pref.Enabled(uModel.NotificationEventWeeklyDigest)
	default:
		return true
	}
}

type EventType string

const (
	EventTypeOnAssignment        EventType = "on_assignment"
	EventTypeOnPendingApproval   EventType = "on_pending_approval"
	EventTypeOnRedemptionRequest EventType = "on_redemption_request"
	EventTypeUnknown             EventType = "unknown"
	EventTypeDue                 EventType = "due"
	EventTypePreDue              EventType = "pre_due"
	EventTypeOverdue             EventType = "overdue"
)
