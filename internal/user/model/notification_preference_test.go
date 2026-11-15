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

		// Just-in-time: only remind when actionable, no digests. Doubles as the
		// free default, so it's spelled out in full -- it has to enable
		// everything a non-plus member can have, and nothing else.
		{NotificationProfileJustInTime, NotificationEventOnAssignment, true},
		{NotificationProfileJustInTime, NotificationEventOnPendingApproval, true},
		{NotificationProfileJustInTime, NotificationEventOnRedemptionRequest, true},
		{NotificationProfileJustInTime, NotificationEventPreDue, false},
		{NotificationProfileJustInTime, NotificationEventDue, true},
		{NotificationProfileJustInTime, NotificationEventOverdueFollowup, false},
		{NotificationProfileJustInTime, NotificationEventDailyDigest, false},
		{NotificationProfileJustInTime, NotificationEventWeeklyDigest, false},

		// Persistent: follow up on unresolved overdue work.
		{NotificationProfilePersistent, NotificationEventOverdueFollowup, true},

		// Quiet: minimal noise, overdue follow-up included.
		{NotificationProfileQuiet, NotificationEventDue, false},
		{NotificationProfileQuiet, NotificationEventOverdueFollowup, false},
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
		IsPlus:  true,
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

// A nil preference belongs to a user we know nothing about, so it resolves
// against the non-plus default rather than assuming a subscription.
func TestEnabledNilPreferenceFallsBackToDefaultProfile(t *testing.T) {
	var pref *UserNotificationPreference
	if !pref.Enabled(NotificationEventDue) {
		t.Error("nil preference should resolve using the free default profile")
	}
	for _, eventType := range PlusOnlyEvents() {
		if pref.Enabled(eventType) {
			t.Errorf("nil preference should not enable the plus-only %s", eventType)
		}
	}
}

// The gate overrides everything the row says: an explicit override to true is
// still a no for a non-plus member.
func TestEnabledGatesPlusOnlyEventsForNonPlusMembers(t *testing.T) {
	for _, eventType := range PlusOnlyEvents() {
		pref := &UserNotificationPreference{
			Profile:   NotificationProfileCustom,
			IsPlus:    false,
			Overrides: NotificationOverrides{eventType: true},
		}
		if pref.Enabled(eventType) {
			t.Errorf("%s is plus-only and must resolve to false without a subscription", eventType)
		}

		pref.IsPlus = true
		if !pref.Enabled(eventType) {
			t.Errorf("%s should resolve to true for a plus member who enabled it", eventType)
		}
	}
}

// The free default has to be a profile a non-plus member can wear whole,
// without anything being silently suppressed -- otherwise an unconfigured user
// is shown a posture that doesn't match what they receive.
func TestFreeDefaultProfileNeedsNoSubscription(t *testing.T) {
	if blocked := PlusOnlyEventsEnabledBy(NotificationProfileFreeDefault, nil); len(blocked) != 0 {
		t.Errorf("the free default should enable no plus-only event types, got %v", blocked)
	}

	free := &UserNotificationPreference{Profile: NotificationProfileFreeDefault, IsPlus: false}
	plus := &UserNotificationPreference{Profile: NotificationProfileFreeDefault, IsPlus: true}
	for eventType := range profileDefaults[NotificationProfileFreeDefault] {
		if free.Enabled(eventType) != plus.Enabled(eventType) {
			t.Errorf("the free default's %s should resolve the same with or without a subscription", eventType)
		}
	}
}

// The free default must be a profile every already-released client can render
// and post back. A name they don't know reads as a profile switch on the next
// save, which drops the user's overrides -- so it has to come from the set the
// update endpoint has always accepted.
func TestFreeDefaultProfileIsNotANewProfileName(t *testing.T) {
	for _, profile := range []NotificationProfile{
		NotificationProfileBalanced,
		NotificationProfilePlanner,
		NotificationProfileJustInTime,
		NotificationProfilePersistent,
		NotificationProfileQuiet,
		NotificationProfileCustom,
	} {
		if profile == NotificationProfileFreeDefault {
			return
		}
	}
	t.Errorf("the free default %q is not one of the originally released profiles", NotificationProfileFreeDefault)
}

