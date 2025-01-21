package chore

import (
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

func TestScheduleNextDueDateBasicTests(t *testing.T) {
	// location, err := time.LoadLocation("America/New_York")
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	now := time.Date(2025, 1, 2, 0, 15, 0, 0, location)
	tests := []scheduleTest{
		{
			name: "Daily",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDaily,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(now.AddDate(0, 0, 1)),
		},
		{
			name: "Daily - (IsRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDaily,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now.AddDate(0, 1, 0),
			want:          timePtr(now.AddDate(0, 1, 1)),
		},

		{
			name: "Weekly",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeWeekly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(now.AddDate(0, 0, 7)),
		},
		{
			name: "Weekly - (IsRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeWeekly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now.AddDate(1, 0, 0),
			want:          timePtr(now.AddDate(1, 0, 7)),
		},
		{
			name: "Monthly",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeMonthly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(now.AddDate(0, 1, 0)),
		},
		{
			name: "Monthly - (IsRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeMonthly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now.AddDate(0, 0, 2),
			want:          timePtr(now.AddDate(0, 1, 2)),
		},
		{
			name: "Yearly",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeYearly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(now.AddDate(1, 0, 0)),
		},
		{
			name: "Yearly - (IsRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeYearly,
				FrequencyMetadata: jsonPtr(`{"time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now.AddDate(0, 0, 2),
			want:          timePtr(now.AddDate(1, 0, 2)),
		},
	}
	executeTestTable(t, tests)
}

func TestScheduleNextDueDateInterval(t *testing.T) {
	// location, err := time.LoadLocation("America/New_York")
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	now := time.Date(2025, 1, 2, 0, 15, 0, 0, location)
	tests := []scheduleTest{
		{
			name: "Interval - 2 Days",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         2,
				FrequencyMetadata: jsonPtr(`{"unit": "days","time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 0, 2)).Add(14*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 4 Weeks",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         4,
				FrequencyMetadata: jsonPtr(`{"unit": "weeks","time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 0, 4*7)).Add(14*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 3 Months",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         3,
				FrequencyMetadata: jsonPtr(`{"unit": "months","time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 3, 0)).Add(14*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 2 Years",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         2,
				FrequencyMetadata: jsonPtr(`{"unit": "years","time":"2024-07-07T14:30:00-04:00"}`),
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(2, 0, 0)).Add(14*time.Hour + 30*time.Minute)),
		},
	}
	executeTestTable(t, tests)
}

func TestScheduleNextDueDateDayOfWeek(t *testing.T) {
	// location, err := time.LoadLocation("America/New_York")
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	now := time.Date(2025, 1, 2, 0, 15, 0, 0, location)
	tests := []scheduleTest{
		{
			name: "Days of the week - next Monday",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,

				FrequencyMetadata: jsonPtr(`{"days": ["monday"], "time": "2025-01-20T18:00:00-05:00"}`),
			},
			completedDate: now,
			want: func() *time.Time {
				// Calculate next Monday at 18:00 EST
				nextMonday := now.AddDate(0, 0, (int(time.Monday)-int(now.Weekday())+7)%7)
				nextMonday = truncateToDay(nextMonday).Add(18*time.Hour + 0*time.Minute)
				return &nextMonday
			}(),
		},
		{
			name: "Days of the week - next Monday(IsRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDayOfTheWeek,
				IsRolling:         true,
				FrequencyMetadata: jsonPtr(`{"days": ["monday"], "time": "2025-01-20T18:00:00-05:00"}`),
			},

			completedDate: now.AddDate(0, 1, 0),
			want: func() *time.Time {
				// Calculate next Thursday at 18:00 EST
				completedDate := now.AddDate(0, 1, 0)
				nextMonday := completedDate.AddDate(0, 0, (int(time.Monday)-int(completedDate.Weekday())+7)%7)
				nextMonday = truncateToDay(nextMonday).Add(18*time.Hour + 0*time.Minute)
				return &nextMonday
			}(),
		},
	}
	executeTestTable(t, tests)
}

func TestScheduleNextDueDateDayOfMonth(t *testing.T) {
	// location, err := time.LoadLocation("America/New_York")
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	now := time.Date(2025, 1, 2, 0, 15, 0, 0, location)
	tests := []scheduleTest{
		{
			name: "Day of the month - 15th of January",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDayOfTheMonth,
				Frequency:         15,
				FrequencyMetadata: jsonPtr(`{ "unit": "days", "time": "2025-01-20T18:00:00-05:00", "days": [], "months": [ "january" ] }`),
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 15, 18, 0, 0, 0, location)),
		},
		{
			name: "Day of the month - 15th of January(isRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDayOfTheMonth,
				Frequency:         15,
				FrequencyMetadata: jsonPtr(`{ "unit": "days", "time": "2025-01-20T18:00:00-05:00", "days": [], "months": [ "january" ] }`),
			},
			completedDate: now.AddDate(1, 1, 0),
			want:          timePtr(time.Date(2027, 1, 15, 18, 0, 0, 0, location)),
		},
	}
	executeTestTable(t, tests)

}

func TestScheduleNextDueDateErrors(t *testing.T) {
	// location, err := time.LoadLocation("America/New_York")
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	now := time.Date(2025, 1, 2, 0, 15, 0, 0, location)
	tests := []scheduleTest{
		{
			name: "Invalid frequency Metadata",
			chore: chModel.Chore{
				FrequencyType:     "invalid",
				FrequencyMetadata: jsonPtr(``),
			},
			completedDate: now,
			wantErr:       true,
			wantErrMsg:    "error unmarshalling frequency metadata: unexpected end of JSON input",
		},
	}
	executeTestTable(t, tests)
}
func TestScheduleNextDueDate(t *testing.T) {

}

func executeTestTable(t *testing.T, tests []scheduleTest) {

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduleNextDueDate(&tt.chore, tt.completedDate)
			if (err != nil) != tt.wantErr {
				t.Errorf("testcase: %s", tt.name)
				t.Errorf("scheduleNextDueDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err.Error() != tt.wantErrMsg {
					t.Errorf("testcase: %s", tt.name)
					t.Errorf("scheduleNextDueDate() error message = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if !equalTime(got, tt.want) {
				t.Errorf("testcase: %s", tt.name)
				t.Errorf("scheduleNextDueDate() = %v, want %v", got, tt.want)

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

func timePtr(t time.Time) *time.Time {
	return &t
}

func jsonPtr(s string) *string {
	return &s
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
