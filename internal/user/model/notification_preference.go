package user

import "time"

// NotificationProfile is a named bundle of NotificationEventType defaults a
// user can pick instead of toggling every event type individually.
type NotificationProfile string

const (
	NotificationProfileBalanced   NotificationProfile = "balanced"
	NotificationProfilePlanner    NotificationProfile = "planner"
	NotificationProfileJustInTime NotificationProfile = "just_in_time"
	NotificationProfilePersistent NotificationProfile = "persistent"
	NotificationProfileQuiet      NotificationProfile = "quiet"
	NotificationProfileCustom     NotificationProfile = "custom"
	// Defaults by tier -- use DefaultProfileFor rather than either directly.
	// JustInTime is the richest posture a non-plus member can wear whole.
	NotificationProfileDefault     = NotificationProfileBalanced
	NotificationProfileFreeDefault = NotificationProfileJustInTime
)

// NotificationEventType is one independently toggleable kind of notification.
type NotificationEventType string

const (
	// NotificationEventOnAssignment notifies a user when they become the assigned user.
	NotificationEventOnAssignment NotificationEventType = "on_assignment"
	// NotificationEventOnPendingApproval tells an approver (admin/manager) that
	// somebody's completion is waiting on them.
	NotificationEventOnPendingApproval NotificationEventType = "on_pending_approval"
	// NotificationEventOnRedemptionRequest tells an approver (admin/manager) that
	// somebody asked to redeem a reward.
	NotificationEventOnRedemptionRequest NotificationEventType = "on_redemption_request"
	// NotificationEventPreDue reminds before a chore is due.
	NotificationEventPreDue NotificationEventType = "pre_due"
	// NotificationEventDue reminds at/around the moment a chore becomes due.
	NotificationEventDue NotificationEventType = "due"
	// NotificationEventOverdueFollowup reminds about an unresolved overdue chore.
	NotificationEventOverdueFollowup NotificationEventType = "overdue_followup"
	// NotificationEventDailyDigest sends one message/day summarizing overdue + upcoming chores.
	NotificationEventDailyDigest NotificationEventType = "daily_digest"
	// NotificationEventWeeklyDigest sends one message/week previewing the week ahead.
	NotificationEventWeeklyDigest NotificationEventType = "weekly_digest"
)

// NotificationOverrides holds only the event types a user explicitly flipped
// away from their profile's default. Sparse by design: anything absent here
// falls back to the profile default, so switching profiles doesn't require
// migrating a fully-populated settings blob.
type NotificationOverrides map[NotificationEventType]bool

// profileDefaults is the source of truth for what each built-in profile
// enables. NotificationProfileCustom has no entry: Custom always starts from
// Overrides alone, defaulting anything unset to false (every type is opt-in
// once a user leaves the named profiles).
var profileDefaults = map[NotificationProfile]NotificationOverrides{
	NotificationProfileBalanced: {
		NotificationEventOnAssignment:        true,
		NotificationEventOnPendingApproval:   true,
		NotificationEventOnRedemptionRequest: true,
		NotificationEventPreDue:              true,
		NotificationEventDue:                 true,
		NotificationEventOverdueFollowup:     true,
		NotificationEventDailyDigest:         true,
		NotificationEventWeeklyDigest:        false,
	},
	NotificationProfilePlanner: {
		NotificationEventOnAssignment:        true,
		NotificationEventOnPendingApproval:   true,
		NotificationEventOnRedemptionRequest: true,
		NotificationEventPreDue:              false,
		NotificationEventDue:                 true,
		NotificationEventOverdueFollowup:     false,
		NotificationEventDailyDigest:         true,
		NotificationEventWeeklyDigest:        true,
	},
	NotificationProfileJustInTime: {
		NotificationEventOnAssignment:        true,
		NotificationEventOnPendingApproval:   true,
		NotificationEventOnRedemptionRequest: true,
		NotificationEventPreDue:              false,
		NotificationEventDue:                 true,
		NotificationEventOverdueFollowup:     false,
		NotificationEventDailyDigest:         false,
		NotificationEventWeeklyDigest:        false,
	},
	NotificationProfilePersistent: {
		NotificationEventOnAssignment:        true,
		NotificationEventOnPendingApproval:   true,
		NotificationEventOnRedemptionRequest: true,
		NotificationEventPreDue:              false,
		NotificationEventDue:                 true,
		NotificationEventOverdueFollowup:     true,
		NotificationEventDailyDigest:         true,
		NotificationEventWeeklyDigest:        false,
	},
	NotificationProfileQuiet: {
		NotificationEventOnAssignment:        false,
		NotificationEventOnPendingApproval:   false,
		NotificationEventOnRedemptionRequest: false,
		NotificationEventPreDue:              false,
		NotificationEventDue:                 false,
		NotificationEventOverdueFollowup:     false,
		NotificationEventDailyDigest:         false,
		NotificationEventWeeklyDigest:        false,
	},
}

