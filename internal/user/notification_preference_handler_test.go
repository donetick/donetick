package user

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// newHostedNotificationPreferenceHandler builds a handler for a donetick.com
// instance, where the plus gate actually bites: self-hosted instances treat
// every user as plus.
func newHostedNotificationPreferenceHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.UserNotificationPreference{}))

	cfg := &config.Config{IsDoneTickDotCom: true}
	cfg.Database.Type = "sqlite"
	return &Handler{userRepo: uRepo.NewUserRepository(db, cfg)}, db
}

// createHostedUser inserts a user whose subscription has or hasn't lapsed.
// Expiration is a read-only gorm column (it's owned by the billing webhook),
// so it has to be written with raw SQL.
func createHostedUser(t *testing.T, db *gorm.DB, userID int, isPlus bool) {
	t.Helper()
	require.NoError(t, db.Create(&uModel.User{
		ID: userID, Username: fmt.Sprintf("u%d", userID), DisplayName: "U", Timezone: "UTC",
	}).Error)

	expiration := time.Now().UTC().Add(-24 * time.Hour)
	if isPlus {
		expiration = time.Now().UTC().Add(30 * 24 * time.Hour)
	}
	require.NoError(t, db.Exec("UPDATE users SET expiration = ? WHERE id = ?", expiration, userID).Error)
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
		"overrides": {"due": true, "overdue_followup": true}
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	saved, err := h.userRepo.GetNotificationPreference(c, 5)
	require.NoError(t, err)
	if !saved.Enabled(uModel.NotificationEventDue) || !saved.Enabled(uModel.NotificationEventOverdueFollowup) {
		t.Errorf("expected custom overrides to be kept, got %v", saved.Overrides)
	}
	if saved.Enabled(uModel.NotificationEventDailyDigest) {
		t.Error("expected anything unset under custom to default to off")
	}
}

