package chore

import (
	"context"
	"fmt"
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
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Time: "2024-07-07T14:30:00-04:00", // for backward compatibility
				},
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
				FrequencyMetadataV2: &chModel.FrequencyMetadata{ // for backward compatibility
					Time: "2024-07-07T14:30:00-04:00",
					Unit: jsonPtr("days"),
				},
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 0, 2)).Add(18*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 4 Weeks",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         4,
				FrequencyMetadata: jsonPtr(`{"unit": "weeks","time":"2024-07-07T14:30:00-04:00"}`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{ // for backward compatibility
					Time: "2024-07-07T14:30:00-04:00",
					Unit: jsonPtr("weeks"), // this is needed for interval calculations
				},
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 0, 4*7)).Add(18*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 3 Months",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         3,
				FrequencyMetadata: jsonPtr(`{"unit": "months","time":"2024-07-07T14:30:00-04:00"}`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{ // for backward compatibility
					Time: "2024-07-07T14:30:00-04:00", // this is needed for interval calculations
					Unit: jsonPtr("months"),
				},
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(0, 3, 0)).Add(18*time.Hour + 30*time.Minute)),
		},
		{
			name: "Interval - 2 Years",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeInterval,
				Frequency:         2,
				FrequencyMetadata: jsonPtr(`{"unit": "years","time":"2024-07-07T14:30:00-04:00"}`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{ // for backward compatibility
					Time: "2024-07-07T14:30:00-04:00", // this is needed for interval calculations
					Unit: jsonPtr("years"),
				},
			},
			completedDate: now,
			want:          timePtr(truncateToDay(now.AddDate(2, 0, 0)).Add(18*time.Hour + 30*time.Minute)),
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
				NextDueDate:   timePtr(time.Date(2025, 1, 2, 0, 12, 0, 0, location)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days: []*string{jsonPtr("monday")},
					Time: "2025-01-20T01:00:00-05:00",
				},
			},
			completedDate: now,
			want: func() *time.Time {
				// Calculate next Monday at 18:00 EST
				nextMonday := now.AddDate(0, 0, (int(time.Monday)-int(now.Weekday())+7)%7)
				// print the nextMonday date and time:
				fmt.Println("nextMonday:", nextMonday)
				nextMonday = truncateToDay(nextMonday).Add(6*time.Hour + 0*time.Minute)
				return &nextMonday
			}(),
		},
		// {
		// 	name: "Days of the week - next Monday(IsRolling)",
		// 	chore: chModel.Chore{
		// 		FrequencyType:     chModel.FrequencyTypeDayOfTheWeek,
		// 		IsRolling:         true,
		// 		FrequencyMetadata: jsonPtr(`{"days": ["monday"], "time": "2025-01-20T01:00:00-05:00"}`),
		// 	},

		// 	completedDate: now.AddDate(0, 1, 0),
		// 	want: func() *time.Time {
		// 		// Calculate next Thursday at 18:00 EST
		// 		completedDate := now.AddDate(0, 1, 0)
		// 		nextMonday := completedDate.AddDate(0, 0, (int(time.Monday)-int(completedDate.Weekday())+7)%7)
		// 		nextMonday = truncateToDay(nextMonday).Add(6*time.Hour + 0*time.Minute)
		// 		return &nextMonday
		// 	}(),
		// },
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
				FrequencyMetadata: jsonPtr(`{ "unit": "days", "time": "2025-01-20T14:00:00-05:00", "days": [], "months": [ "january" ] }`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Time: "2025-01-20T14:00:00-05:00",
					Unit: jsonPtr("days"),
					Months: []*string{
						jsonPtr("january"),
					},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 15, 19, 0, 0, 0, location)),
		},
		{
			name: "Day of the month - 15th of January(isRolling)",
			chore: chModel.Chore{
				FrequencyType:     chModel.FrequencyTypeDayOfTheMonth,
				Frequency:         15,
				IsRolling:         true,
				FrequencyMetadata: jsonPtr(`{ "unit": "days", "time": "2025-01-20T02:00:00-05:00", "days": [], "months": [ "january" ] }`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Time: "2025-01-20T02:00:00-05:00", // this is needed for interval calculations
					Unit: jsonPtr("days"),
					Months: []*string{
						jsonPtr("january"),
					},
				},
			},
			completedDate: now.AddDate(1, 1, 0),
			want:          timePtr(time.Date(2027, 1, 15, 7, 0, 0, 0, location)),
		},
		// test if completed before the 15th of the month:
		{
			name: "Day of the month - 15th of January(isRolling)(Completed before due date)",
			chore: chModel.Chore{
				NextDueDate:       timePtr(time.Date(2025, 1, 15, 18, 0, 0, 0, location)),
				FrequencyType:     chModel.FrequencyTypeDayOfTheMonth,
				Frequency:         15,
				IsRolling:         true,
				FrequencyMetadata: jsonPtr(`{ "unit": "days", "time": "2025-01-20T18:00:00-05:00", "days": [], "months": [ "january" ] }`),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Time: "2025-01-20T18:00:00-05:00", // this is needed for interval calculations
					Unit: jsonPtr("days"),             // this is needed for interval calculations
					Months: []*string{
						jsonPtr("january"),
					},
				},
			},
			completedDate: now.AddDate(0, 0, 2),
			want:          timePtr(time.Date(2026, 1, 15, 18, 0, 0, 0, location)),
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
				FrequencyType:       "invalid",
				FrequencyMetadata:   jsonPtr(``),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{},
			},
			completedDate: now,
			wantErr:       true,
			wantErrMsg:    "invalid frequency type: invalid",
		},
	}
	executeTestTable(t, tests)
}
func TestScheduleNextDueDate(t *testing.T) {

}

