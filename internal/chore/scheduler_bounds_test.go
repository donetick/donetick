package chore

import (
	"context"
	"testing"
	"time"
	_ "time/tzdata"

	chModel "donetick.com/core/internal/chore/model"
)

func boundedChore(freqType chModel.FrequencyType, tz string, due time.Time, bounds *chModel.ScheduleBounds) chModel.Chore {
	return chModel.Chore{
		FrequencyType: freqType,
		NextDueDate:   &due,
		FrequencyMetadataV2: &chModel.FrequencyMetadata{
			Time:           "2024-07-07T14:30:00Z",
			Timezone:       tz,
			ScheduleBounds: bounds,
		},
	}
}

func starts(date string) *chModel.ActivePeriod {
	return &chModel.ActivePeriod{Type: chModel.PeriodStarts, Start: date}
}

func seasonal(start, end string) *chModel.ActivePeriod {
	return &chModel.ActivePeriod{Type: chModel.PeriodSeasonal, Start: start, End: end}
}

func stopOn(date string) *chModel.StopRepeating {
	return &chModel.StopRepeating{Type: chModel.StopDate, Date: date}
}

func utcDate(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
}

func TestScheduleNextDueDateBounds(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("error loading location: %v", err)
	}

	tests := []scheduleTest{
		{
			name:  "No bounds leaves the next occurrence alone",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0), nil),
			want:  timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
		{
			name: "Starts date dormant until the first eligible occurrence",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15")}),
			want: timePtr(utcDate(2026, time.March, 15, 9, 0)),
		},
		{
			name: "Starts date already passed",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.March, 20, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15")}),
			want: timePtr(utcDate(2026, time.March, 21, 9, 0)),
		},
		{
			name: "Weekday anchor survives the seasonal clamp",
			chore: boundedChore(chModel.FrequencyTypeWeekly, "UTC", utcDate(2026, time.January, 7, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("06-01", "08-31")}),
			want: timePtr(utcDate(2026, time.June, 3, 9, 0)), // still a Wednesday
		},
		{
			name: "Season wrapping New Year",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.June, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("11-01", "02-28")}),
			want: timePtr(utcDate(2026, time.November, 1, 9, 0)),
		},
		{
			name: "Inside a wrapping season in January",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("11-01", "02-28")}),
			want: timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
		{
			name: "Missing seasonal endpoints mean the whole year",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("", "")}),
			want: timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
		{
			name: "Missing seasonal start means January 1",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.June, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("", "02-28")}),
			want: timePtr(utcDate(2027, time.January, 1, 9, 0)),
		},
		{
			name: "Equal seasonal endpoints mean a single day",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("07-04", "07-04")}),
			want: timePtr(utcDate(2026, time.July, 4, 9, 0)),
		},
		{
			name: "February 29 matches only leap years",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("02-29", "02-29")}),
			want: timePtr(utcDate(2028, time.February, 29, 9, 0)),
		},
		{
			name: "Stop date is inclusive",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{StopRepeating: stopOn("2026-01-11")}),
			want: timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
		{
			name: "Stop date passed ends the recurrence",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{StopRepeating: stopOn("2026-01-10")}),
			want: nil,
		},
		{
			name: "A stop date reached while dormant ends the recurrence",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("06-01", "08-31"), StopRepeating: stopOn("2026-03-01")}),
			want: nil,
		},
		{
			name: "An incomplete stop selection is no stop condition",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopDate}}),
			want: timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
		{
			name: "Stop date compares civil dates in the chore timezone, not UTC",
			// 2026-03-14 23:30 in New York, so the next occurrence is 03-15
			// locally but 03-16 in UTC.
			chore: boundedChore(chModel.FrequencyTypeDaily, "America/New_York", utcDate(2026, time.March, 15, 3, 30),
				&chModel.ScheduleBounds{StopRepeating: stopOn("2026-03-15")}),
			want: timePtr(utcDate(2026, time.March, 16, 3, 30)),
		},
		{
			name: "Seasonal start compares civil dates across a DST transition",
			// Daily from 2026-10-29 23:30 EDT; 2026-11-01 23:30 is EST.
			chore: boundedChore(chModel.FrequencyTypeDaily, "America/New_York", utcDate(2026, time.October, 30, 3, 30),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("11-01", "12-31")}),
			want: timePtr(time.Date(2026, time.November, 1, 23, 30, 0, 0, ny).UTC()),
		},
		{
			name: "Interval does not restart at the window boundary",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeInterval,
				Frequency:     10,
				NextDueDate:   timePtr(utcDate(2026, time.January, 1, 9, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Unit:           jsonPtr("days"),
					Time:           "2026-01-01T09:00:00Z",
					Timezone:       "UTC",
					ScheduleBounds: &chModel.ScheduleBounds{ActivePeriod: starts("2026-02-01")},
				},
			},
			// Jan 11, 21, 31, then Feb 10 — the first on-sequence date in window.
			want: timePtr(utcDate(2026, time.February, 10, 9, 0)),
		},
		{
			name: "Monthly day anchor survives the seasonal clamp",
			chore: chModel.Chore{
				FrequencyType: chModel.FrequencyTypeDayOfTheMonth,
				Frequency:     15,
				NextDueDate:   timePtr(utcDate(2026, time.January, 15, 9, 0)),
				FrequencyMetadataV2: &chModel.FrequencyMetadata{
					Months:         []*string{jsonPtr("january"), jsonPtr("june"), jsonPtr("july")},
					Time:           "2026-01-01T09:00:00Z",
					Timezone:       "UTC",
					ScheduleBounds: &chModel.ScheduleBounds{ActivePeriod: seasonal("06-01", "08-31")},
				},
			},
			want: timePtr(utcDate(2026, time.June, 15, 9, 0)),
		},
		{
			name: "A window that is never reached fails open",
			chore: boundedChore(chModel.FrequencyTypeYearly, "UTC", utcDate(2026, time.March, 1, 9, 0),
				&chModel.ScheduleBounds{ActivePeriod: seasonal("08-01", "08-31")}),
			want: timePtr(utcDate(2027, time.March, 1, 9, 0)),
		},
		{
			name: "Completion limit is not the scheduler's decision",
			chore: boundedChore(chModel.FrequencyTypeDaily, "UTC", utcDate(2026, time.January, 10, 9, 0),
				&chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopCompletions, CompletionLimit: intPtr(1)}}),
			want: timePtr(utcDate(2026, time.January, 11, 9, 0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completedDate := tt.completedDate
			if completedDate.IsZero() {
				completedDate = *tt.chore.NextDueDate
			}
			got, err := scheduleNextDueDate(context.TODO(), &tt.chore, completedDate)
			if err != nil {
				t.Fatalf("scheduleNextDueDate() error = %v", err)
			}
			if !equalTime(got, tt.want) {
				t.Errorf("scheduleNextDueDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScheduleAdaptiveNextDueDateBounds(t *testing.T) {
	due := utcDate(2026, time.January, 10, 9, 0)
	completed := utcDate(2026, time.January, 11, 9, 0)

	// Adaptive skips the active-period clamp: adaptive plus seasonal is meaningless.
	chore := boundedChore(chModel.FrequencyTypeAdaptive, "UTC", due,
		&chModel.ScheduleBounds{ActivePeriod: seasonal("06-01", "08-31")})
	got, err := scheduleAdaptiveNextDueDate(context.TODO(), &chore, completed, nil)
	if err != nil {
		t.Fatalf("scheduleAdaptiveNextDueDate() error = %v", err)
	}
	if !equalTime(got, timePtr(utcDate(2026, time.January, 12, 9, 0))) {
		t.Errorf("adaptive with a season = %v, want the unclamped date", got)
	}

	// The stop condition still applies.
	chore = boundedChore(chModel.FrequencyTypeAdaptive, "UTC", due,
		&chModel.ScheduleBounds{StopRepeating: stopOn("2026-01-11")})
	got, err = scheduleAdaptiveNextDueDate(context.TODO(), &chore, completed, nil)
	if err != nil {
		t.Fatalf("scheduleAdaptiveNextDueDate() error = %v", err)
	}
	if got != nil {
		t.Errorf("adaptive past its stop date = %v, want nil", got)
	}
}
