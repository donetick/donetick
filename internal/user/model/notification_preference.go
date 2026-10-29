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
	NotificationProfileDefault                        = NotificationProfileBalanced
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
}

func (UserNotificationPreference) TableName() string { return "user_notification_preferences" }

// DefaultDigestHour is the send hour a user gets until they configure one.
const DefaultDigestHour = 8

// Enabled resolves whether eventType should fire for this user: an explicit
// override always wins, otherwise it falls back to the profile default.
func (p *UserNotificationPreference) Enabled(eventType NotificationEventType) bool {
	if p == nil {
		return notificationEnabled(NotificationProfileDefault, nil, eventType)
	}
	return notificationEnabled(p.Profile, p.Overrides, eventType)
}

func notificationEnabled(profile NotificationProfile, overrides NotificationOverrides, eventType NotificationEventType) bool {
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

func (s UserDigestSettings) Enabled(eventType NotificationEventType) bool {
	return notificationEnabled(s.Profile, s.Overrides, eventType)
}

// DefaultNotificationPreference is what a user who never configured
// notifications gets: the Balanced profile, no overrides.
func DefaultNotificationPreference(userID int) *UserNotificationPreference {
	return &UserNotificationPreference{
		UserID:     userID,
		Profile:    NotificationProfileDefault,
		DigestHour: DefaultDigestHour,
	}
}
