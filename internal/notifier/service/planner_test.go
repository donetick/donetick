package service

import (
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
)

func member(userID int, active bool, platform nModel.NotificationPlatform, targetID string) *cModel.UserCircleDetail {
	return &cModel.UserCircleDetail{
		UserCircle: cModel.UserCircle{
			UserID:   userID,
			CircleID: 5,
			IsActive: active,
		},
		Username:         "user",
		DisplayName:      "User",
		NotificationType: platform,
		TargetID:         targetID,
	}
}

// choreDueTomorrow builds a chore with a single due-date template. The due date must be in
// the future or generateNotificationsFromTemplate skips the template as already passed.
func choreDueTomorrow() *chModel.Chore {
	dueDate := time.Now().UTC().Add(24 * time.Hour)
	return &chModel.Chore{
		ID:           7,
		Name:         "Take out trash",
		CircleID:     5,
		Notification: true,
		NextDueDate:  &dueDate,
		NotificationMetadataV2: &chModel.NotificationMetadata{
			Templates: []*chModel.NotificationTemplate{
				{Value: 0, Unit: chModel.NotificationTemplateUnitDay},
			},
		},
	}
}

func TestNotifiableMembersFiltersUndeliverableRecipients(t *testing.T) {
	// Only members who are approved and have a delivery target should receive rows.
	eligible := member(1, true, nModel.NotificationPlatformPushover, "pushover-key")
	pendingJoinRequest := member(2, false, nModel.NotificationPlatformPushover, "pushover-key")
	noPlatform := member(3, true, nModel.NotificationPlatformNone, "some-target")
	noTarget := member(4, true, nModel.NotificationPlatformTelegram, "")

	got := notifiableMembers([]*cModel.UserCircleDetail{
		eligible, pendingJoinRequest, noPlatform, noTarget,
	})

	if len(got) != 1 {
		t.Fatalf("expected 1 notifiable member, got %d", len(got))
	}
	if got[0].UserID != eligible.UserID {
		t.Errorf("expected user %d, got %d", eligible.UserID, got[0].UserID)
	}
}

func TestNotifiableMembersEmptyCircle(t *testing.T) {
	if got := notifiableMembers(nil); len(got) != 0 {
		t.Errorf("expected no members, got %d", len(got))
	}
}

func TestGenerateNotificationsFromTemplateUnassignedTargetsRecipient(t *testing.T) {
	// For an "Anyone" chore there is no assigned user, so the notification must be
	// addressed to the recipient and must not claim to be assigned to anyone.
	recipient := member(3, true, nModel.NotificationPlatformPushover, "recipient-key")

	notifications := generateNotificationsFromTemplate(choreDueTomorrow(), recipient, nil, nil)

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	got := notifications[0]

	if got.UserID != recipient.UserID {
		t.Errorf("expected UserID %d, got %d", recipient.UserID, got.UserID)
	}
	if got.TargetID != recipient.TargetID {
		t.Errorf("expected TargetID %q, got %q", recipient.TargetID, got.TargetID)
	}
	if got.TypeID != recipient.NotificationType {
		t.Errorf("expected TypeID %d, got %d", recipient.NotificationType, got.TypeID)
	}
	if got.CircleID != recipient.CircleID {
		t.Errorf("expected CircleID %d, got %d", recipient.CircleID, got.CircleID)
	}
	if got.Text != "📅 Reminder: *Take out trash* is due today and can be completed by anyone." {
		t.Errorf("unexpected unassigned text: %q", got.Text)
	}
	// Webhook consumers read these keys, so they must still be present, just empty.
	if got.RawEvent["assignee"] != "" {
		t.Errorf("expected empty assignee, got %v", got.RawEvent["assignee"])
	}
	if got.RawEvent["assignee_username"] != "" {
		t.Errorf("expected empty assignee_username, got %v", got.RawEvent["assignee_username"])
	}
}

