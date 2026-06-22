package chore

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

type scheduleTest struct {
	name          string
	chore         chModel.Chore
	completedDate time.Time
	want          *time.Time
	wantErr       bool
	wantErrMsg    string
}

var utc = time.UTC

// dt is a terse UTC time builder for test expectations.
func dt(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, utc)
}

// TestScheduleNextDueDateBasic covers bare interval recurrences (no by-rules).
func TestScheduleNextDueDateBasic(t *testing.T) {
	tests := []scheduleTest{
		{
			name: "Hourly every 6 hours",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeHourly,
				Frequency:     6,
				NextDueDate:   timePtr(dt(2025, 1, 2, 18, 0)),
			},
			want: timePtr(dt(2025, 1, 3, 0, 0)),
		},
		{
			name: "Daily every 1",
			chore: chModel.Chore{
				FrequencyType:       chModel.FrequencyTypeDaily,
				Frequency:           1,
				NextDueDate:         timePtr(dt(2025, 1, 2, 18, 30)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{Time: "2025-01-02T18:30:00Z"},
			},
			want: timePtr(dt(2025, 1, 3, 18, 30)),
		},
		{
			name: "Daily every 2",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDaily,
				Frequency:     2,
				NextDueDate:   timePtr(dt(2025, 1, 2, 18, 0)),
			},
			want: timePtr(dt(2025, 1, 4, 18, 0)),
		},
		{
			name: "Weekly bare (every 7 days on the due weekday)",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeWeekly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 2, 10, 0)), // Thursday
			},
			want: timePtr(dt(2025, 1, 9, 10, 0)),
		},
		{
			name: "Monthly bare every 2 months",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     2,
				NextDueDate:   timePtr(dt(2025, 1, 15, 10, 0)),
			},
			want: timePtr(dt(2025, 3, 15, 10, 0)),
		},
		{
			name: "Yearly bare",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeYearly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 2, 10, 9, 0)),
			},
			want: timePtr(dt(2026, 2, 10, 9, 0)),
		},
	}
	executeTestTable(t, tests)
}

// TestScheduleNextDueDateWeekly covers weekly weekday selection.
func TestScheduleNextDueDateWeekly(t *testing.T) {
	tests := []scheduleTest{
		{
			name: "Weekly on Monday from a Thursday",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeWeekly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 2, 10, 0)), // Thursday
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days: []*string{jsonPtr("monday")},
					Time: "2025-01-06T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 1, 6, 10, 0)), // next Monday
		},
	}
	executeTestTable(t, tests)
}

// TestScheduleNextDueDateMonthly covers Apple "Each" and "On the" monthly modes.
func TestScheduleNextDueDateMonthly(t *testing.T) {
	// Jan 2025: Sun 5,12,19,26 | Fri 3,10,17,24,31 | Jan 1 is Wed
	tests := []scheduleTest{
		{
			name: "First Sunday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 5, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:     []*string{jsonPtr("sunday")},
					SetPos:   []int{1},
					DayToken: jsonPtr(chModel.DayTokenSpecific),
					Time:     "2025-01-05T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 2, 2, 10, 0)),
		},
		{
			name: "Every 2 months on the first Sunday",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     2,
				NextDueDate:   timePtr(dt(2025, 1, 5, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:   []*string{jsonPtr("sunday")},
					SetPos: []int{1},
					Time:   "2025-01-05T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 3, 2, 10, 0)),
		},
		{
			name: "Last Friday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 31, 17, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:   []*string{jsonPtr("friday")},
					SetPos: []int{-1},
					Time:   "2025-01-31T17:00:00Z",
				},
			},
			want: timePtr(dt(2025, 2, 28, 17, 0)),
		},
		{
			name: "Next-to-last Sunday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 1, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:   []*string{jsonPtr("sunday")},
					SetPos: []int{-2},
					Time:   "2025-01-01T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 1, 19, 10, 0)),
		},
		{
			name: "First weekday of month (day token)",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 1, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					DayToken: jsonPtr(chModel.DayTokenWeekday),
					SetPos:   []int{1},
					Time:     "2025-01-01T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 2, 3, 10, 0)), // Feb 1 Sat, 2 Sun, 3 Mon
		},
		{
			name: "Each: 1st and 15th",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeMonthly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 1, 1, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					MonthDays: []int{1, 15},
					Time:      "2025-01-01T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 1, 15, 10, 0)),
		},
	}
	executeTestTable(t, tests)
}

