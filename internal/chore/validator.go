package chore

import (
	"fmt"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

func validateOverdueNagging(metadata *chModel.NotificationMetadata, frequency chModel.FrequencyType, dueDate *time.Time) error {
	if metadata == nil || metadata.OverdueNagging == nil {
		return nil
	}
	followUps := metadata.OverdueNagging
	if followUps.IntervalMinutes < 1 || followUps.IntervalMinutes > chModel.MaxOverdueNaggingMinutes {
		return fmt.Errorf("overdue follow-up interval must be between 1 and %d minutes", chModel.MaxOverdueNaggingMinutes)
	}
	if !followUps.ContinueUntilComplete && (followUps.MaxFollowUps < 1 || followUps.MaxFollowUps > chModel.MaxOverdueFollowUpLimit) {
		return fmt.Errorf("maximum overdue follow-ups must be between 1 and %d", chModel.MaxOverdueFollowUpLimit)
	}
	if dueDate == nil || frequency == chModel.FrequencyTypeTrigger || frequency == chModel.FrequencyTypeAlways {
		return fmt.Errorf("overdue follow-ups require a scheduled due date")
	}
	return nil
}

// ChoreReqStructLevelValidation registers the custom cross-field validation rules
func ChoreReqStructLevelValidation(sl validator.StructLevel) {
	req := sl.Current().Interface().(ChoreReq)

	if req.FrequencyMetadata != nil || req.Frequency != nil {
		validateFrequencyLogic(sl, req) // 1. Validate Frequency Logic
	}
	if req.AssignStrategy != "" || req.AssignedTo != nil || len(req.Assignees) > 0 {
		validateAssignments(sl, req) // 2. Validate Assignments
	}
	if req.Notification || req.NotificationMetadata != nil {
		validateNotifications(sl, req) // 3. Validate Notifications
	}
	validateConcurrencyControl(sl, req) // 4. Validate Optimistic Concurrency Control
}

func validateFrequencyLogic(sl validator.StructLevel, req ChoreReq) {
	hasMetadata := req.FrequencyMetadata != nil

	switch req.FrequencyType {
	case chModel.FrequencyTypeInterval:
		// Interval must have metadata and a defined unit (hours, days, etc.)
		if !hasMetadata || req.FrequencyMetadata.Unit == nil {
			sl.ReportError(req.FrequencyMetadata, "Unit", "unit", "required_with_interval", "")
		}
		// Interval requires a frequency value
		if req.Frequency == nil {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "required_with_interval", "")
		}

	case chModel.FrequencyTypeDayOfTheWeek:
		// Day of the week requires at least one specified day in the array
		if !hasMetadata || len(req.FrequencyMetadata.Days) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Days", "days", "required_with_day_of_week", "")
		}
		// Only validate occurrences if a specific monthly or quarterly week pattern is requested
		if hasMetadata && req.FrequencyMetadata.WeekPattern != nil {
			pattern := string(*req.FrequencyMetadata.WeekPattern)
			if pattern == "week_of_month" || pattern == "week_of_quarter" {
				hasOccurrences := len(req.FrequencyMetadata.Occurrences) > 0 || len(req.FrequencyMetadata.WeekNumbers) > 0
				if !hasOccurrences {
					sl.ReportError(req.FrequencyMetadata, "Occurrences", "occurrences", "required_with_week_pattern", "")
				}
			}
		}

	case chModel.FrequencyTypeDayOfTheMonth:
		// Day of the month requires at least one specified month in the array
		if !hasMetadata || len(req.FrequencyMetadata.Months) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Months", "months", "required_with_day_of_month", "")
		}
		// The struct tag handles min=1, but the upper bound of 31 requires struct-level checking
		if req.Frequency == nil || *req.Frequency > 31 {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "valid_day_of_month", "")
		}
	}
}

func validateAssignments(sl validator.StructLevel, req ChoreReq) {
	isNoAssignee := req.AssignStrategy == "no_assignee"
	hasAssigneesList := len(req.Assignees) > 0

	if isNoAssignee {
		// If the strategy is no_assignee, AssignedTo or Assignees must not be sent
		if req.AssignedTo != nil {
			sl.ReportError(req.AssignedTo, "AssignedTo", "assignedTo", "forbidden_with_no_assignee", "")
		}
		if hasAssigneesList {
			sl.ReportError(req.Assignees, "Assignees", "assignees", "forbidden_with_no_assignee", "")
		}
	} else {
		// Strategies that specifically require an assignees list to calculate the next assignee
		requiresList := req.AssignStrategy == "round_robin" ||
			req.AssignStrategy == "random" ||
			req.AssignStrategy == "least_assigned" ||
			req.AssignStrategy == "least_completed" ||
			req.AssignStrategy == "random_except_last_assigned"

		if requiresList && !hasAssigneesList {
			sl.ReportError(req.Assignees, "Assignees", "assignees", "required_with_assign_strategy", "")
		}
	}
}

func validateNotifications(sl validator.StructLevel, req ChoreReq) {
	hasNotificationMetadata := req.NotificationMetadata != nil && len(req.NotificationMetadata.Templates) != 0

	if req.Notification {
		// Notifications are invalid for 'trigger' and 'always' frequency types since neither has a due date to notify against
		if req.FrequencyType == chModel.FrequencyTypeTrigger || req.FrequencyType == chModel.FrequencyTypeAlways {
			sl.ReportError(req.Notification, "Notification", "notification", "forbidden_with_trigger_frequency", "")
		}

		// Metadata is required if notifications are enabled
		if !hasNotificationMetadata {
			sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "required_when_notifications_enabled", "")
		}
		if req.NextDueDate == nil {
			sl.ReportError(req.NextDueDate, "NextDueDate", "nextDueDate", "required_with_notifications", "")
		}
	} else if hasNotificationMetadata {
		// TODO(v0.1.78+): Re-enable strict validation once all clients stop sending notificationMetadata when notification is false.
		// sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "forbidden_when_notifications_disabled", "")
	}
}

func validateConcurrencyControl(sl validator.StructLevel, req ChoreReq) {
	if req.UpdatedAt != nil {
		// Allow a 30-second buffer for slight clock skew between client and server
		cooldown := time.Second * 30
		maxAllowedTime := time.Now().UTC().Add(cooldown)

		// Ensure the provided UpdatedAt timestamp is not in the future
		if req.UpdatedAt.After(maxAllowedTime) {
			sl.ReportError(req.UpdatedAt, "UpdatedAt", "updatedAt", "cannot_be_in_future", "")
		}
	}
}
