package chore

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func scheduleNextDueDate(chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
	if chore.FrequencyType == "once" || chore.FrequencyType == "no_repeat" || chore.FrequencyType == "trigger" {
		return nil, nil
	}

	var baseDate time.Time
	if chore.NextDueDate != nil {
		baseDate = chore.NextDueDate.UTC()
	} else {
		baseDate = completedDate.UTC()
	}
	if chore.IsRolling {
		baseDate = completedDate.UTC()
	}

	// Handle time-based frequencies, ensure time is in the future
	if chore.FrequencyType == "day_of_the_month" || chore.FrequencyType == "days_of_the_week" || chore.FrequencyType == "interval" {
		t, err := time.Parse(time.RFC3339, chore.FrequencyMetadataV2.Time)
		if err != nil {
			return nil, fmt.Errorf("error parsing time in frequency metadata: %w", err)
		}
		t = t.UTC()
		baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		// If the time is in the past today, move it to tomorrow
		if baseDate.Before(completedDate) {
			baseDate = baseDate.AddDate(0, 0, 1)
		}
	}

	switch chore.FrequencyType {
	case "daily":
		baseDate = baseDate.AddDate(0, 0, 1)
	case "weekly":
		baseDate = baseDate.AddDate(0, 0, 7)
	case "monthly":
		baseDate = baseDate.AddDate(0, 1, 0)
	case "yearly":
		baseDate = baseDate.AddDate(1, 0, 0)
	case "adaptive":
		// TODO: Implement a more sophisticated adaptive logic
		diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
		baseDate = completedDate.UTC().Add(diff)
	case "interval":
		switch *chore.FrequencyMetadataV2.Unit {
		case "hours":
			baseDate = baseDate.Add(time.Duration(chore.Frequency) * time.Hour)
		case "days":
			baseDate = baseDate.AddDate(0, 0, chore.Frequency)
		case "weeks":
			baseDate = baseDate.AddDate(0, 0, chore.Frequency*7)
		case "months":
			baseDate = baseDate.AddDate(0, chore.Frequency, 0)
		case "years":
			baseDate = baseDate.AddDate(chore.Frequency, 0, 0)
		default:
			return nil, fmt.Errorf("invalid frequency unit: %s", *chore.FrequencyMetadataV2.Unit)
		}
	case "days_of_the_week":
		if len(chore.FrequencyMetadataV2.Days) == 0 {
			return nil, fmt.Errorf("days_of_the_week requires at least one day")
		}
		// Find the next valid day of the week
		for i := 1; i <= 7; i++ {
			nextDueDate := baseDate.AddDate(0, 0, i)
			nextDay := strings.ToLower(nextDueDate.Weekday().String())
			for _, day := range chore.FrequencyMetadataV2.Days {
				if strings.ToLower(*day) == nextDay {
					return &nextDueDate, nil
				}
			}
		}
		return nil, fmt.Errorf("no matching day of the week found")
	case "day_of_the_month":
		// for day of the month we need to pick the highest between completed date and next due date
		// when the chore is rolling. i keep forgetting so am writing a detail comment here:
		// if task due every 15 of jan, and you completed it on the 13 of jan( before the due date ) if we schedule from due date
		// we will go back to 15 of jan. so we need to pick the highest between the two dates specifically for day of the month
		if chore.IsRolling && chore.NextDueDate != nil {
			secondAfterDueDate := chore.NextDueDate.UTC().Add(time.Second)
			if completedDate.Before(secondAfterDueDate) {
				baseDate = secondAfterDueDate
			}
		}
		if len(chore.FrequencyMetadataV2.Months) == 0 {
			return nil, fmt.Errorf("day_of_the_month requires at least one month")
		}
		// Ensure the day of the month is valid
		if chore.Frequency <= 0 || chore.Frequency > 31 {
			return nil, fmt.Errorf("invalid day of the month: %d", chore.Frequency)
		}

		// Find the next valid day of the month, considering the year
		currentMonth := int(baseDate.Month())

		var startFrom int
		if chore.NextDueDate != nil && baseDate.Month() == chore.NextDueDate.Month() {
			startFrom = 1
		}

		for i := startFrom; i < 12+startFrom; i++ { // Start from 0 to check the current month first
			nextDueDate := baseDate.AddDate(0, i, 0)
			nextMonth := (currentMonth + i) % 12 // Use modulo to cycle through months
			if nextMonth == 0 {
				nextMonth = 12 // Adjust for December
			}

			// Ensure the target day exists in the month (e.g., Feb 30th is invalid)
			lastDayOfMonth := time.Date(nextDueDate.Year(), time.Month(nextMonth+1), 0, 0, 0, 0, 0, time.UTC).Day()
			targetDay := chore.Frequency
			if targetDay > lastDayOfMonth {
				targetDay = lastDayOfMonth
			}

			nextDueDate = time.Date(nextDueDate.Year(), time.Month(nextMonth), targetDay, nextDueDate.Hour(), nextDueDate.Minute(), 0, 0, time.UTC)

			for _, month := range chore.FrequencyMetadataV2.Months {
				if strings.ToLower(*month) == strings.ToLower(time.Month(nextMonth).String()) {
					return &nextDueDate, nil
				}
			}
		}
		return nil, fmt.Errorf("no matching month found")
	default:
		return nil, fmt.Errorf("invalid frequency type: %s", chore.FrequencyType)
	}

	return &baseDate, nil
}
func scheduleAdaptiveNextDueDate(chore *chModel.Chore, completedDate time.Time, history []*chModel.ChoreHistory) (*time.Time, error) {

	history = append([]*chModel.ChoreHistory{
		{
			CompletedAt: &completedDate,
		},
	}, history...)

	if len(history) < 2 {
		if chore.NextDueDate != nil {
			diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
			nextDueDate := completedDate.UTC().Add(diff)
			return &nextDueDate, nil
		}
		return nil, nil
	}

	var totalDelay float64
	var totalWeight float64
	decayFactor := 0.5 // Adjust this value to control the decay rate

	for i := 0; i < len(history)-1; i++ {
		delay := history[i].CompletedAt.UTC().Sub(history[i+1].CompletedAt.UTC()).Seconds()
		weight := math.Pow(decayFactor, float64(i))
		totalDelay += delay * weight
		totalWeight += weight
	}

	averageDelay := totalDelay / totalWeight
	nextDueDate := completedDate.UTC().Add(time.Duration(averageDelay) * time.Second)

	return &nextDueDate, nil
}
func RemoveAssigneeAndReassign(chore *chModel.Chore, userID int) {
	for i, assignee := range chore.Assignees {
		if assignee.UserID == userID {
			chore.Assignees = append(chore.Assignees[:i], chore.Assignees[i+1:]...)
			break
		}
	}
	if len(chore.Assignees) == 0 {
		chore.AssignedTo = chore.CreatedBy
	} else {
		chore.AssignedTo = chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
	}
	chore.UpdatedAt = time.Now()
}
