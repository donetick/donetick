package service

import (
	"fmt"
	"strings"
	"time"

	"context"

	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	dRepo "donetick.com/core/internal/device/repo"
	nModel "donetick.com/core/internal/notifier/model"
	nRepo "donetick.com/core/internal/notifier/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

// DigestChoreIDDaily/Weekly are sentinel chore_id values for digest notification
// rows, which aren't about one real chore. Negative so they can never collide
// with a real chore's (positive, autoincrement) ID.
const (
	DigestChoreIDDaily  = -1
	DigestChoreIDWeekly = -2
)

type DigestPlanner struct {
	choreRepo  *chRepo.ChoreRepository
	circleRepo *cRepo.CircleRepository
	userRepo   *uRepo.UserRepository
	deviceRepo *dRepo.DeviceRepository
	nRepo      *nRepo.NotificationRepository
}

func NewDigestPlanner(cr *chRepo.ChoreRepository, cir *cRepo.CircleRepository, ur *uRepo.UserRepository, dr *dRepo.DeviceRepository, nr *nRepo.NotificationRepository) *DigestPlanner {
	return &DigestPlanner{
		choreRepo:  cr,
		circleRepo: cir,
		userRepo:   ur,
		deviceRepo: dr,
		nRepo:      nr,
	}
}

// GenerateDueDigests scans every user and generates a daily and/or weekly digest
// notification for whoever is due one right now: their notification preference
// has the corresponding type enabled, the current local hour (in User.Timezone)
// matches their configured DigestHour, and (for weekly) today is Monday. Safe to
// call more often than once an hour -- ExistsNotification dedupes within the
// relevant window so a user is never digested twice for the same day/week.
func (d *DigestPlanner) GenerateDueDigests(c context.Context, now time.Time) (int, error) {
	log := logging.FromContext(c)
	users, err := d.userRepo.GetAllUserDigestSettings(c)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, user := range users {
		loc := resolveLocation(user.Timezone)
		local := now.In(loc)
		if local.Hour() != user.DigestHour {
			continue
		}

		if user.Enabled(uModel.NotificationEventDailyDigest) {
			if ok, err := d.generateDigest(c, user.ID, local, digestKindDaily); err != nil {
				log.Error("digest: failed to generate daily digest", "userID", user.ID, "err", err)
			} else if ok {
				sent++
			}
		}

		if local.Weekday() == time.Monday && user.Enabled(uModel.NotificationEventWeeklyDigest) {
			if ok, err := d.generateDigest(c, user.ID, local, digestKindWeekly); err != nil {
				log.Error("digest: failed to generate weekly digest", "userID", user.ID, "err", err)
			} else if ok {
				sent++
			}
		}
	}
	return sent, nil
}

type digestKind int

const (
	digestKindDaily digestKind = iota
	digestKindWeekly
)

// generateDigest builds and inserts one digest notification for user, local
// being the current time already converted to the user's own timezone. Returns
// false (no error) when a digest was already sent for this window.
func (d *DigestPlanner) generateDigest(c context.Context, userID int, local time.Time, kind digestKind) (bool, error) {
	sentinelChoreID := DigestChoreIDDaily
	windowStart := startOfDay(local)
	lookaheadDays := 1 // through the end of today
	if kind == digestKindWeekly {
		sentinelChoreID = DigestChoreIDWeekly
		windowStart = startOfWeek(local)
		lookaheadDays = 8 // through the end of the 7th day out
	}

	alreadySent, err := d.nRepo.ExistsNotification(c, userID, sentinelChoreID, windowStart.UTC())
	if err != nil {
		return false, err
	}
	if alreadySent {
		return false, nil
	}

	// Digests are push-only (FCM, or Bridge relaying FCM for self-hosted
	// instances -- internal/notifier.Notifier picks between them by TypeID),
	// never Telegram/Pushover/Discord or the circle webhook: it's a personal
	// summary, not a per-chore event those channels are built around.
	tokens, err := d.deviceRepo.GetActiveDeviceTokens(c, userID)
	if err != nil {
		return false, err
	}
	if len(tokens) == 0 {
		return false, nil // No device to push a digest to.
	}

	circles, err := d.circleRepo.GetUserCircles(c, userID)
	if err != nil {
		return false, err
	}
	if len(circles) == 0 {
		return false, nil
	}
	circleIDs := make([]int, len(circles))
	for i, circle := range circles {
		circleIDs[i] = circle.ID
	}

	upcomingBefore := startOfDay(local).AddDate(0, 0, lookaheadDays)

	chores, err := d.choreRepo.GetChoresForDigest(c, userID, circleIDs, upcomingBefore.UTC())
	if err != nil {
		return false, err
	}
	if len(chores) == 0 {
		return false, nil
	}

	nowUTC := local.UTC()
	var overdue, upcoming []*chModel.Chore
	for _, chore := range chores {
		if chore.NextDueDate.Before(nowUTC) {
			overdue = append(overdue, chore)
		} else {
			upcoming = append(upcoming, chore)
		}
	}

	title, body := formatDigestPayload(kind, overdue, upcoming, local.Location())

	notifications := make([]*nModel.Notification, 0, len(tokens))
	for _, token := range tokens {
		if token.Token == "" {
			continue
		}
		notifications = append(notifications, &nModel.Notification{
			ChoreID:      sentinelChoreID,
			CircleID:     circleIDs[0],
			UserID:       userID,
			TargetID:     token.Token,
			TypeID:       nModel.NotificationPlatformFCM,
			Text:         body,
			IsSent:       false,
			ScheduledFor: local.UTC(),
			CreatedAt:    local.UTC(),
			RawEvent: map[string]interface{}{
				"type":           string(digestEventType(kind)),
				"overdue_count":  len(overdue),
				"upcoming_count": len(upcoming),
				// Title/body convention read by fcm.FCMNotifier and
				// bridgenotifier.BridgeNotifier -- see their RawEvent["payload"] handling.
				"payload": map[string]interface{}{
					"title": title,
					"body":  body,
				},
			},
		})
	}
	if len(notifications) == 0 {
		return false, nil
	}

	if err := d.nRepo.BatchInsertNotifications(notifications); err != nil {
		return false, err
	}
	return true, nil
}

func digestEventType(kind digestKind) uModel.NotificationEventType {
	if kind == digestKindWeekly {
		return uModel.NotificationEventWeeklyDigest
	}
	return uModel.NotificationEventDailyDigest
}

// maxNamedTasks is how many task names a digest spells out before falling
// back to "+N more" -- past this a notification tray truncates the line
// anyway, so the interesting part (the count, the soonest task) should come
// first rather than get cut off mid-list.
const maxNamedTasks = 3

// formatDigestPayload builds the push notification's title and body.
//
// Daily leads with the count, since that's the only thing guaranteed visible
// before a lock-screen notification truncates: "3 tasks today", not a
// generic "summary" label. Weekly instead leads with the soonest task as a
// hook ("starting with Groceries (Mon)") since enumerating a week's worth of
// tasks is exactly the wall-of-text pattern that gets silently cut off.
func formatDigestPayload(kind digestKind, overdue, upcoming []*chModel.Chore, loc *time.Location) (title, body string) {
	if kind == digestKindWeekly {
		return formatWeeklyDigestPayload(overdue, upcoming, loc)
	}
	return formatDailyDigestPayload(overdue, upcoming)
}

func formatDailyDigestPayload(overdue, upcoming []*chModel.Chore) (title, body string) {
	total := len(overdue) + len(upcoming)
	title = fmt.Sprintf("🗓️ %d %s today", total, pluralize("task", total))

	var parts []string
	if len(overdue) > 0 {
		parts = append(parts, fmt.Sprintf("Overdue: %s", summarizeTaskNames(overdue)))
	}
	if len(upcoming) > 0 {
		parts = append(parts, fmt.Sprintf("Due today: %s", summarizeTaskNames(upcoming)))
	}
	body = strings.Join(parts, " · ")
	return title, body
}

func formatWeeklyDigestPayload(overdue, upcoming []*chModel.Chore, loc *time.Location) (title, body string) {
	title = "📅 Your week ahead"

	var overduePrefix string
	if len(overdue) > 0 {
		overduePrefix = fmt.Sprintf("%d %s overdue. ", len(overdue), pluralize("task", len(overdue)))
	}

	if len(upcoming) == 0 {
		body = strings.TrimSpace(overduePrefix + "Nothing new scheduled this week.")
		return title, body
	}

	first := upcoming[0]
	day := first.NextDueDate.In(loc).Format("Mon")
	if len(upcoming) == 1 {
		body = fmt.Sprintf("%s1 task this week: %s (%s).", overduePrefix, first.Name, day)
	} else {
		body = fmt.Sprintf("%s%d tasks this week, starting with %s (%s).", overduePrefix, len(upcoming), first.Name, day)
	}
	return title, body
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// summarizeTaskNames joins up to maxNamedTasks names, collapsing the rest
// into a "+N more" tail instead of spelling out a long list.
func summarizeTaskNames(chores []*chModel.Chore) string {
	if len(chores) <= maxNamedTasks {
		return choreNames(chores)
	}
	shown := choreNames(chores[:maxNamedTasks])
	return fmt.Sprintf("%s, +%d more", shown, len(chores)-maxNamedTasks)
}

func choreNames(chores []*chModel.Chore) string {
	names := make([]string, len(chores))
	for i, chore := range chores {
		names[i] = chore.Name
	}
	return strings.Join(names, ", ")
}

func resolveLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	day := startOfDay(t)
	// Monday-anchored week, matching the weekly digest's Monday send day.
	offset := (int(day.Weekday()) + 6) % 7 // Monday=0 .. Sunday=6
	return day.AddDate(0, 0, -offset)
}
