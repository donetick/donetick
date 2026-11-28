package chore

import (
	"errors"
	"math/rand"
	"reflect"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func rotationTimingEqual(a, b *chModel.AssignmentRotation) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Mode == b.Mode &&
		reflect.DeepEqual(a.Count, b.Count) &&
		reflect.DeepEqual(a.Weekday, b.Weekday) &&
		reflect.DeepEqual(a.DayOfMonth, b.DayOfMonth)
}

func initialRotationProgress(chore *chModel.Chore) *chModel.AssignmentRotationProgress {
	progress := &chModel.AssignmentRotationProgress{ChoreID: chore.ID}
	if chore.AssignmentRotation == nil {
		return progress
	}
	if chore.AssignmentRotation.Mode == chModel.AssignmentRotationModeScheduledOccurrences && chore.NextDueDate != nil {
		progress.Counter = 1
	}
	if chore.NextDueDate != nil {
		progress.LastBoundary = rotationBoundary(chore, *chore.NextDueDate)
	}
	return progress
}

func syncRotationProgress(ctxDB *gorm.DB, previous, current *chModel.Chore) error {
	if current.AssignmentRotation == nil {
		if previous == nil || previous.AssignmentRotation == nil {
			return nil
		}
		return ctxDB.Where("chore_id = ?", current.ID).Delete(&chModel.AssignmentRotationProgress{}).Error
	}
	if previous != nil && rotationTimingEqual(previous.AssignmentRotation, current.AssignmentRotation) {
		return nil
	}
	progress := initialRotationProgress(current)
	return ctxDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chore_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"counter", "last_boundary", "updated_at"}),
	}).Create(progress).Error
}

func copyRotationProgress(p *chModel.AssignmentRotationProgress) *chModel.AssignmentRotationProgress {
	if p == nil {
		return nil
	}
	copy := *p
	return &copy
}

// nextConfiguredAssignee evaluates and persists custom-rotation progress in the
// same transaction that advances the occurrence. finalized is true only for a
// completed (including approved) occurrence; skips and approval submissions do
// not increment completion policies.
func nextConfiguredAssignee(tx *gorm.DB, chore *chModel.Chore, nextDueDate *time.Time, fallback *int, finalized bool) (*int, *chModel.AssignmentRotationProgress, error) {
	policy := chore.AssignmentRotation
	if policy == nil || nextDueDate == nil {
		return fallback, nil, nil
	}

	query := tx
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var progress chModel.AssignmentRotationProgress
	err := query.Where("chore_id = ?", chore.ID).First(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		progress = *initialRotationProgress(chore)
	} else if err != nil {
		return nil, nil, err
	}
	before := copyRotationProgress(&progress)

	switches := 0
	switch policy.Mode {
	case chModel.AssignmentRotationModeDefault:
		switches = 1
	case chModel.AssignmentRotationModeScheduledOccurrences:
		if progress.Counter <= 0 {
			progress.Counter = 1
		}
		slots := chore.RotationAdvanceSlots
		if slots < 1 {
			slots = 1
		}
		for i := 0; i < slots; i++ {
			if progress.Counter >= *policy.Count {
				switches++
				progress.Counter = 1
			} else {
				progress.Counter++
			}
		}
	case chModel.AssignmentRotationModeCompletions:
		if finalized {
			progress.Counter++
			if progress.Counter >= *policy.Count {
				switches = 1
				progress.Counter = 0
			}
		}
	case chModel.AssignmentRotationModeWeek, chModel.AssignmentRotationModeMonth:
		boundary := rotationBoundary(chore, *nextDueDate)
		if progress.LastBoundary == "" {
			progress.LastBoundary = rotationBoundary(chore, valueOr(nextDueDate, chore.NextDueDate))
		}
		if boundary != "" && boundary != progress.LastBoundary {
			switches = 1
		}
		progress.LastBoundary = boundary
	}

	assigned := chore.AssignedTo
	for i := 0; i < switches; i++ {
		assigned = rotateConfiguredAssignee(chore, assigned)
	}
	if err := tx.Save(&progress).Error; err != nil {
		return nil, nil, err
	}
	return assigned, before, nil
}

func valueOr(primary, fallback *time.Time) time.Time {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return time.Time{}
}

func configuredOrder(chore *chModel.Chore) []int {
	eligible := make(map[int]struct{}, len(chore.Assignees))
	for _, assignee := range chore.Assignees {
		eligible[assignee.UserID] = struct{}{}
	}
	order := make([]int, 0, len(eligible))
	seen := make(map[int]struct{}, len(eligible))
	if chore.AssignmentRotation != nil {
		for _, id := range chore.AssignmentRotation.Order {
			if _, ok := eligible[id]; !ok {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			order = append(order, id)
		}
	}
	for _, assignee := range chore.Assignees {
		if _, ok := seen[assignee.UserID]; !ok {
			seen[assignee.UserID] = struct{}{}
			order = append(order, assignee.UserID)
		}
	}
	return order
}

func rotateConfiguredAssignee(chore *chModel.Chore, current *int) *int {
	order := configuredOrder(chore)
	if len(order) == 0 {
		return current
	}
	if current == nil {
		id := order[0]
		return &id
	}
	currentIndex := -1
	for i, id := range order {
		if id == *current {
			currentIndex = i
			break
		}
	}
	if currentIndex < 0 {
		id := order[0]
		return &id
	}
	if chore.AssignStrategy == chModel.AssignmentStrategyRandom || chore.AssignStrategy == chModel.AssignmentStrategyRandomExceptLastAssigned {
		choices := append([]int(nil), order[:currentIndex]...)
		choices = append(choices, order[currentIndex+1:]...)
		if len(choices) == 0 {
			return current
		}
		id := choices[rand.Intn(len(choices))]
		return &id
	}
	id := order[(currentIndex+1)%len(order)]
	return &id
}

func rotationBoundary(chore *chModel.Chore, occurrence time.Time) string {
	if chore.AssignmentRotation == nil || occurrence.IsZero() {
		return ""
	}
	loc := time.UTC
	if chore.FrequencyMetadataV2 != nil && chore.FrequencyMetadataV2.Timezone != "" {
		if loaded, err := time.LoadLocation(chore.FrequencyMetadataV2.Timezone); err == nil {
			loc = loaded
		}
	}
	local := occurrence.In(loc)
	switch chore.AssignmentRotation.Mode {
	case chModel.AssignmentRotationModeWeek:
		target := time.Monday
		if chore.AssignmentRotation.Weekday != nil {
			target = rotationWeekday(*chore.AssignmentRotation.Weekday)
		}
		delta := (int(local.Weekday()) - int(target) + 7) % 7
		boundary := local.AddDate(0, 0, -delta)
		return boundary.Format("2006-01-02")
	case chModel.AssignmentRotationModeMonth:
		day := 1
		if chore.AssignmentRotation.DayOfMonth != nil {
			day = *chore.AssignmentRotation.DayOfMonth
		}
		boundary := clampedMonthDay(local.Year(), local.Month(), day, loc)
		if local.Before(boundary) {
			previous := local.AddDate(0, -1, 0)
			boundary = clampedMonthDay(previous.Year(), previous.Month(), day, loc)
		}
		return boundary.Format("2006-01-02")
	default:
		return ""
	}
}

func rotationWeekday(weekday string) time.Weekday {
	switch weekday {
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	case "sunday":
		return time.Sunday
	default:
		return time.Monday
	}
}

func clampedMonthDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}
