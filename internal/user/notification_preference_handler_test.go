package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newNotificationPreferenceHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.UserNotificationPreference{}))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return &Handler{userRepo: uRepo.NewUserRepository(db, cfg)}, db
}

// putNotificationPreference drives the handler the way the route does, with an
// authenticated user already in the context.
func putNotificationPreference(t *testing.T, h *Handler, userID int, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/users/notifications/preferences", func(c *gin.Context) {
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: userID}})
		h.UpdateNotificationPreference(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/notifications/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The bug this guards: clients render the resolved "effective" map and post it
// back as overrides. Since an explicit override beats the profile default, a
// dense map pins the old posture and makes picking a new profile a no-op.
func TestUpdateNotificationPreferenceProfileSwitchDiscardsCarriedOverOverrides(t *testing.T) {
	h, _ := newNotificationPreferenceHandler(t)
	c := t.Context()

	// Start on Balanced, then switch to Quiet while echoing back Balanced's
	// resolved set as overrides.
	require.NoError(t, h.userRepo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  3,
		Profile: uModel.NotificationProfileBalanced,
	}))

	w := putNotificationPreference(t, h, 3, `{
		"profile": "quiet",
		"overrides": {"pre_due": true, "due": true, "overdue_followup": true, "escalation": false, "daily_digest": true, "weekly_digest": false}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	saved, err := h.userRepo.GetNotificationPreference(c, 3)
	require.NoError(t, err)
	if saved.Profile != uModel.NotificationProfileQuiet {
		t.Fatalf("expected the quiet profile to be saved, got %q", saved.Profile)
	}
	if len(saved.Overrides) != 0 {
		t.Errorf("expected overrides from the previous profile to be discarded, got %v", saved.Overrides)
	}
	for _, eventType := range []uModel.NotificationEventType{
		uModel.NotificationEventDue,
		uModel.NotificationEventOverdueFollowup,
		uModel.NotificationEventDailyDigest,
	} {
		if saved.Enabled(eventType) {
			t.Errorf("expected %q to be muted by the quiet profile", eventType)
		}
	}
}

// Toggling one event type without changing profile is a real override and has
// to survive -- only entries matching the profile default are dropped.
func TestUpdateNotificationPreferenceKeepsGenuineOverridesWithinAProfile(t *testing.T) {
	h, _ := newNotificationPreferenceHandler(t)
	c := t.Context()

	require.NoError(t, h.userRepo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  4,
		Profile: uModel.NotificationProfileBalanced,
	}))

	w := putNotificationPreference(t, h, 4, `{
		"profile": "balanced",
		"overrides": {"pre_due": true, "due": true, "weekly_digest": true}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	saved, err := h.userRepo.GetNotificationPreference(c, 4)
	require.NoError(t, err)
	if len(saved.Overrides) != 1 || !saved.Overrides[uModel.NotificationEventWeeklyDigest] {
		t.Errorf("expected only the weekly digest override to be kept, got %v", saved.Overrides)
	}
	if !saved.Enabled(uModel.NotificationEventWeeklyDigest) {
		t.Error("expected the weekly digest to be enabled by the override")
	}
}

// Custom is nothing but overrides, so switching to it must keep them.
func TestUpdateNotificationPreferenceCustomKeepsOverrides(t *testing.T) {
	h, _ := newNotificationPreferenceHandler(t)
	c := t.Context()

	require.NoError(t, h.userRepo.UpsertNotificationPreference(c, &uModel.UserNotificationPreference{
		UserID:  5,
		Profile: uModel.NotificationProfileBalanced,
	}))

	w := putNotificationPreference(t, h, 5, `{
		"profile": "custom",
		"overrides": {"due": true, "escalation": true}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	saved, err := h.userRepo.GetNotificationPreference(c, 5)
	require.NoError(t, err)
	if !saved.Enabled(uModel.NotificationEventDue) || !saved.Enabled(uModel.NotificationEventEscalation) {
		t.Errorf("expected custom overrides to be kept, got %v", saved.Overrides)
	}
	if saved.Enabled(uModel.NotificationEventDailyDigest) {
		t.Error("expected anything unset under custom to default to off")
	}
}

// The response has to describe what was saved, not what was posted, or the
// client re-renders the toggles it sent and the switch looks like it failed.
func TestUpdateNotificationPreferenceResponseReflectsSavedPreference(t *testing.T) {
	h, _ := newNotificationPreferenceHandler(t)

	w := putNotificationPreference(t, h, 6, `{
		"profile": "quiet",
		"overrides": {"due": true, "daily_digest": true}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Profile   string          `json:"profile"`
		Effective map[string]bool `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	if resp.Profile != string(uModel.NotificationProfileQuiet) {
		t.Errorf("expected the quiet profile in the response, got %q", resp.Profile)
	}
	if resp.Effective["due"] || resp.Effective["daily_digest"] {
		t.Errorf("expected the response to report the quiet profile's resolved set, got %v", resp.Effective)
	}
	if !resp.Effective["escalation"] {
		t.Errorf("expected quiet to keep escalation on, got %v", resp.Effective)
	}
}
