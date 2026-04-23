package chore

import (
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

// ChoreReqStructLevelValidation registers the custom cross-field validation rules
func ChoreReqStructLevelValidation(sl validator.StructLevel) {
	req := sl.Current().Interface().(ChoreReq)

	validateFrequencyLogic(sl, req)     // 1. Validate Frequency Logic
	validateAssignments(sl, req)        // 2. Validate Assignments
	validateNotifications(sl, req)      // 3. Validate Notifications
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
	hasNotificationMetadata := req.NotificationMetadata != nil

	if req.Notification {
		// Notifications are invalid for 'trigger' frequency types
		if req.FrequencyType == chModel.FrequencyTypeTrigger {
			sl.ReportError(req.Notification, "Notification", "notification", "forbidden_with_trigger_frequency", "")
		}

		// Metadata is required if notifications are enabled
		if !hasNotificationMetadata {
			sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "required_when_notifications_enabled", "")
		}
	} else if hasNotificationMetadata {
		// If notifications are disabled (or nil), ensure the client isn't sending unused metadata
		sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "forbidden_when_notifications_disabled", "")
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