func executeTestTable(t *testing.T, tests []scheduleTest) {

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduleNextDueDate(context.TODO(), &tt.chore, tt.completedDate)
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

func intPtr(i int) *int {
	return &i
}

func TestScheduleNextDueDateWeekPatterns(t *testing.T) {
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	// January 2025 calendar:
	// Sun Mon Tue Wed Thu Fri Sat
	//           1   2   3   4
	//  5   6   7   8   9  10  11
	// 12  13  14  15  16  17  18
	// 19  20  21  22  23  24  25
	// 26  27  28  29  30  31

	// Starting from Thursday, January 1, 2025
	now := time.Date(2025, 1, 1, 10, 0, 0, 0, location)

	tests := []scheduleTest{
		{
			name: "1st Monday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("monday")},
					Time:        "2025-01-06T10:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					Occurrences: []*int{intPtr(1)},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 6, 10, 0, 0, 0, location)), // First Monday (Jan 6th)
		},
		{
			name: "2nd Tuesday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("tuesday")},
					Time:        "2025-01-14T14:30:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					Occurrences: []*int{intPtr(2)},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 14, 14, 30, 0, 0, location)), // Second Tuesday (Jan 14th)
		},
		{
			name: "1st Friday of quarter",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("friday")},
					Time:        "2025-01-03T09:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfQuarter; return &w }(),
					Occurrences: []*int{intPtr(1)},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 3, 9, 0, 0, 0, location)), // First Friday of Q1
		},
		{
			name: "Every week pattern (default behavior)",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("wednesday")},
					Time:        "2025-01-08T16:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekpatternEveryWeek; return &w }(),
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 8, 16, 0, 0, 0, location)), // Next Wednesday
		},
		{
			name: "No week pattern specified (default behavior)",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days: []*string{jsonPtr("saturday")},
					Time: "2025-01-04T12:00:00Z",
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 4, 12, 0, 0, 0, location)), // Next Saturday
		},
		{
			name: "1st and 3rd Monday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("monday")},
					Time:        "2025-01-03T08:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					Occurrences: []*int{intPtr(1), intPtr(3)},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 6, 8, 0, 0, 0, location)), // First Monday (Jan 6th)
		},
		{
			name: "Last Friday of month",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("friday")},
					Time:        "2025-01-31T17:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					Occurrences: []*int{intPtr(-1)},
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 31, 17, 0, 0, 0, location)), // Last Friday (Jan 31st)
		},
		{
			name: "Error - week_of_month without occurrences",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("monday")},
					Time:        "2025-01-06T10:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					Occurrences: []*int{},
				},
			},
			completedDate: now,
			wantErr:       true,
			wantErrMsg:    "week_of_month requires at least one occurrence",
		},
		{
			name: "Error - week_of_quarter without occurrences",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("friday")},
					Time:        "2025-01-03T09:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfQuarter; return &w }(),
					Occurrences: []*int{},
				},
			},
			completedDate: now,
			wantErr:       true,
			wantErrMsg:    "week_of_quarter requires at least one occurrence",
		},
		{
			name: "Backward compatibility - legacy WeekNumbers",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Days:        []*string{jsonPtr("monday")},
					Time:        "2025-01-06T10:00:00Z",
					WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
					WeekNumbers: []int{1}, // This should still work
				},
			},
			completedDate: now,
			want:          timePtr(time.Date(2025, 1, 6, 10, 0, 0, 0, location)), // First Monday
		},
		// {
		// 	name: "Week of month - 2nd week Tuesday",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("tuesday")},
		// 			Time:        "2025-01-14T14:30:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
		// 			WeekNumbers: []int{2},
		// 		},
		// 	},
		// 	completedDate: now,
		// 	want:          timePtr(time.Date(2025, 1, 14, 14, 30, 0, 0, location)), // Second Tuesday (2nd week)
		// },
		// {
		// 	name: "Week of quarter - 1st week Friday",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("friday")},
		// 			Time:        "2025-01-03T09:00:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfQuarter; return &w }(),
		// 			WeekNumbers: []int{1},
		// 		},
		// 	},
		// 	completedDate: now,
		// 	want:          timePtr(time.Date(2025, 1, 3, 9, 0, 0, 0, location)), // First Friday of Q1
		// },
		// {
		// 	name: "Every week pattern (default behavior)",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("wednesday")},
		// 			Time:        "2025-01-08T16:00:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekpatternEveryWeek; return &w }(),
		// 		},
		// 	},
		// 	completedDate: now,
		// 	want:          timePtr(time.Date(2025, 1, 8, 16, 0, 0, 0, location)), // Next Wednesday
		// },
		// {
		// 	name: "No week pattern specified (default behavior)",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days: []*string{jsonPtr("saturday")},
		// 			Time: "2025-01-04T12:00:00Z",
		// 		},
		// 	},
		// 	completedDate: now,
		// 	want:          timePtr(time.Date(2025, 1, 4, 12, 0, 0, 0, location)), // Next Saturday
		// },
		// {
		// 	name: "Week of month - multiple days and weeks",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("monday"), jsonPtr("friday")},
		// 			Time:        "2025-01-03T08:00:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
		// 			WeekNumbers: []int{1, 3},
		// 		},
		// 	},
		// 	completedDate: now,
		// 	want:          timePtr(time.Date(2025, 1, 3, 8, 0, 0, 0, location)), // First Friday (1st week)
		// },
		// {
		// 	name: "Error - week_of_month without week numbers",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("monday")},
		// 			Time:        "2025-01-06T10:00:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfMonth; return &w }(),
		// 			WeekNumbers: []int{},
		// 		},
		// 	},
		// 	completedDate: now,
		// 	wantErr:       true,
		// 	wantErrMsg:    "week_of_month requires at least one week number",
		// },
		// {
		// 	name: "Error - week_of_quarter without week numbers",
		// 	chore: chModel.Chore{
		// 		FrequencyType: chModel.FrequencyTypeDayOfTheWeek,
		// 		FrequencyMetadataV2: &chModel.FrequencyMetadata{
		// 			Days:        []*string{jsonPtr("friday")},
		// 			Time:        "2025-01-03T09:00:00Z",
		// 			WeekPattern: func() *chModel.Weekpattern { w := chModel.WeekPatternWeekOfQuarter; return &w }(),
		// 			WeekNumbers: []int{},
		// 		},
		// 	},
		// 	completedDate: now,
		// 	wantErr:       true,
		// 	wantErrMsg:    "week_of_quarter requires at least one week number",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduleNextDueDate(context.Background(), &tt.chore, tt.completedDate)
			if tt.wantErr {
				if err == nil {
					t.Errorf("scheduleNextDueDate() expected error but got none")
					return
				}
				if tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
					t.Errorf("scheduleNextDueDate() error = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("scheduleNextDueDate() error = %v", err)
				return
			}
			if !equalTime(got, tt.want) {
				t.Errorf("scheduleNextDueDate() = %v, want %v", got, tt.want)
			}
		})
	}
}
