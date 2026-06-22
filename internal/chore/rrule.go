package chore

import (
	"fmt"
	"strings"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/teambition/rrule-go"
)

// weekdayByName maps lowercase weekday names to rrule weekdays.
var weekdayByName = map[string]rrule.Weekday{
	"monday":    rrule.MO,
	"tuesday":   rrule.TU,
	"wednesday": rrule.WE,
	"thursday":  rrule.TH,
	"friday":    rrule.FR,
	"saturday":  rrule.SA,
	"sunday":    rrule.SU,
}

// monthByName maps lowercase month names to their 1-based number.
var monthByName = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4,
	"may": 5, "june": 6, "july": 7, "august": 8,
	"september": 9, "october": 10, "november": 11, "december": 12,
}

// freqByType maps a chore FrequencyType to an rrule.Frequency. The boolean is
// false for non-RRULE-schedulable types (once/trigger/no_repeat/adaptive).
func freqByType(ft chModel.FrequencyType) (rrule.Frequency, bool) {
	switch ft {
	case chModel.FrequencyTypeHourly:
		return rrule.HOURLY, true
	case chModel.FrequencyTypeDaily:
		return rrule.DAILY, true
	case chModel.FrequencyTypeWeekly:
		return rrule.WEEKLY, true
	case chModel.FrequencyTypeMonthly:
		return rrule.MONTHLY, true
	case chModel.FrequencyTypeYearly:
		return rrule.YEARLY, true
	default:
		return 0, false
	}
}

// isRRuleSchedulable reports whether the chore's frequency is computed via RRULE.
func isRRuleSchedulable(ft chModel.FrequencyType) bool {
	_, ok := freqByType(ft)
	return ok
}

// buildROption translates a chore's FrequencyType + FrequencyMetadata + interval
// into an rrule.ROption anchored at dtstart. dtstart carries both the series phase
// (for INTERVAL) and the time-of-day/timezone of generated occurrences.
func buildROption(chore *chModel.Chore, dtstart time.Time) (rrule.ROption, error) {
	freq, ok := freqByType(chore.FrequencyType)
	if !ok {
		return rrule.ROption{}, fmt.Errorf("frequency type %q is not RRULE-schedulable", chore.FrequencyType)
	}

	opt := rrule.ROption{
		Freq:     freq,
		Dtstart:  dtstart,
		Interval: chore.Frequency,
	}
	if opt.Interval < 1 {
		opt.Interval = 1
	}

	meta := chore.FrequencyMetadataV2
	if meta == nil {
		// Bare hourly/daily/weekly/monthly/yearly recurrence anchored at dtstart.
		return opt, nil
	}

	switch freq {
	case rrule.WEEKLY:
		if len(meta.Days) > 0 {
			wd, err := weekdays(meta.Days)
			if err != nil {
				return rrule.ROption{}, err
			}
			opt.Byweekday = wd
		}

	case rrule.MONTHLY:
		if len(meta.MonthDays) > 0 {
			// "Each" mode: specific day numbers of the month.
			opt.Bymonthday = append([]int(nil), meta.MonthDays...)
		} else if len(meta.SetPos) > 0 {
			// "On the" mode: ordinal weekday(s).
			wd, err := ordinalWeekdays(meta)
			if err != nil {
				return rrule.ROption{}, err
			}
			opt.Byweekday = wd
			opt.Bysetpos = append([]int(nil), meta.SetPos...)
		}

	case rrule.YEARLY:
		if len(meta.Months) > 0 {
			mo, err := months(meta.Months)
			if err != nil {
				return rrule.ROption{}, err
			}
			opt.Bymonth = mo
		}
		if len(meta.SetPos) > 0 {
			// Optional "On the" ordinal block scoped to the selected months.
			wd, err := ordinalWeekdays(meta)
			if err != nil {
				return rrule.ROption{}, err
			}
			opt.Byweekday = wd
			opt.Bysetpos = append([]int(nil), meta.SetPos...)
		}
		// With months but no ordinal block, rrule defaults Bymonthday to the
		// dtstart day-of-month, i.e. the start date's day within each month.

	case rrule.DAILY, rrule.HOURLY:
		// No additional selectors.
	}

	return opt, nil
}

// ordinalWeekdays expands the DayToken + Days of an "On the" ordinal rule into a
// plain (non-ordinal) weekday set; the ordinal is applied separately via Bysetpos.
func ordinalWeekdays(meta *chModel.FrequencyMetadata) ([]rrule.Weekday, error) {
	token := chModel.DayTokenSpecific
	if meta.DayToken != nil && *meta.DayToken != "" {
		token = *meta.DayToken
	}
	switch token {
	case chModel.DayTokenDay:
		return []rrule.Weekday{rrule.MO, rrule.TU, rrule.WE, rrule.TH, rrule.FR, rrule.SA, rrule.SU}, nil
	case chModel.DayTokenWeekday:
		return []rrule.Weekday{rrule.MO, rrule.TU, rrule.WE, rrule.TH, rrule.FR}, nil
	case chModel.DayTokenWeekend:
		return []rrule.Weekday{rrule.SA, rrule.SU}, nil
	default: // specific
		if len(meta.Days) == 0 {
			return nil, fmt.Errorf("ordinal rule with dayToken 'specific' requires at least one day")
		}
		return weekdays(meta.Days)
	}
}

// weekdays converts day-name pointers into plain rrule weekdays.
func weekdays(days []*string) ([]rrule.Weekday, error) {
	out := make([]rrule.Weekday, 0, len(days))
	for _, d := range days {
		if d == nil {
			continue
		}
		wd, ok := weekdayByName[strings.ToLower(strings.TrimSpace(*d))]
		if !ok {
			return nil, fmt.Errorf("invalid weekday: %q", *d)
		}
		out = append(out, wd)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid weekdays provided")
	}
	return out, nil
}

// months converts month-name pointers into 1-based month numbers.
func months(in []*string) ([]int, error) {
	out := make([]int, 0, len(in))
	for _, m := range in {
		if m == nil {
			continue
		}
		n, ok := monthByName[strings.ToLower(strings.TrimSpace(*m))]
		if !ok {
			return nil, fmt.Errorf("invalid month: %q", *m)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid months provided")
	}
	return out, nil
}