func TestGenerateNotificationsFromTemplateAssignedIsUnchanged(t *testing.T) {
	// The assigned case must keep its existing wording and routing.
	assignedUser := member(3, true, nModel.NotificationPlatformPushover, "recipient-key")
	assignedUser.DisplayName = "Alice"
	assignedUser.Username = "alice"

	notifications := generateNotificationsFromTemplate(choreDueTomorrow(), assignedUser, assignedUser, nil)

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	got := notifications[0]

	if got.Text != "📅 Reminder: *Take out trash* is due today and assigned to Alice." {
		t.Errorf("unexpected assigned text: %q", got.Text)
	}
	if got.RawEvent["assignee"] != "Alice" {
		t.Errorf("expected assignee Alice, got %v", got.RawEvent["assignee"])
	}
	if got.RawEvent["assignee_username"] != "alice" {
		t.Errorf("expected assignee_username alice, got %v", got.RawEvent["assignee_username"])
	}
}

func TestGenerateNotificationsFromTemplateOverrideTargetWins(t *testing.T) {
	// The CircleGroup path overrides the recipient's own target with the group chat ID.
	assignedUser := member(3, true, nModel.NotificationPlatformTelegram, "personal-chat")
	groupID := int64(-100123)

	notifications := generateNotificationsFromTemplate(choreDueTomorrow(), assignedUser, assignedUser, &groupID)

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].TargetID != "-100123" {
		t.Errorf("expected group target -100123, got %q", notifications[0].TargetID)
	}
}

func TestGenerateNotificationsFromTemplateFanOutPerMember(t *testing.T) {
	// End result of the fix: one notification per eligible member, each to their own target.
	chore := choreDueTomorrow()
	members := notifiableMembers([]*cModel.UserCircleDetail{
		member(1, true, nModel.NotificationPlatformPushover, "key-1"),
		member(2, true, nModel.NotificationPlatformTelegram, "chat-2"),
		member(3, false, nModel.NotificationPlatformPushover, "key-3"), // pending, excluded
	})

	notifications := make([]*nModel.Notification, 0)
	for _, m := range members {
		notifications = append(notifications, generateNotificationsFromTemplate(chore, m, nil, nil)...)
	}

	if len(notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifications))
	}
	targets := map[string]bool{}
	for _, n := range notifications {
		targets[n.TargetID] = true
	}
	for _, want := range []string{"key-1", "chat-2"} {
		if !targets[want] {
			t.Errorf("expected a notification targeting %q", want)
		}
	}
}

func TestAnyoneChoreRecipientsSkipsMembersWhoCannotSeeAPrivateChore(t *testing.T) {
	// An "Anyone" chore fans out to the whole circle, so a private one (every chore in a
	// private project is private) must not reach members that can't see it.
	circleMembers := []*cModel.UserCircleDetail{
		member(1, true, nModel.NotificationPlatformPushover, "key-1"),
		member(2, true, nModel.NotificationPlatformTelegram, "chat-2"),
		member(3, true, nModel.NotificationPlatformPushover, "key-3"),
	}

	chore := choreDueTomorrow()
	chore.CreatedBy = 1
	chore.IsPrivate = true
	chore.Assignees = []chModel.ChoreAssignees{{ChoreID: chore.ID, UserID: 2}}

	got := anyoneChoreRecipients(chore, circleMembers)

	recipients := map[int]bool{}
	for _, r := range got {
		recipients[r.UserID] = true
	}
	if len(got) != 2 || !recipients[1] || !recipients[2] {
		t.Errorf("got recipients %v, want the creator (1) and the assignee (2) only", recipients)
	}
}

func TestAnyoneChoreRecipientsKeepsWholeCircleForPublicChores(t *testing.T) {
	circleMembers := []*cModel.UserCircleDetail{
		member(1, true, nModel.NotificationPlatformPushover, "key-1"),
		member(2, true, nModel.NotificationPlatformTelegram, "chat-2"),
		member(3, false, nModel.NotificationPlatformPushover, "key-3"), // pending, excluded
	}

	chore := choreDueTomorrow()
	chore.CreatedBy = 1

	if got := anyoneChoreRecipients(chore, circleMembers); len(got) != 2 {
		t.Errorf("expected 2 recipients for a public chore, got %d", len(got))
	}
}