// A free user who asks for a plus-only type gets saved without it rather than
// refused: the request succeeds, and the response/row describe the posture
// that will actually fire.
func TestUpdateNotificationPreferenceDropsPlusOnlyOverridesWithoutSubscription(t *testing.T) {
	for _, eventType := range uModel.PlusOnlyEvents() {
		t.Run(string(eventType), func(t *testing.T) {
			h, db := newHostedNotificationPreferenceHandler(t)
			createHostedUser(t, db, 20, false)

			body, err := json.Marshal(map[string]interface{}{
				"profile":   "custom",
				"overrides": map[string]bool{string(eventType): true},
			})
			require.NoError(t, err)

			w := putNotificationPreference(t, h, 20, string(body))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			var resp struct {
				Profile   string          `json:"profile"`
				Overrides map[string]bool `json:"overrides"`
				Effective map[string]bool `json:"effective"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			// Custom enables nothing by default, so it survives the downgrade
			// -- only the gated override is dropped.
			require.Equal(t, string(uModel.NotificationProfileCustom), resp.Profile)
			require.NotContains(t, resp.Overrides, string(eventType))
			require.False(t, resp.Effective[string(eventType)])

			saved, err := h.userRepo.GetNotificationPreference(t.Context(), 20)
			require.NoError(t, err)
			require.Equal(t, uModel.NotificationProfileCustom, saved.Profile)
			require.NotContains(t, saved.Overrides, eventType)
		})
	}
}

// Picking a profile is the other way to turn a gated type on, so the downgrade
// has to catch it too -- otherwise "planner" is a free back door to both
// digests.
func TestUpdateNotificationPreferenceDowngradesPlusOnlyProfilesWithoutSubscription(t *testing.T) {
	for _, profile := range []uModel.NotificationProfile{
		uModel.NotificationProfileBalanced,
		uModel.NotificationProfilePlanner,
		uModel.NotificationProfilePersistent,
	} {
		t.Run(string(profile), func(t *testing.T) {
			h, db := newHostedNotificationPreferenceHandler(t)
			createHostedUser(t, db, 21, false)

			w := putNotificationPreference(t, h, 21, fmt.Sprintf(`{"profile": %q}`, profile))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			var resp struct {
				Profile   string          `json:"profile"`
				Effective map[string]bool `json:"effective"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(t, string(uModel.NotificationProfileFreeDefault), resp.Profile,
				"a gated profile is saved as the free default, so the client re-renders what actually applies")
			for _, eventType := range uModel.PlusOnlyEvents() {
				require.False(t, resp.Effective[string(eventType)])
			}

			saved, err := h.userRepo.GetNotificationPreference(t.Context(), 21)
			require.NoError(t, err)
			require.Equal(t, uModel.NotificationProfileFreeDefault, saved.Profile)
		})
	}
}

// The downgrade only strips the gated types: what the user chose for the
// ungated ones is carried onto the free default rather than reset to its
// defaults.
func TestUpdateNotificationPreferenceDowngradeKeepsUngatedChoices(t *testing.T) {
	h, db := newHostedNotificationPreferenceHandler(t)
	createHostedUser(t, db, 26, true)

	// Saved while subscribed: Balanced with assignment notifications off.
	w := putNotificationPreference(t, h, 26,
		`{"profile": "balanced", "overrides": {"on_assignment": false}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.NoError(t, db.Exec("UPDATE users SET expiration = ? WHERE id = ?",
		time.Now().UTC().Add(-time.Hour), 26).Error)

	// Re-saving the same posture after the subscription lapsed.
	w = putNotificationPreference(t, h, 26,
		`{"profile": "balanced", "overrides": {"on_assignment": false, "daily_digest": true}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Profile   string          `json:"profile"`
		Effective map[string]bool `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, string(uModel.NotificationProfileFreeDefault), resp.Profile)
	require.False(t, resp.Effective["on_assignment"], "the user's own choice must survive the downgrade")
	require.False(t, resp.Effective["daily_digest"])
	require.True(t, resp.Effective["due"], "the free default's ungated defaults still apply")
}

// The profiles that enable nothing gated stay available for free, including the
// one a non-plus member starts on.
func TestUpdateNotificationPreferenceAllowsFreeProfilesWithoutSubscription(t *testing.T) {
	for _, profile := range []uModel.NotificationProfile{
		uModel.NotificationProfileJustInTime,
		uModel.NotificationProfileQuiet,
		uModel.NotificationProfileCustom,
	} {
		t.Run(string(profile), func(t *testing.T) {
			h, db := newHostedNotificationPreferenceHandler(t)
			createHostedUser(t, db, 22, false)

			w := putNotificationPreference(t, h, 22, fmt.Sprintf(`{"profile": %q}`, profile))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			saved, err := h.userRepo.GetNotificationPreference(t.Context(), 22)
			require.NoError(t, err)
			require.Equal(t, profile, saved.Profile)
		})
	}
}

func TestUpdateNotificationPreferenceAllowsPlusOnlyEventsForPlusMembers(t *testing.T) {
	h, db := newHostedNotificationPreferenceHandler(t)
	createHostedUser(t, db, 23, true)

	w := putNotificationPreference(t, h, 23, `{"profile": "planner"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Profile   string          `json:"profile"`
		Effective map[string]bool `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, string(uModel.NotificationProfilePlanner), resp.Profile)
	require.True(t, resp.Effective["daily_digest"])
	require.True(t, resp.Effective["weekly_digest"])
}

// A non-plus member who never configured notifications lands on the free
// default, and every plus-only type reads as off.
func TestGetNotificationPreferenceDefaultsNonPlusMembersToFreeDefault(t *testing.T) {
	h, db := newHostedNotificationPreferenceHandler(t)
	createHostedUser(t, db, 24, false)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/users/notifications/preferences", func(c *gin.Context) {
		c.Set("id", &uModel.UserDetails{User: uModel.User{ID: 24}})
		h.GetNotificationPreference(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users/notifications/preferences", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Profile   string          `json:"profile"`
		Effective map[string]bool `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, string(uModel.NotificationProfileFreeDefault), resp.Profile)
	for _, eventType := range uModel.PlusOnlyEvents() {
		require.False(t, resp.Effective[string(eventType)], "%s must read as off without a subscription", eventType)
	}
	// The free types are still on under the free default.
	require.True(t, resp.Effective["on_assignment"])
	require.True(t, resp.Effective["due"])
}

// A member who enabled the digests while subscribed and then lapsed keeps the
// saved profile, but nothing gated resolves to on any more.
func TestNotificationPreferenceStopsPlusOnlyEventsAfterSubscriptionLapses(t *testing.T) {
	h, db := newHostedNotificationPreferenceHandler(t)
	createHostedUser(t, db, 25, true)

	w := putNotificationPreference(t, h, 25, `{"profile": "planner"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.NoError(t, db.Exec("UPDATE users SET expiration = ? WHERE id = ?",
		time.Now().UTC().Add(-time.Hour), 25).Error)

	saved, err := h.userRepo.GetNotificationPreference(t.Context(), 25)
	require.NoError(t, err)
	require.Equal(t, uModel.NotificationProfilePlanner, saved.Profile, "the saved profile is kept, so resubscribing restores it")
	require.False(t, saved.IsPlus)
	for _, eventType := range uModel.PlusOnlyEvents() {
		require.False(t, saved.Enabled(eventType), "%s must stop firing once the subscription lapses", eventType)
	}
	require.True(t, saved.Enabled(uModel.NotificationEventDue))
}

// The response has to describe what was saved, not what was posted, or the
// client re-renders the toggles it sent and the switch looks like it failed.
func TestUpdateNotificationPreferenceOnAssignment(t *testing.T) {
	h, _ := newNotificationPreferenceHandler(t)
	for _, enabled := range []bool{true, false} {
		body, err := json.Marshal(map[string]interface{}{
			"profile": "custom", "overrides": map[string]bool{"on_assignment": enabled},
		})
		require.NoError(t, err)
		w := putNotificationPreference(t, h, 8, string(body))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Effective map[string]bool `json:"effective"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Contains(t, resp.Effective, "on_assignment")
		require.Equal(t, enabled, resp.Effective["on_assignment"])
		pref, err := h.userRepo.GetNotificationPreference(context.Background(), 8)
		require.NoError(t, err)
		require.Equal(t, enabled, pref.Enabled(uModel.NotificationEventOnAssignment))
	}
}

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
	if resp.Effective["overdue_followup"] {
		t.Errorf("expected quiet to keep overdue follow-up off, got %v", resp.Effective)
	}
}
