package chore

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func TestScheduleNextDueDateBasic(t *testing.T) {
	choreTime := time.Now()
	freqencyMetadataBytes := `{"time":"2024-07-07T14:30:00-04:00"}`
	intervalFreqencyMetadataBytes := `{"time":"2024-07-07T14:30:00-04:00", "unit": "days"}`

	testTable := []struct {
		Name        string
		chore       *chModel.Chore
		completedAt time.Time
		expected    time.Time
	}{
		{
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeDaily,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
			},
			completedAt: choreTime,
			expected:    choreTime.AddDate(0, 0, 1),
		},
		{ // Completed 1 day late
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeDaily,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
			},
			completedAt: choreTime.AddDate(0, 0, 1),
			expected:    choreTime.AddDate(0, 0, 1),
		},
		{
			Name: "Rolling completed 1 day late",
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeDaily,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
				IsRolling:         true,
			},
			completedAt: choreTime.AddDate(0, 0, 1),
			expected:    choreTime.AddDate(0, 0, 1+1),
		},
		{
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeWeekly,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
			},
			completedAt: choreTime,
			expected:    choreTime.AddDate(0, 0, 7),
		},
		{
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeMonthly,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
			},
			completedAt: choreTime,
			expected:    choreTime.AddDate(0, 1, 0),
		},
		{
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeYearly,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &freqencyMetadataBytes,
			},
			completedAt: choreTime,
			expected:    choreTime.AddDate(1, 0, 0),
		},
		{
			Name: "14 days interval Rolling Completed in time",
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeIntervel,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &intervalFreqencyMetadataBytes,
				Frequency:         14,
				IsRolling:         true,
			},
			completedAt: choreTime,
			expected:    choreTime.AddDate(0, 0, 14),
		},
		{
			Name: "14 days interval Rolling Completed late",
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeIntervel,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &intervalFreqencyMetadataBytes,
				Frequency:         14,
				IsRolling:         true,
			},
			completedAt: choreTime.AddDate(0, 0, 1),
			expected:    choreTime.AddDate(0, 0, 14+1),
		},
		{
			Name: "14 days interval Completed in time",
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeIntervel,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &intervalFreqencyMetadataBytes,
				Frequency:         14,
				IsRolling:         false,
			},
			completedAt: choreTime,
			expected:    truncateToDay(choreTime.AddDate(0, 0, 14).UTC()).Add(18 * time.Hour).Add(30 * time.Minute), // Note: Same Hour and Minute as Metadata time
		},
		{
			Name: "14 days interval Completed late",
			chore: &chModel.Chore{
				FrequencyType:     chModel.FrequancyTypeIntervel,
				NextDueDate:       &choreTime,
				FrequencyMetadata: &intervalFreqencyMetadataBytes,
				Frequency:         14,
				IsRolling:         false,
			},
			completedAt: choreTime.AddDate(0, 0, 1),
			expected:    truncateToDay(choreTime.AddDate(0, 0, 14).UTC()).Add(18 * time.Hour).Add(30 * time.Minute), // Note: Same Hour and Minute as Metadata time
		},

		//
	}
	for i, tt := range testTable {
		t.Run(fmt.Sprintf("%s %s %d", tt.chore.FrequencyType, tt.Name, i), func(t *testing.T) {

			actual, err := scheduleNextDueDate(tt.chore, tt.completedAt)
			if err != nil {
				t.Errorf("Error: %v", err)
				t.FailNow()
			}
			if actual == nil {
				t.Errorf("Expected: %v, Error: Actual missing", tt.expected)
			} else if actual.UTC().Format(time.RFC3339) != tt.expected.UTC().Format(time.RFC3339) {
				t.Errorf("Expected: %v, Actual: %v", tt.expected, actual)
			}
		})
	}
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func TestScheduleNextDueDateDayOfTheWeek(t *testing.T) {
	choreTime := time.Now()

	Monday := "monday"
	Wednesday := "wednesday"

	timeOfChore := "2024-07-07T16:30:00-04:00"
	getExpectedTime := func(choreTime time.Time, timeOfChore string) time.Time {
		t, err := time.Parse(time.RFC3339, timeOfChore)
		if err != nil {
			return time.Time{}
		}
		return time.Date(choreTime.Year(), choreTime.Month(), choreTime.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	}
	nextSaturday := choreTime.AddDate(0, 0, 1)
	for nextSaturday.Weekday() != time.Saturday {
		nextSaturday = nextSaturday.AddDate(0, 0, 1)
	}

	nextMonday := choreTime.AddDate(0, 0, 1)
	for nextMonday.Weekday() != time.Monday {
		nextMonday = nextMonday.AddDate(0, 0, 1)
	}

	nextTuesday := choreTime.AddDate(0, 0, 1)
	for nextTuesday.Weekday() != time.Tuesday {
		nextTuesday = nextTuesday.AddDate(0, 0, 1)
	}

	nextWednesday := choreTime.AddDate(0, 0, 1)
	for nextWednesday.Weekday() != time.Wednesday {
		nextWednesday = nextWednesday.AddDate(0, 0, 1)
	}

	nextThursday := choreTime.AddDate(0, 0, 1)
	for nextThursday.Weekday() != time.Thursday {
		nextThursday = nextThursday.AddDate(0, 0, 1)
	}

	testTable := []struct {
		chore             *chModel.Chore
		frequencyMetadata *chModel.FrequencyMetadata
		expected          time.Time
	}{
		{
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequancyTypeDayOfTheWeek,
				NextDueDate:   &nextSaturday,
			},
			frequencyMetadata: &chModel.FrequencyMetadata{
				Time: timeOfChore,
				Days: []*string{&Monday, &Wednesday},
			},

			expected: getExpectedTime(nextMonday, timeOfChore),
		},
		{
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequancyTypeDayOfTheWeek,
				NextDueDate:   &nextMonday,
			},
			frequencyMetadata: &chModel.FrequencyMetadata{
				Time: timeOfChore,
				Days: []*string{&Monday, &Wednesday},
			},
			expected: getExpectedTime(nextWednesday, timeOfChore),
		},
	}
	for _, tt := range testTable {
		t.Run(string(tt.chore.FrequencyType), func(t *testing.T) {
			bytesFrequencyMetadata, err := json.Marshal(tt.frequencyMetadata)
			stringFrequencyMetadata := string(bytesFrequencyMetadata)

			if err != nil {
				t.Errorf("Error: %v", err)
			}
			tt.chore.FrequencyMetadata = &stringFrequencyMetadata
			actual, err := scheduleNextDueDate(tt.chore, choreTime)

			if err != nil {
				t.Errorf("Error: %v", err)
			}
			if actual != nil && actual.UTC().Format(time.RFC3339) != tt.expected.UTC().Format(time.RFC3339) {
				t.Errorf("Expected: %v, Actual: %v", tt.expected, actual)
			}
		})
	}
}
func TestScheduleAdaptiveNextDueDate(t *testing.T) {
	getTimeFromDate := func(timeOfChore string) *time.Time {
		t, err := time.Parse(time.RFC3339, timeOfChore)
		if err != nil {
			return nil
		}
		return &t
	}
	testTable := []struct {
		description  string
		history      []*chModel.ChoreHistory
		chore        *chModel.Chore
		expected     *time.Time
		completeDate *time.Time
	}{
		{
			description: "Every Two days",
			chore: &chModel.Chore{
				NextDueDate: getTimeFromDate("2024-07-13T01:30:00-00:00"),
			},
			history: []*chModel.ChoreHistory{
				{
					CompletedAt: getTimeFromDate("2024-07-11T01:30:00-00:00"),
				},
				// {
				// 	CompletedAt: getTimeFromDate("2024-07-09T01:30:00-00:00"),
				// },
				// {
				// 	CompletedAt: getTimeFromDate("2024-07-07T01:30:00-00:00"),
				// },
			},
			expected: getTimeFromDate("2024-07-15T01:30:00-00:00"),
		},
		{
			description: "Every 8 days",
			chore: &chModel.Chore{
				NextDueDate: getTimeFromDate("2024-07-13T01:30:00-00:00"),
			},
			history: []*chModel.ChoreHistory{
				{
					CompletedAt: getTimeFromDate("2024-07-05T01:30:00-00:00"),
				},
				{
					CompletedAt: getTimeFromDate("2024-06-27T01:30:00-00:00"),
				},
			},
			expected: getTimeFromDate("2024-07-21T01:30:00-00:00"),
		},
		{
			description: "40 days with limit Data",
			chore: &chModel.Chore{
				NextDueDate: getTimeFromDate("2024-07-13T01:30:00-00:00"),
			},
			history: []*chModel.ChoreHistory{
				{CompletedAt: getTimeFromDate("2024-06-03T01:30:00-00:00")},
			},
			expected: getTimeFromDate("2024-08-22T01:30:00-00:00"),
		},
	}
	for _, tt := range testTable {
		t.Run(tt.description, func(t *testing.T) {
			expectedNextDueDate := tt.expected

			actualNextDueDate, err := scheduleAdaptiveNextDueDate(tt.chore, *tt.chore.NextDueDate, tt.history)
			if err != nil {
				t.Errorf("Error: %v", err)
			}

			if actualNextDueDate == nil || !actualNextDueDate.Equal(*expectedNextDueDate) {
				t.Errorf("Expected: %v, Actual: %v", expectedNextDueDate, actualNextDueDate)
			}
		})
	}
}