// TestScheduleNextDueDateYearly covers yearly month selection with/without ordinal.
func TestScheduleNextDueDateYearly(t *testing.T) {
	tests := []scheduleTest{
		{
			name: "First Sunday of March every 2 years",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeYearly,
				Frequency:     2,
				NextDueDate:   timePtr(dt(2025, 3, 2, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Months: []*string{jsonPtr("march")},
					Days:   []*string{jsonPtr("sunday")},
					SetPos: []int{1},
					Time:   "2025-03-02T10:00:00Z",
				},
			},
			want: timePtr(dt(2027, 3, 7, 10, 0)),
		},
		{
			name: "Yearly in March & June on the start day (no ordinal)",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeYearly,
				Frequency:     1,
				NextDueDate:   timePtr(dt(2025, 3, 21, 10, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Months: []*string{jsonPtr("march"), jsonPtr("june")},
					Time:   "2025-03-21T10:00:00Z",
				},
			},
			want: timePtr(dt(2025, 6, 21, 10, 0)),
		},
	}
	executeTestTable(t, tests)
}

// TestScheduleNextDueDateRolling verifies rolling chores re-anchor at completion.
func TestScheduleNextDueDateRolling(t *testing.T) {
	tests := []scheduleTest{
		{
			name: "Daily rolling anchors at completion + metadata time",
			chore: chModel.Chore{
				FrequencyType:       chModel.FrequencyTypeDaily,
				Frequency:           1,
				IsRolling:           true,
				NextDueDate:         timePtr(dt(2025, 1, 1, 18, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{Time: "2025-01-01T18:00:00Z"},
			},
			completedDate: dt(2025, 6, 10, 12, 0),
			want:          timePtr(dt(2025, 6, 11, 18, 0)),
		},
	}
	executeTestTable(t, tests)
}

func TestScheduleNextDueDateSpecial(t *testing.T) {
	now := dt(2025, 1, 2, 0, 15)
	tests := []scheduleTest{
		{
			name:          "Once -> no next due",
			chore:         chModel.Chore{FrequencyType: chModel.FrequencyTypeOnce},
			completedDate: now,
			want:          nil,
		},
		{
			name:          "Trigger -> no next due",
			chore:         chModel.Chore{FrequencyType: chModel.FrequencyTypeTrigger},
			completedDate: now,
			want:          nil,
		},
		{
			name:          "Invalid frequency type",
			chore:         chModel.Chore{FrequencyType: "invalid"},
			completedDate: now,
			wantErr:       true,
			wantErrMsg:    "invalid frequency type: invalid",
		},
	}
	executeTestTable(t, tests)
}

func executeTestTable(t *testing.T, tests []scheduleTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduleNextDueDate(context.TODO(), &tt.chore, tt.completedDate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("scheduleNextDueDate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if err.Error() != tt.wantErrMsg {
					t.Errorf("scheduleNextDueDate() error message = %q, want %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if !equalTime(got, tt.want) {
				t.Errorf("scheduleNextDueDate() = %v, want %v", fmtTime(got), fmtTime(tt.want))
			}
		})
	}
}

func equalTime(t1, t2 *time.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(*t2)
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "<nil>"
	}
	return t.UTC().Format(time.RFC3339)
}

func timePtr(t time.Time) *time.Time { return &t }

func jsonPtr(s string) *string { return &s }

func intPtr(i int) *int { return &i }