// plusOnlyEvents require an active plus subscription: the ones that cost real
// delivery volume rather than reacting to something the user just did.
var plusOnlyEvents = map[NotificationEventType]bool{
	NotificationEventPreDue:          true,
	NotificationEventOverdueFollowup: true,
	NotificationEventDailyDigest:     true,
	NotificationEventWeeklyDigest:    true,
}

// RequiresPlus reports whether eventType is only available to plus members.
func RequiresPlus(eventType NotificationEventType) bool {
	return plusOnlyEvents[eventType]
}

// PlusOnlyEvents lists the plus-gated event types in the canonical order used
// by API responses.
func PlusOnlyEvents() []NotificationEventType {
	return []NotificationEventType{
		NotificationEventPreDue,
		NotificationEventOverdueFollowup,
		NotificationEventDailyDigest,
		NotificationEventWeeklyDigest,
	}
}

// DefaultProfileFor returns the profile a user who never configured
// notifications lands on.
func DefaultProfileFor(isPlus bool) NotificationProfile {
	if isPlus {
		return NotificationProfileDefault
	}
	return NotificationProfileFreeDefault
}

// AllEventTypes lists every toggleable event type in the canonical order used
// by API responses.
func AllEventTypes() []NotificationEventType {
	return []NotificationEventType{
		NotificationEventOnAssignment,
		NotificationEventOnPendingApproval,
		NotificationEventOnRedemptionRequest,
		NotificationEventPreDue,
		NotificationEventDue,
		NotificationEventOverdueFollowup,
		NotificationEventDailyDigest,
		NotificationEventWeeklyDigest,
	}
}

// PlusOnlyEventsEnabledBy returns the plus-gated event types that profile ⊕
// overrides would turn on. Empty means the combination needs no subscription.
func PlusOnlyEventsEnabledBy(profile NotificationProfile, overrides NotificationOverrides) []NotificationEventType {
	var blocked []NotificationEventType
	for _, eventType := range PlusOnlyEvents() {
		if notificationEnabled(profile, overrides, eventType, true) {
			blocked = append(blocked, eventType)
		}
	}
	return blocked
}

// DowngradeToFree rewrites a posture into the closest one a non-plus member
// can wear: every plus-only type off, falling back to the free default if the
// profile itself enables one. Ungated choices are preserved.
//
// Enabled already gates these at read time, so this is only about what gets
// persisted: the saved row should match what will really fire. It's the
// backstop for clients that don't lock the gated toggles themselves.
//
// The free default must stay a profile every released client already renders:
// one they can't post back reads as a profile switch on the next save, which
// drops the user's overrides. Label it as the free tier client-side instead.
func DowngradeToFree(profile NotificationProfile, overrides NotificationOverrides) (NotificationProfile, NotificationOverrides) {
	if len(PlusOnlyEventsEnabledBy(profile, overrides)) == 0 {
		return profile, overrides
	}

	// Resolve the free types under the posted profile before replacing it, then
	// re-sparsify against the one actually saved. Skipping the gated types here
	// is what clears their overrides.
	resolved := NotificationOverrides{}
	for _, eventType := range AllEventTypes() {
		if RequiresPlus(eventType) {
			continue
		}
		resolved[eventType] = notificationEnabled(profile, overrides, eventType, false)
	}

	downgraded := profile
	if len(PlusOnlyEventsEnabledBy(profile, nil)) > 0 {
		downgraded = NotificationProfileFreeDefault
	}
	return downgraded, SparseOverrides(downgraded, resolved)
}

// DefaultsForProfile returns the named profile's defaults. Unknown or Custom
// profiles return an empty map (nothing enabled) -- see profileDefaults.
func DefaultsForProfile(p NotificationProfile) NotificationOverrides {
	defaults, ok := profileDefaults[p]
	if !ok {
		return NotificationOverrides{}
	}
	out := make(NotificationOverrides, len(defaults))
	for k, v := range defaults {
		out[k] = v
	}
	return out
}

