package user

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/database"
	nModel "donetick.com/core/internal/notifier/model"
)

func newNotificationTestRepo(t *testing.T) (*NotificationRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notifications.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migration(db))
	return NewNotificationRepository(db), db
}

// TestGetPendingNotificationSuppressesWebhookForDigests checks that a digest
// notification (negative, sentinel chore_id -- it isn't about one real chore)
// never gets a webhook_url, even when its circle has one configured, while an
// ordinary per-chore reminder in the same circle still does. Digests are a
// personal push summary, not a per-chore event a webhook consumer expects.
func TestGetPendingNotificationSuppressesWebhookForDigests(t *testing.T) {
	repo, db := newNotificationTestRepo(t)
	c := context.Background()

	webhookURL := "https://example.com/webhook"
	circle := &cModel.Circle{Name: "Test Circle", WebhookURL: &webhookURL}
	require.NoError(t, db.Create(circle).Error)

	userID := 1
	chore := &chModel.Chore{Name: "Real chore", CircleID: circle.ID, AssignedTo: &userID, IsActive: true}
	require.NoError(t, db.Create(chore).Error)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&nModel.Notification{
		ChoreID: chore.ID, CircleID: circle.ID, UserID: userID,
		IsSent: false, ScheduledFor: now, CreatedAt: now, Text: "chore reminder",
	}).Error)
	require.NoError(t, db.Create(&nModel.Notification{
		ChoreID: -1, CircleID: circle.ID, UserID: userID,
		IsSent: false, ScheduledFor: now, CreatedAt: now, Text: "daily digest",
	}).Error)

	pending, err := repo.GetPendingNotification(c, time.Hour)
	require.NoError(t, err)

	byChoreID := map[int]*nModel.NotificationDetails{}
	for _, n := range pending {
		byChoreID[n.ChoreID] = n
	}

	choreReminder, ok := byChoreID[chore.ID]
	if !ok {
		t.Fatalf("expected the chore reminder to be in the pending set")
	}
	if choreReminder.WebhookURL == nil || *choreReminder.WebhookURL != webhookURL {
		t.Errorf("expected the chore reminder to carry the circle's webhook_url, got %v", choreReminder.WebhookURL)
	}

	digest, ok := byChoreID[-1]
	if !ok {
		t.Fatalf("expected the digest notification to be in the pending set")
	}
	if digest.WebhookURL != nil {
		t.Errorf("expected the digest notification's webhook_url to be suppressed, got %v", *digest.WebhookURL)
	}
}

// The scheduler re-checks notification preferences at send time, and the
// escalation rule is priority-gated, so pending rows have to carry the
// priority of the chore they're about. Digests aren't about one chore and get
// 0 rather than a NULL that would fail the scan.
func TestGetPendingNotificationIncludesChorePriority(t *testing.T) {
	repo, db := newNotificationTestRepo(t)
	c := context.Background()

	circle := &cModel.Circle{Name: "Test Circle"}
	require.NoError(t, db.Create(circle).Error)

	userID := 1
	chore := &chModel.Chore{
		Name: "Urgent chore", CircleID: circle.ID, AssignedTo: &userID,
		IsActive: true, Priority: chModel.PriorityUrgent,
	}
	require.NoError(t, db.Create(chore).Error)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&nModel.Notification{
		ChoreID: chore.ID, CircleID: circle.ID, UserID: userID,
		IsSent: false, ScheduledFor: now, CreatedAt: now, Text: "chore reminder",
	}).Error)
	require.NoError(t, db.Create(&nModel.Notification{
		ChoreID: -1, CircleID: circle.ID, UserID: userID,
		IsSent: false, ScheduledFor: now, CreatedAt: now, Text: "daily digest",
	}).Error)

	pending, err := repo.GetPendingNotification(c, time.Hour)
	require.NoError(t, err)

	byChoreID := map[int]*nModel.NotificationDetails{}
	for _, n := range pending {
		byChoreID[n.ChoreID] = n
	}

	if got := byChoreID[chore.ID]; got == nil || got.ChorePriority != chModel.PriorityUrgent {
		t.Errorf("expected the reminder to carry priority %d, got %v", chModel.PriorityUrgent, got)
	}
	if got := byChoreID[-1]; got == nil || got.ChorePriority != 0 {
		t.Errorf("expected the digest to carry priority 0, got %v", got)
	}
}
