package model

import (
	"encoding/json"
	"testing"
)

func TestScheduleBoundsUnmarshalLegacyMode(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		period  ActivePeriod
		stop    StopRepeating
	}{
		{
			name:    "none becomes always and never",
			payload: `{"mode":"none"}`,
			period:  ActivePeriod{Type: PeriodAlways},
			stop:    StopRepeating{Type: StopNever},
		},
		{
			name:    "absolute splits into starts and a stop date",
			payload: `{"mode":"absolute","start":"2026-03-15","end":"2026-05-01"}`,
			period:  ActivePeriod{Type: PeriodStarts, Start: "2026-03-15"},
			stop:    StopRepeating{Type: StopDate, Date: "2026-05-01"},
		},
		{
			name:    "absolute with both an end and a limit keeps the end date",
			payload: `{"mode":"absolute","start":"2026-03-15","end":"2026-05-01","completionLimit":20}`,
			period:  ActivePeriod{Type: PeriodStarts, Start: "2026-03-15"},
			stop:    StopRepeating{Type: StopDate, Date: "2026-05-01"},
		},
		{
			name:    "annual becomes seasonal",
			payload: `{"mode":"annual","start":"11-01","end":"02-28"}`,
			period:  ActivePeriod{Type: PeriodSeasonal, Start: "11-01", End: "02-28"},
			stop:    StopRepeating{Type: StopNever},
		},
		{
			name:    "occurrences becomes always plus a completions stop",
			payload: `{"mode":"occurrences","completionLimit":20}`,
			period:  ActivePeriod{Type: PeriodAlways},
			stop:    StopRepeating{Type: StopCompletions, CompletionLimit: intPtr(20)},
		},
		{
			name:    "an unrecognized mode is carried through so it still fails validation",
			payload: `{"mode":"bogus"}`,
			period:  ActivePeriod{Type: "bogus"},
			stop:    StopRepeating{Type: StopNever},
		},
		{
			name:    "the two-axis shape is taken as is",
			payload: `{"activePeriod":{"type":"starts","start":"2026-03-15"},"stopRepeating":{"type":"completions","completionLimit":5}}`,
			period:  ActivePeriod{Type: PeriodStarts, Start: "2026-03-15"},
			stop:    StopRepeating{Type: StopCompletions, CompletionLimit: intPtr(5)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bounds ScheduleBounds
			if err := json.Unmarshal([]byte(tt.payload), &bounds); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if *bounds.ActivePeriod != tt.period {
				t.Errorf("activePeriod = %+v, want %+v", *bounds.ActivePeriod, tt.period)
			}
			got := *bounds.StopRepeating
			if got.Type != tt.stop.Type || got.Date != tt.stop.Date {
				t.Errorf("stopRepeating = %+v, want %+v", got, tt.stop)
			}
			switch {
			case got.CompletionLimit == nil && tt.stop.CompletionLimit != nil,
				got.CompletionLimit != nil && tt.stop.CompletionLimit == nil:
				t.Errorf("completionLimit = %v, want %v", got.CompletionLimit, tt.stop.CompletionLimit)
			case got.CompletionLimit != nil && *got.CompletionLimit != *tt.stop.CompletionLimit:
				t.Errorf("completionLimit = %d, want %d", *got.CompletionLimit, *tt.stop.CompletionLimit)
			}
		})
	}
}

// Legacy payloads are normalized on write too, so what round-trips through the
// JSON column is always the two-axis shape.
func TestScheduleBoundsNormalizedOnWrite(t *testing.T) {
	var metadata FrequencyMetadata
	if err := metadata.Scan(`{"scheduleBounds":{"mode":"annual","start":"06-01","end":"08-31"}}`); err != nil {
		t.Fatalf("scan: %v", err)
	}
	stored, err := metadata.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	want := `{"unit":null,"time":"","timezone":"","weekPattern":null,"scheduleBounds":{"activePeriod":{"type":"seasonal","start":"06-01","end":"08-31"},"stopRepeating":{"type":"never"}}}`
	if string(stored.([]byte)) != want {
		t.Errorf("stored = %s, want %s", stored, want)
	}
}

func TestCompletionLimit(t *testing.T) {
	var nilMetadata *FrequencyMetadata
	if nilMetadata.CompletionLimit() != nil {
		t.Error("nil metadata should carry no limit")
	}
	// An incomplete selection means no stop condition.
	metadata := FrequencyMetadata{ScheduleBounds: &ScheduleBounds{
		StopRepeating: &StopRepeating{Type: StopCompletions},
	}}
	if metadata.CompletionLimit() != nil {
		t.Error("a completions stop without a limit should carry no limit")
	}
	metadata.ScheduleBounds.StopRepeating.CompletionLimit = intPtr(3)
	if got := metadata.CompletionLimit(); got == nil || *got != 3 {
		t.Errorf("CompletionLimit() = %v, want 3", got)
	}
}

func TestParseMonthDay(t *testing.T) {
	for _, value := range []string{"02-29", "01-01", "12-31"} {
		if _, ok := ParseMonthDay(value); !ok {
			t.Errorf("ParseMonthDay(%q) should be valid", value)
		}
	}
	for _, value := range []string{"", "2-29", "02-30", "13-01", "2026-02-01"} {
		if _, ok := ParseMonthDay(value); ok {
			t.Errorf("ParseMonthDay(%q) should be invalid", value)
		}
	}
}

func TestParseCivilDate(t *testing.T) {
	for _, value := range []string{"2024-02-29", "2026-12-31"} {
		if _, ok := ParseCivilDate(value); !ok {
			t.Errorf("ParseCivilDate(%q) should be valid", value)
		}
	}
	for _, value := range []string{"", "2026-02-29", "2026-13-01", "2026-1-01", "02-28"} {
		if _, ok := ParseCivilDate(value); ok {
			t.Errorf("ParseCivilDate(%q) should be invalid", value)
		}
	}
}

func intPtr(i int) *int {
	return &i
}
