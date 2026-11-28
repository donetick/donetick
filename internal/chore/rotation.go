package chore

import (
	"fmt"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func validateAndNormalizeAssignmentRotation(policy *chModel.AssignmentRotation, strategy chModel.AssignmentStrategy, frequency chModel.FrequencyType, assignees []chModel.ChoreAssignees, dueDate *time.Time) (*chModel.AssignmentRotation, error) {
	if policy == nil {
		return nil, nil
	}
	switch strategy {
	case chModel.AssignmentStrategyRoundRobin, chModel.AssignmentStrategyRandom, chModel.AssignmentStrategyRandomExceptLastAssigned:
	default:
		return nil, fmt.Errorf("assignmentRotation is only supported with round_robin, random or random_except_last_assigned")
	}
	orderOnlyUnscheduled := strategy == chModel.AssignmentStrategyRoundRobin && policy.Mode == chModel.AssignmentRotationModeDefault &&
		(frequency == chModel.FrequencyTypeAlways || frequency == chModel.FrequencyTypeTrigger)
	if !orderOnlyUnscheduled {
		switch frequency {
		case chModel.FrequencyTypeOnce, chModel.FrequencyTypeNoRepeat, chModel.FrequencyTypeAlways, chModel.FrequencyTypeTrigger:
			return nil, fmt.Errorf("assignmentRotation requires a scheduled repeating task")
		}
		if dueDate == nil {
			return nil, fmt.Errorf("assignmentRotation requires a scheduled repeating task")
		}
	}
	eligibleCount := make(map[int]struct{}, len(assignees))
	for _, assignee := range assignees {
		eligibleCount[assignee.UserID] = struct{}{}
	}
	if len(eligibleCount) < 2 {
		return nil, fmt.Errorf("assignmentRotation requires at least two explicit assignees")
	}

	normalized := &chModel.AssignmentRotation{Mode: policy.Mode}
	switch policy.Mode {
	case chModel.AssignmentRotationModeDefault:
		if policy.Count != nil {
			return nil, fmt.Errorf("assignmentRotation.count is only valid for counted modes")
		}
	case chModel.AssignmentRotationModeScheduledOccurrences, chModel.AssignmentRotationModeCompletions:
		if policy.Count == nil || *policy.Count < 1 || *policy.Count > 1000 {
			return nil, fmt.Errorf("assignmentRotation.count must be an integer between 1 and 1000")
		}
		normalized.Count = policy.Count
	case chModel.AssignmentRotationModeWeek:
		if policy.Count != nil {
			return nil, fmt.Errorf("assignmentRotation.count is not valid for week")
		}
		if policy.Weekday != nil && !validRotationWeekday(*policy.Weekday) {
			return nil, fmt.Errorf("assignmentRotation.weekday must be monday through sunday")
		}
		normalized.Weekday = policy.Weekday
	case chModel.AssignmentRotationModeMonth:
		if policy.Count != nil {
			return nil, fmt.Errorf("assignmentRotation.count is not valid for month")
		}
		if policy.DayOfMonth != nil && (*policy.DayOfMonth < 1 || *policy.DayOfMonth > 31) {
			return nil, fmt.Errorf("assignmentRotation.dayOfMonth must be an integer between 1 and 31")
		}
		normalized.DayOfMonth = policy.DayOfMonth
	default:
		return nil, fmt.Errorf("invalid assignmentRotation.mode")
	}

	eligible := make(map[int]struct{}, len(assignees))
	for _, assignee := range assignees {
		eligible[assignee.UserID] = struct{}{}
	}
	seen := make(map[int]struct{}, len(assignees))
	for _, id := range policy.Order {
		if _, ok := eligible[id]; !ok {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		normalized.Order = append(normalized.Order, id)
	}
	for _, assignee := range assignees {
		if _, ok := seen[assignee.UserID]; ok {
			continue
		}
		seen[assignee.UserID] = struct{}{}
		normalized.Order = append(normalized.Order, assignee.UserID)
	}
	return normalized, nil
}

func validRotationWeekday(weekday string) bool {
	switch weekday {
	case "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		return true
	default:
		return false
	}
}
