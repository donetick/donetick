package chore

import (
	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

func ChoreReqStructLevelValidation(sl validator.StructLevel) {
	req := sl.Current().Interface().(ChoreReq)
	hasMetadata := req.FrequencyMetadata != nil

	switch req.FrequencyType {
	case chModel.FrequencyTypeInterval:
		if !hasMetadata || req.FrequencyMetadata.Unit == nil {
			sl.ReportError(req.FrequencyMetadata, "Unit", "unit", "required_with_interval", "")
		}

	case chModel.FrequencyTypeDayOfTheWeek:
		if !hasMetadata || req.FrequencyMetadata.Days == nil {
			sl.ReportError(req.FrequencyMetadata, "Days", "days", "required_with_day_of_week", "")
		}
		if !hasMetadata || req.FrequencyMetadata.WeekPattern == nil {
			sl.ReportError(req.FrequencyMetadata, "WeekPattern", "weekPattern", "required_with_day_of_week", "")
		}

	case chModel.FrequencyTypeDayOfTheMonth:
		if !hasMetadata || req.FrequencyMetadata.Months == nil {
			sl.ReportError(req.FrequencyMetadata, "Months", "months", "required_with_day_of_month", "")
		}
		if *req.Frequency <= 0 || *req.Frequency > 31 {
			sl.ReportError(req.Frequency, "Frequency", "frequency", "valid_day_of_month", "")
		}
	}
}
