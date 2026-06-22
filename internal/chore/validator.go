package chore

import (
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

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
	case chModel.FrequencyTypeHourly, chModel.FrequencyTypeDaily:
		// "Every X" interval only; Frequency must be a positive count when provided.
		if req.Frequency != nil && *req.Frequency < 1 {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "valid_interval", "")
		}

	case chModel.FrequencyTypeWeekly:
		// Weekly requires at least one selected weekday.
		if !hasMetadata || len(req.FrequencyMetadata.Days) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Days", "days", "required_with_weekly", "")
		}

	case chModel.FrequencyTypeMonthly:
		if !hasMetadata {
			sl.ReportError(req.FrequencyMetadata, "FrequencyMetadata", "frequencyMetadata", "required_with_monthly", "")
			return
		}
		m := req.FrequencyMetadata
		hasEach := len(m.MonthDays) > 0 // "Each" mode: specific day numbers
		hasOnThe := len(m.SetPos) > 0   // "On the" mode: ordinal weekday(s)
		if !hasEach && !hasOnThe {
			sl.ReportError(m, "MonthDays", "monthDays", "required_with_monthly", "")
		}
		if hasEach {
			for _, d := range m.MonthDays {
				if d < 1 || d > 31 {
					sl.ReportError(m, "MonthDays", "monthDays", "valid_month_day", "")
					break
				}
			}
		}
		if hasOnThe {
			validateOrdinalBlock(sl, m)
		}

	case chModel.FrequencyTypeYearly:
		// Yearly requires at least one selected month, with an optional ordinal block.
		if !hasMetadata || len(req.FrequencyMetadata.Months) == 0 {
			sl.ReportError(req.FrequencyMetadata, "Months", "months", "required_with_yearly", "")
			return
		}
		if len(req.FrequencyMetadata.SetPos) > 0 {
			validateOrdinalBlock(sl, req.FrequencyMetadata)
		}
	}
}

// validateOrdinalBlock validates an "On the …" ordinal rule (SetPos + DayToken/Days).
func validateOrdinalBlock(sl validator.StructLevel, m *chModel.FrequencyMetadata) {
	for _, p := range m.SetPos {
		// Allowed ordinals: 1..5, -1 (last), -2 (next to last).
		if p == 0 || p < -2 || p > 5 {
			sl.ReportError(m, "SetPos", "setPos", "valid_set_pos", "")
			break
		}
	}
	token := chModel.DayTokenSpecific
	if m.DayToken != nil && *m.DayToken != "" {
		token = *m.DayToken
	}
	if token == chModel.DayTokenSpecific && len(m.Days) == 0 {
		sl.ReportError(m, "Days", "days", "required_with_ordinal", "")
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
