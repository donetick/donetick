package chore

import (
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

func ChoreReqStructLevelValidation(sl validator.StructLevel) {
	req := sl.Current().Interface().(ChoreReq)

	validateFrequencyLogic(sl, req)     // 1. Validate Frequency Logic
	validateTimeFormat(sl, req)         // 2. Validate RFC3339 Time format
	validateAssignments(sl, req)        // 3. Validate Assignments
	validateNotifications(sl, req)      // 4. Validate Notifications
	validateConcurrencyControl(sl, req) // 5. Validate Optimistic Concurrency Control
	validateRollingLogic(sl, req)       // 6. Validate Rolling Logic

}

func validateFrequencyLogic(sl validator.StructLevel, req ChoreReq) {
	hasMetadata := req.FrequencyMetadata != nil

	switch req.FrequencyType {
	case chModel.FrequencyTypeInterval:
		if !hasMetadata || req.FrequencyMetadata.Unit == nil {
			sl.ReportError(req.FrequencyMetadata, "Unit", "unit", "required_with_interval", "")
		}
		// Interval also requires a frequency > 0
		if req.Frequency == nil || *req.Frequency <= 0 {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "required_with_interval", "")
		}

	case chModel.FrequencyTypeDayOfTheWeek:
		if !hasMetadata || len(req.FrequencyMetadata.Days) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Days", "days", "required_with_day_of_week", "")
		}
		// Only validate occurrences if a specific week pattern is requested
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
		if !hasMetadata || len(req.FrequencyMetadata.Months) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Months", "months", "required_with_day_of_month", "")
		}
		// Safe check for nil before dereferencing
		if req.Frequency == nil || *req.Frequency <= 0 || *req.Frequency > 31 {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "valid_day_of_month", "")
		}
	}
}

func validateTimeFormat(sl validator.StructLevel, req ChoreReq) {
	if req.FrequencyType == chModel.FrequencyTypeDayOfTheMonth ||
		req.FrequencyType == chModel.FrequencyTypeDayOfTheWeek ||
		req.FrequencyType == chModel.FrequencyTypeInterval {

		hasMetadata := req.FrequencyMetadata != nil
		if hasMetadata && req.FrequencyMetadata.Time != "" {
			if _, err := time.Parse(time.RFC3339, req.FrequencyMetadata.Time); err != nil {
				sl.ReportError(req.FrequencyMetadata.Time, "Time", "time", "invalid_rfc3339", "")
			}
		}
	}
}

func validateAssignments(sl validator.StructLevel, req ChoreReq) {
	isNoAssignee := req.AssignStrategy == "no_assignee"
	hasAssigneesList := len(req.Assignees) > 0

	if isNoAssignee {
		// If strategy is no_assignee, AssignedTo or Assignees should not be sent
		if req.AssignedTo != nil {
			sl.ReportError(req.AssignedTo, "AssignedTo", "assignedTo", "forbidden_with_no_assignee", "")
		}
		if hasAssigneesList {
			sl.ReportError(req.Assignees, "Assignees", "assignees", "forbidden_with_no_assignee", "")
		}
	} else {
		// Strategies that specifically require an assignees list to function
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
	hasNotification := req.Notification != nil && *req.Notification
	hasNotificationMetadata := req.NotificationMetadata != nil

	if hasNotification {
		// Notifications are invalid for 'trigger' frequency types
		if req.FrequencyType == chModel.FrequencyTypeTrigger {
			sl.ReportError(req.Notification, "Notification", "notification", "forbidden_with_trigger_frequency", "")
		}

		if !hasNotificationMetadata {
			sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "required_when_notifications_enabled", "")
		} else {
			// CircleGroupID is only required if CircleGroup is toggled ON
			if req.NotificationMetadata.CircleGroup {
				if req.NotificationMetadata.CircleGroupID == nil || *req.NotificationMetadata.CircleGroupID <= 0 {
					sl.ReportError(req.NotificationMetadata.CircleGroupID, "CircleGroupID", "circleGroupID", "required_with_circle_group", "")
				}
			}
		}
	} else {
		// If notifications are disabled (or nil), ensure the client isn't sending useless metadata
		if hasNotificationMetadata {
			sl.ReportError(req.NotificationMetadata, "NotificationMetadata", "notificationMetadata", "forbidden_when_notifications_disabled", "")
		}
	}
}

func validateConcurrencyControl(sl validator.StructLevel, req ChoreReq) {
	if req.UpdatedAt != nil {
		// Allow a 30-second buffer for slight clock skew between client and server
		cooldown := time.Second * 30
		maxAllowedTime := time.Now().UTC().Add(cooldown)

		if req.UpdatedAt.After(maxAllowedTime) {
			sl.ReportError(req.UpdatedAt, "UpdatedAt", "updatedAt", "cannot_be_in_future", "")
		}
	}
}

func validateRollingLogic(sl validator.StructLevel, req ChoreReq) {
	// IsRolling depends on having a set due date to calculate the shift
	if req.IsRolling != nil && *req.IsRolling {
		if req.NextDueDate == nil {
			sl.ReportError(req.IsRolling, "IsRolling", "isRolling", "requires_next_due_date", "")
		}
	}
}