func TestDowngradeToFree(t *testing.T) {
	// A combination that needs no subscription is returned untouched.
	overrides := NotificationOverrides{NotificationEventDue: false}
	profile, got := DowngradeToFree(NotificationProfileFreeDefault, overrides)
	if profile != NotificationProfileFreeDefault || got[NotificationEventDue] != false || len(got) != 1 {
		t.Errorf("a free posture should pass through unchanged, got %s %v", profile, got)
	}

	// A gated profile becomes the free default, carrying the ungated choices
	// over.
	profile, got = DowngradeToFree(NotificationProfilePlanner, NotificationOverrides{
		NotificationEventOnAssignment: false,
	})
	if profile != NotificationProfileFreeDefault {
		t.Errorf("Planner should downgrade to the free default, got %s", profile)
	}
	if blocked := PlusOnlyEventsEnabledBy(profile, got); len(blocked) != 0 {
		t.Errorf("the downgraded posture still enables %v", blocked)
	}
	if got[NotificationEventOnAssignment] != false {
		t.Errorf("the ungated override should survive, got %v", got)
	}

	// A free profile with a gated override keeps the profile and loses only
	// the override.
	profile, got = DowngradeToFree(NotificationProfileQuiet, NotificationOverrides{
		NotificationEventWeeklyDigest: true,
	})
	if profile != NotificationProfileQuiet {
		t.Errorf("Quiet enables nothing gated, so it should be kept, got %s", profile)
	}
	if _, ok := got[NotificationEventWeeklyDigest]; ok {
		t.Errorf("the gated override should be dropped, got %v", got)
	}

	// Downgrading is idempotent and never leaves a gated type on, whichever
	// profile it starts from.
	for profile := range profileDefaults {
		p, o := DowngradeToFree(profile, DefaultsForProfile(profile))
		if blocked := PlusOnlyEventsEnabledBy(p, o); len(blocked) != 0 {
			t.Errorf("%s downgraded to %s still enables %v", profile, p, blocked)
		}
		p2, o2 := DowngradeToFree(p, o)
		if p2 != p || len(o2) != len(o) {
			t.Errorf("%s: downgrading twice changed the result (%s %v -> %s %v)", profile, p, o, p2, o2)
		}
	}
}

func TestPlusOnlyEventsEnabledByReportsEveryWayOfTurningOneOn(t *testing.T) {
	// By profile default.
	if blocked := PlusOnlyEventsEnabledBy(NotificationProfilePlanner, nil); len(blocked) != 2 {
		t.Errorf("Planner enables both digests, got %v", blocked)
	}
	// By override on an otherwise-free profile.
	blocked := PlusOnlyEventsEnabledBy(NotificationProfileQuiet, NotificationOverrides{
		NotificationEventWeeklyDigest: true,
	})
	if len(blocked) != 1 || blocked[0] != NotificationEventWeeklyDigest {
		t.Errorf("an override on a free profile should still be reported, got %v", blocked)
	}
	// An override that turns a gated default off clears it.
	if blocked := PlusOnlyEventsEnabledBy(NotificationProfilePlanner, NotificationOverrides{
		NotificationEventDailyDigest:  false,
		NotificationEventWeeklyDigest: false,
	}); len(blocked) != 0 {
		t.Errorf("Planner with both digests overridden off needs no subscription, got %v", blocked)
	}
	// Ungated types never show up.
	if blocked := PlusOnlyEventsEnabledBy(NotificationProfileCustom, NotificationOverrides{
		NotificationEventDue:          true,
		NotificationEventOnAssignment: true,
	}); len(blocked) != 0 {
		t.Errorf("free event types should never be reported, got %v", blocked)
	}
}

func TestDefaultProfileForTier(t *testing.T) {
	if got := DefaultProfileFor(true); got != NotificationProfileBalanced {
		t.Errorf("plus members should default to balanced, got %q", got)
	}
	if got := DefaultProfileFor(false); got != NotificationProfileJustInTime {
		t.Errorf("non-plus members should default to just_in_time, got %q", got)
	}
}

func TestEnabledCustomProfileOnlyHonorsOverrides(t *testing.T) {
	pref := &UserNotificationPreference{
		Profile: NotificationProfileCustom,
		IsPlus:  true,
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
