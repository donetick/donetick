package user

import "testing"

func TestDefaultsForProfileMatchesDesignedBehavior(t *testing.T) {
	tests := []struct {
		profile   NotificationProfile
		eventType NotificationEventType
		want      bool
	}{
		// Balanced: a little prep, timely reminders, limited follow-up.
		{NotificationProfileBalanced, NotificationEventPreDue, true},
		{NotificationProfileBalanced, NotificationEventDue, true},
		{NotificationProfileBalanced, NotificationEventDailyDigest, true},
		{NotificationProfileBalanced, NotificationEventWeeklyDigest, false},

		// Planner: weekly reminder + daily preview, no pre-due noise.
		{NotificationProfilePlanner, NotificationEventPreDue, false},
		{NotificationProfilePlanner, NotificationEventWeeklyDigest, true},
		{NotificationProfilePlanner, NotificationEventDailyDigest, true},

		// Just-in-time: only remind when actionable, no digests.
		{NotificationProfileJustInTime, NotificationEventPreDue, false},
		{NotificationProfileJustInTime, NotificationEventDue, true},
		{NotificationProfileJustInTime, NotificationEventDailyDigest, false},
		{NotificationProfileJustInTime, NotificationEventWeeklyDigest, false},

		// Persistent: follow up and escalate on unresolved overdue work.
		{NotificationProfilePersistent, NotificationEventOverdueFollowup, true},
		{NotificationProfilePersistent, NotificationEventEscalation, true},

		// Quiet: only urgent/high-priority overdue nudges, nothing else.
		{NotificationProfileQuiet, NotificationEventDue, false},
		{NotificationProfileQuiet, NotificationEventOverdueFollowup, false},
		{NotificationProfileQuiet, NotificationEventEscalation, true},
		{NotificationProfileQuiet, NotificationEventDailyDigest, false},
	}

	for _, test := range tests {
		got := DefaultsForProfile(test.profile)[test.eventType]
		if got != test.want {
			t.Errorf("DefaultsForProfile(%s)[%s] = %v, want %v", test.profile, test.eventType, got, test.want)
		}
	}
}

func TestEnabledOverrideWinsOverProfileDefault(t *testing.T) {
	pref := &UserNotificationPreference{
		Profile: NotificationProfileBalanced,
		Overrides: NotificationOverrides{
			NotificationEventPreDue: false, // Balanced defaults this to true.
		},
	}

	if pref.Enabled(NotificationEventPreDue) {
		t.Error("explicit override to false should win over the Balanced default of true")
	}
	if !pref.Enabled(NotificationEventDue) {
		t.Error("unoverridden event type should fall back to the Balanced default")
	}
}

func TestEnabledNilPreferenceFallsBackToDefaultProfile(t *testing.T) {
	var pref *UserNotificationPreference
	if !pref.Enabled(NotificationEventDue) {
		t.Error("nil preference should resolve using the default (Balanced) profile")
	}
}

func TestEnabledCustomProfileOnlyHonorsOverrides(t *testing.T) {
	pref := &UserNotificationPreference{
		Profile: NotificationProfileCustom,
		Overrides: NotificationOverrides{
			NotificationEventDue: true,
		},
	}
	if !pref.Enabled(NotificationEventDue) {
		t.Error("Custom profile should honor an explicit override")
	}
	if pref.Enabled(NotificationEventWeeklyDigest) {
		t.Error("Custom profile should default anything unset to false")
	}
}

func TestDefaultsForProfileReturnsIndependentCopies(t *testing.T) {
	a := DefaultsForProfile(NotificationProfileBalanced)
	a[NotificationEventDue] = false
	b := DefaultsForProfile(NotificationProfileBalanced)
	if !b[NotificationEventDue] {
		t.Error("mutating one DefaultsForProfile result should not affect another call's result")
	}
}

// A client that posts back the full resolved set it was displaying must not
// end up pinning every event type: those entries aren't user intent, and as
// overrides they would mask the profile for good.
func TestSparseOverridesDropsEntriesMatchingTheProfile(t *testing.T) {
	dense := NotificationOverrides{
		NotificationEventPreDue:          true,
		NotificationEventDue:             true,
		NotificationEventOverdueFollowup: true,
		NotificationEventEscalation:      false,
		NotificationEventDailyDigest:     true,
		NotificationEventWeeklyDigest:    true, // the only one Balanced disables
	}

	sparse := SparseOverrides(NotificationProfileBalanced, dense)

	if len(sparse) != 1 {
		t.Fatalf("expected only the flipped event type to survive, got %v", sparse)
	}
	if !sparse[NotificationEventWeeklyDigest] {
		t.Errorf("expected the weekly digest override to survive, got %v", sparse)
	}
}

func TestSparseOverridesReturnsNilWhenNothingDiffersFromTheProfile(t *testing.T) {
	if got := SparseOverrides(NotificationProfileQuiet, DefaultsForProfile(NotificationProfileQuiet)); got != nil {
		t.Errorf("expected no overrides when every entry matches the profile, got %v", got)
	}
}

// Custom has no profile defaults, so everything is opt-in: only the event
// types explicitly turned on carry information.
func TestSparseOverridesKeepsEnabledEntriesForCustom(t *testing.T) {
	sparse := SparseOverrides(NotificationProfileCustom, NotificationOverrides{
		NotificationEventDue:          true,
		NotificationEventDailyDigest:  false,
		NotificationEventWeeklyDigest: false,
	})

	if len(sparse) != 1 || !sparse[NotificationEventDue] {
		t.Errorf("expected only the enabled event type to survive, got %v", sparse)
	}
}