// SparseOverrides strips the entries that already match profile's default,
// keeping only the event types the user actually flipped away from it. This
// enforces the sparseness NotificationOverrides documents: a client that
// posts back a fully-populated map (e.g. the resolved "effective" set) would
// otherwise mask every profile default, making profile switches a no-op.
func SparseOverrides(profile NotificationProfile, overrides NotificationOverrides) NotificationOverrides {
	if len(overrides) == 0 {
		return nil
	}
	defaults := DefaultsForProfile(profile)
	sparse := make(NotificationOverrides, len(overrides))
	for eventType, enabled := range overrides {
		if defaults[eventType] == enabled {
			continue
		}
		sparse[eventType] = enabled
	}
	if len(sparse) == 0 {
		return nil
	}
	return sparse
}

// UserNotificationPreference is the single row per user storing their chosen
// notification profile plus any per-type overrides. Scoped per-user (not
// per-circle): a user's notification posture is a personal preference that
// should follow them across every circle they belong to.
type UserNotificationPreference struct {
	UserID    int                   `json:"userId" gorm:"column:user_id;primaryKey"`
	Profile   NotificationProfile   `json:"profile" gorm:"column:profile;default:balanced"`
	Overrides NotificationOverrides `json:"overrides" gorm:"column:overrides;serializer:json;type:json"`
	// DigestHour is the local hour (0-23, in the user's own User.Timezone) that
	// daily_digest and weekly_digest send at. Same hour drives both for now.
	//
	// Deliberately carries no column default: gorm omits a zero-valued field
	// with a `default` tag from the insert, which made hour 0 (midnight) --
	// a legitimate choice -- silently save as 8. Rows are always written with
	// an explicit hour; readers fill in DefaultDigestHour when there's no row.
	DigestHour int       `json:"digestHour" gorm:"column:digest_hour"`
	UpdatedAt  time.Time `json:"updatedAt" gorm:"column:updated_at"`
	// IsPlus is the owner's subscription state at load time, not a stored
	// column: the repository fills it in so a lapsed subscription stops
	// delivering gated notifications without rewriting the row.
	IsPlus bool `json:"isPlus" gorm:"-"`
}

func (UserNotificationPreference) TableName() string { return "user_notification_preferences" }

// DefaultDigestHour is the send hour a user gets until they configure one.
const DefaultDigestHour = 8

// Enabled resolves whether eventType should fire for this user: an explicit
// override always wins, otherwise it falls back to the profile default. A
// plus-only type is always false for a non-plus member, whatever the row says.
func (p *UserNotificationPreference) Enabled(eventType NotificationEventType) bool {
	if p == nil {
		return notificationEnabled(DefaultProfileFor(false), nil, eventType, false)
	}
	return notificationEnabled(p.Profile, p.Overrides, eventType, p.IsPlus)
}

func notificationEnabled(profile NotificationProfile, overrides NotificationOverrides, eventType NotificationEventType, isPlus bool) bool {
	if !isPlus && RequiresPlus(eventType) {
		return false
	}
	if enabled, ok := overrides[eventType]; ok {
		return enabled
	}
	return DefaultsForProfile(profile)[eventType]
}

// UserDigestSettings is the narrow user/preference projection needed by digest
// generation. Keeping it separate from User avoids loading unrelated columns.
type UserDigestSettings struct {
	ID         int                   `gorm:"column:id"`
	Timezone   string                `gorm:"column:timezone"`
	Profile    NotificationProfile   `gorm:"column:profile"`
	Overrides  NotificationOverrides `gorm:"column:overrides;serializer:json"`
	DigestHour int                   `gorm:"column:digest_hour"`
}

// Enabled resolves eventType for this user. Plus is taken as given: the only
// query that produces these (GetAllUserDigestSettings) filters to plus members.
func (s UserDigestSettings) Enabled(eventType NotificationEventType) bool {
	return notificationEnabled(s.Profile, s.Overrides, eventType, true)
}

// DefaultNotificationPreference is what a user who never configured
// notifications gets: their tier's default profile, no overrides.
func DefaultNotificationPreference(userID int, isPlus bool) *UserNotificationPreference {
	return &UserNotificationPreference{
		UserID:     userID,
		Profile:    DefaultProfileFor(isPlus),
		DigestHour: DefaultDigestHour,
		IsPlus:     isPlus,
	}
}
