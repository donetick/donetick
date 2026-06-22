package chore

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"donetick.com/core/logging"
	"github.com/teambition/rrule-go"
)

// scheduleNextDueDate computes the next due date for a recurring chore using its
// RRULE-equivalent frequency model. Non-recurring kinds (once/no_repeat/trigger)
// return nil; adaptive chores are scheduled separately via scheduleAdaptiveNextDueDate.
func scheduleNextDueDate(ctx context.Context, chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
	switch chore.FrequencyType {
	case chModel.FrequencyTypeOnce, chModel.FrequencyTypeNoRepeat, chModel.FrequencyTypeTrigger:
		return nil, nil
	}

	if !isRRuleSchedulable(chore.FrequencyType) {
		return nil, fmt.Errorf("invalid frequency type: %s", chore.FrequencyType)
	}

	// Determine the anchor for the next occurrence. Rolling chores re-anchor at
	// the completion time; otherwise the series continues from the current due
	// date (falling back to the completion date when no due date is set).
	anchor := completedDate.UTC()
	if !chore.IsRolling && chore.NextDueDate != nil {
		anchor = chore.NextDueDate.UTC()
	}

	dtstart := rruleDtstart(chore, anchor)

	opt, err := buildROption(chore, dtstart)
	if err != nil {
		log := logging.FromContext(ctx)
		log.Error("error building recurrence rule", "error", err, "chore_id", chore.ID)
		return nil, err
	}

	rule, err := rrule.NewRRule(opt)
	if err != nil {
		log := logging.FromContext(ctx)
		log.Error("error constructing rrule", "error", err, "chore_id", chore.ID)
		return nil, err
	}

	// First occurrence strictly after the anchor.
	next := rule.After(dtstart, false)
	if next.IsZero() {
		return nil, fmt.Errorf("no next due date found for chore %d", chore.ID)
	}

	nextUTC := next.UTC()
	return &nextUTC, nil
}

// rruleDtstart builds the RRULE DTSTART: the anchor date in the chore's timezone,
// with its time-of-day overridden by the frequency metadata Time when provided.
func rruleDtstart(chore *chModel.Chore, anchor time.Time) time.Time {
	loc := time.UTC
	meta := chore.FrequencyMetadataV2
	if meta != nil && meta.Timezone != "" {
		if l, err := time.LoadLocation(meta.Timezone); err == nil {
			loc = l
		}
	}

	dt := anchor.In(loc)
	if meta != nil && meta.Time != "" {
		if t, err := time.Parse(time.RFC3339, meta.Time); err == nil {
			tl := t.In(loc)
			dt = time.Date(dt.Year(), dt.Month(), dt.Day(), tl.Hour(), tl.Minute(), tl.Second(), 0, loc)
		}
	}
	return dt
}

func scheduleAdaptiveNextDueDate(chore *chModel.Chore, completedDate time.Time, history []*chModel.ChoreHistory) (*time.Time, error) {

	history = append([]*chModel.ChoreHistory{
		{
			PerformedAt: &completedDate,
		},
	}, history...)

	if len(history) < 2 {
		if chore.NextDueDate != nil {
			diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
			nextDueDate := completedDate.UTC().Add(diff)
			return &nextDueDate, nil
		}
		return nil, nil
	}

	var totalDelay float64
	var totalWeight float64
	decayFactor := 0.5 // Adjust this value to control the decay rate

	for i := 0; i < len(history)-1; i++ {
		// Skip entries with nil PerformedAt
		if history[i].PerformedAt == nil || history[i+1].PerformedAt == nil {
			continue
		}
		delay := history[i].PerformedAt.UTC().Sub(history[i+1].PerformedAt.UTC()).Seconds()
		weight := math.Pow(decayFactor, float64(i))
		totalDelay += delay * weight
		totalWeight += weight
	}

	// If no valid history entries, fall back to default behavior
	if totalWeight == 0 {
		if chore.NextDueDate != nil {
			diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
			nextDueDate := completedDate.UTC().Add(diff)
			return &nextDueDate, nil
		}
		return nil, nil
	}

	averageDelay := totalDelay / totalWeight
	nextDueDate := completedDate.UTC().Add(time.Duration(averageDelay) * time.Second)

	return &nextDueDate, nil
}
func RemoveAssigneeAndReassign(chore *chModel.Chore, userID int) {
	for i, assignee := range chore.Assignees {
		if assignee.UserID == userID {
			chore.Assignees = append(chore.Assignees[:i], chore.Assignees[i+1:]...)
			break
		}
	}

	// Handle no assignee strategy
	switch {
	case chore.AssignStrategy == chModel.AssignmentStrategyNoAssignee:
		chore.AssignedTo = nil // Set to nil to indicate no assignee
	case len(chore.Assignees) == 0:
		createdBy := chore.CreatedBy
		chore.AssignedTo = &createdBy
	default:
		userID := chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
		chore.AssignedTo = &userID
	}
	chore.UpdatedAt = time.Now().UTC()
}
