package model

import (
	"encoding/json"
	"time"
)

// Active period types: when a recurring chore is live.
const (
	PeriodAlways   = "always"
	PeriodStarts   = "starts"
	PeriodSeasonal = "seasonal"
)

// Stop repeating types: when a recurring chore finishes for good.
const (
	StopNever       = "never"
	StopCompletions = "completions"
	StopDate        = "date"
)

// ScheduleBounds optionally narrows a recurrence on two independent axes. Both
// are optional and compose freely. All dates are inclusive civil dates in the
// recurrence timezone, never UTC instants.
type ScheduleBounds struct {
	ActivePeriod  *ActivePeriod  `json:"activePeriod,omitempty"`
	StopRepeating *StopRepeating `json:"stopRepeating,omitempty"`
}

// ActivePeriod carries only the fields its Type owns: "always" carries none,
// "starts" a YYYY-MM-DD Start, "seasonal" an MM-DD Start and End.
type ActivePeriod struct {
	Type  string `json:"type"`
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// StopRepeating carries only the fields its Type owns: "never" none,
// "completions" a lifetime CompletionLimit, "date" an inclusive YYYY-MM-DD Date.
type StopRepeating struct {
	Type            string `json:"type"`
	CompletionLimit *int   `json:"completionLimit,omitempty"`
	Date            string `json:"date,omitempty"`
}

// UnmarshalJSON also accepts the pre-split shape, where a single `mode`
// (none/absolute/annual/occurrences) fused both axes. Old mobile clients still
// send it, so it is normalized on write and not only on read.
func (s *ScheduleBounds) UnmarshalJSON(data []byte) error {
	var raw struct {
		ActivePeriod    *ActivePeriod  `json:"activePeriod"`
		StopRepeating   *StopRepeating `json:"stopRepeating"`
		Mode            *string        `json:"mode"`
		Start           string         `json:"start"`
		End             string         `json:"end"`
		CompletionLimit *int           `json:"completionLimit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Mode == nil {
		s.ActivePeriod = raw.ActivePeriod
		s.StopRepeating = raw.StopRepeating
		return nil
	}
	*s = boundsFromLegacy(*raw.Mode, raw.Start, raw.End, raw.CompletionLimit)
	return nil
}

func boundsFromLegacy(mode, start, end string, limit *int) ScheduleBounds {
	// An unrecognized mode is carried through so it still fails validation.
	periodType := mode
	switch mode {
	case "none", "occurrences":
		periodType = PeriodAlways
	case "absolute":
		periodType = PeriodStarts
	case "annual":
		periodType = PeriodSeasonal
	}

	period := &ActivePeriod{Type: periodType}
	switch periodType {
	case PeriodStarts:
		period.Start = start
	case PeriodSeasonal:
		period.Start = start
		period.End = end
	}

	switch {
	case mode == "absolute" && end != "":
		// Old editors never wrote an end and a limit together, so preferring
		// the end date loses nothing in practice.
		return ScheduleBounds{ActivePeriod: period, StopRepeating: &StopRepeating{Type: StopDate, Date: end}}
	case limit != nil:
		return ScheduleBounds{ActivePeriod: period, StopRepeating: &StopRepeating{Type: StopCompletions, CompletionLimit: limit}}
	}
	return ScheduleBounds{ActivePeriod: period, StopRepeating: &StopRepeating{Type: StopNever}}
}

// SupportsRecurrenceBounds reports whether a frequency type has a recurrence
// sequence for bounds to narrow.
func SupportsRecurrenceBounds(t FrequencyType) bool {
	switch t {
	case FrequencyTypeOnce, FrequencyTypeNoRepeat, FrequencyTypeTrigger, FrequencyTypeAlways:
		return false
	}
	return true
}

// Bounds returns the schedule bounds, or nil when there are none to enforce.
func (f *FrequencyMetadata) Bounds() *ScheduleBounds {
	if f == nil {
		return nil
	}
	return f.ScheduleBounds
}

// CompletionLimit returns the lifetime completion limit, or nil when the chore
// does not stop on a count. A "completions" stop without a limit is an
// incomplete selection the editor permits, and means no stop condition.
func (f *FrequencyMetadata) CompletionLimit() *int {
	bounds := f.Bounds()
	if bounds == nil || bounds.StopRepeating == nil {
		return nil
	}
	if bounds.StopRepeating.Type != StopCompletions {
		return nil
	}
	return bounds.StopRepeating.CompletionLimit
}

// StopDay returns the inclusive stop date as a comparable YYYYMMDD number.
func (s *ScheduleBounds) StopDay() (int, bool) {
	if s == nil || s.StopRepeating == nil || s.StopRepeating.Type != StopDate {
		return 0, false
	}
	return ParseCivilDate(s.StopRepeating.Date)
}

// InActivePeriod reports whether a civil date — t read in the recurrence
// timezone — falls inside the active period.
func (s *ScheduleBounds) InActivePeriod(t time.Time) bool {
	if s == nil || s.ActivePeriod == nil {
		return true
	}
	switch s.ActivePeriod.Type {
	case PeriodStarts:
		start, ok := ParseCivilDate(s.ActivePeriod.Start)
		return !ok || CivilDay(t) >= start
	case PeriodSeasonal:
		// A missing endpoint means Jan 1 / Dec 31, and an end before its start
		// wraps across New Year.
		start, ok := ParseMonthDay(s.ActivePeriod.Start)
		if !ok {
			start = 101
		}
		end, ok := ParseMonthDay(s.ActivePeriod.End)
		if !ok {
			end = 1231
		}
		day := CivilMonthDay(t)
		if start <= end {
			return day >= start && day <= end
		}
		return day >= start || day <= end
	default:
		return true
	}
}

// CivilDay renders a date as a comparable YYYYMMDD number, dropping the clock
// time. Callers pass t already read in the recurrence timezone.
func CivilDay(t time.Time) int {
	year, month, day := t.Date()
	return year*10000 + int(month)*100 + day
}

// CivilMonthDay renders a date as a comparable MMDD number.
func CivilMonthDay(t time.Time) int {
	_, month, day := t.Date()
	return int(month)*100 + day
}

// ParseCivilDate parses a YYYY-MM-DD date into a comparable YYYYMMDD number.
func ParseCivilDate(value string) (int, bool) {
	if len(value) != 10 {
		return 0, false
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0, false
	}
	return CivilDay(t), true
}

// ParseMonthDay parses an MM-DD seasonal endpoint into a comparable MMDD
// number. Feb 29 is valid and is never clamped: it simply matches no date in a
// non-leap year.
func ParseMonthDay(value string) (int, bool) {
	if len(value) != 5 {
		return 0, false
	}
	// 2000 is a leap year, so 02-29 parses.
	t, err := time.Parse("2006-01-02", "2000-"+value)
	if err != nil {
		return 0, false
	}
	return CivilMonthDay(t), true
}
