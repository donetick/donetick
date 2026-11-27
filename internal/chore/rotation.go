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
	if strategy != chModel.AssignmentStrategyRoundRobin && strategy != chModel.AssignmentStrategyRandom {
		return nil, fmt.Errorf("assignmentRotation is only supported with round_robin or random")
	}
	switch frequency {
	case chModel.FrequencyTypeOnce, chModel.FrequencyTypeNoRepeat, chModel.FrequencyTypeAlways, chModel.FrequencyTypeTrigger:
		return nil, fmt.Errorf("assignmentRotation requires a scheduled repeating task")
	}
	if dueDate == nil {
		return nil, fmt.Errorf("assignmentRotation requires a scheduled repeating task")
	}
	eligibleCount := make(map[int]struct{}, len(assignees))
	for _, assignee := range assignees {
		eligibleCount[assignee.UserID] = struct{}{}
	}
	if len(eligibleCount) < 2 {
		return nil, fmt.Errorf("assignmentRotation requires at least two explicit assignees")
	}

	normalized := &chModel.AssignmentRotation{Mode: policy.Mode, Count: policy.Count}
	switch policy.Mode {
	case chModel.AssignmentRotationModeDefault:
		if policy.Count != nil {
			return nil, fmt.Errorf("assignmentRotation.count is only valid for counted or calendar modes")
		}
	case chModel.AssignmentRotationModeScheduledOccurrences, chModel.AssignmentRotationModeCompletions:
		if policy.Count == nil || *policy.Count < 1 || *policy.Count > 1000 {
			return nil, fmt.Errorf("assignmentRotation.count must be an integer between 1 and 1000")
		}
	case chModel.AssignmentRotationModeWeek:
		// count is optional for calendar policies for backwards compatibility. If
		// omitted, the weekday of the current scheduled occurrence is used.
		if policy.Count != nil && (*policy.Count < 1 || *policy.Count > 7) {
			return nil, fmt.Errorf("assignmentRotation.count for week must be between 1 and 7")
		}
	case chModel.AssignmentRotationModeMonth:
		if policy.Count != nil && (*policy.Count < 1 || *policy.Count > 31) {
			return nil, fmt.Errorf("assignmentRotation.count for month must be between 1 and 31")
		}
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
