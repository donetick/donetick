package chore

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func scheduleNextDueDate(chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
	// if Chore is rolling then the next due date calculated from the completed date, otherwise it's calculated from the due date
	var nextDueDate time.Time
	var baseDate time.Time
	var frequencyMetadata chModel.FrequencyMetadata
	err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling frequency metadata")
	}
	if chore.FrequencyType == "once" {
		return nil, nil
	}

	if chore.NextDueDate != nil {
		// no due date set, use the current date

		baseDate = chore.NextDueDate.UTC()
	} else {
		baseDate = completedDate.UTC()
	}
	if chore.FrequencyType == "day_of_the_month" || chore.FrequencyType == "days_of_the_week" || chore.FrequencyType == "interval" {
		// time in frequency metadata stored as RFC3339 format like  `2024-07-07T13:27:00-04:00`
		// parse it to time.Time:
		t, err := time.Parse(time.RFC3339, frequencyMetadata.Time)
		if err != nil {
			return nil, fmt.Errorf("error parsing time in frequency metadata")
		}
		// set the time to the time in the frequency metadata:
		baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())

	}
	if chore.IsRolling {
		baseDate = completedDate.UTC()
	}
	if chore.FrequencyType == "daily" {
		nextDueDate = baseDate.AddDate(0, 0, 1)
	} else if chore.FrequencyType == "weekly" {
		nextDueDate = baseDate.AddDate(0, 0, 7)
	} else if chore.FrequencyType == "monthly" {
		nextDueDate = baseDate.AddDate(0, 1, 0)
	} else if chore.FrequencyType == "yearly" {
		nextDueDate = baseDate.AddDate(1, 0, 0)
	} else if chore.FrequencyType == "adaptive" {

		// TODO: calculate next due date based on the history of the chore
		// calculate the difference between the due date and now in days:
		diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
		nextDueDate = completedDate.UTC().Add(diff)
	} else if chore.FrequencyType == "once" {
		// if the chore is a one-time chore, then the next due date is nil
	} else if chore.FrequencyType == "interval" {
		// calculate the difference between the due date and now in days:
		if *frequencyMetadata.Unit == "hours" {
			nextDueDate = baseDate.UTC().Add(time.Hour * time.Duration(chore.Frequency))
		} else if *frequencyMetadata.Unit == "days" {
			nextDueDate = baseDate.UTC().AddDate(0, 0, chore.Frequency)
		} else if *frequencyMetadata.Unit == "weeks" {
			nextDueDate = baseDate.UTC().AddDate(0, 0, chore.Frequency*7)
		} else if *frequencyMetadata.Unit == "months" {
			nextDueDate = baseDate.UTC().AddDate(0, chore.Frequency, 0)
		} else if *frequencyMetadata.Unit == "years" {
			nextDueDate = baseDate.UTC().AddDate(chore.Frequency, 0, 0)
		} else {

			return nil, fmt.Errorf("invalid frequency unit, cannot calculate next due date")
		}
	} else if chore.FrequencyType == "days_of_the_week" {
		// TODO : this logic is bad, need to be refactored and be better.
		// coding at night is almost  always bad idea.
		// calculate the difference between the due date and now in days:
		var frequencyMetadata chModel.FrequencyMetadata
		err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
		if err != nil {

			return nil, fmt.Errorf("error unmarshalling frequency metadata")
		}
		//we can only assign to days of the week that part of the frequency metadata.days
		//it's array of days of the week, for example ["monday", "tuesday", "wednesday"]

		// we need to find the next day of the week in the frequency metadata.days that we can schedule
		// if this the last or there is only one. will use same otherwise find the next one:

		// find the index of the chore day in the frequency metadata.days
		// loop for next 7 days from the base, if the day in the frequency metadata.days then we can schedule it:
		for i := 1; i <= 7; i++ {
			nextDueDate = baseDate.AddDate(0, 0, i)
			nextDay := strings.ToLower(nextDueDate.Weekday().String())
			for _, day := range frequencyMetadata.Days {
				if strings.ToLower(*day) == nextDay {
					nextDate := nextDueDate.UTC()
					return &nextDate, nil
				}
			}
		}
	} else if chore.FrequencyType == "day_of_the_month" {
		var frequencyMetadata chModel.FrequencyMetadata
		err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
		if err != nil {

			return nil, fmt.Errorf("error unmarshalling frequency metadata")
		}

		for i := 1; i <= 12; i++ {
			nextDueDate = baseDate.AddDate(0, i, 0)
			// set the date to the first day of the month:
			nextDueDate = time.Date(nextDueDate.Year(), nextDueDate.Month(), chore.Frequency, nextDueDate.Hour(), nextDueDate.Minute(), 0, 0, nextDueDate.Location())
			nextMonth := strings.ToLower(nextDueDate.Month().String())
			for _, month := range frequencyMetadata.Months {
				if *month == nextMonth {
					nextDate := nextDueDate.UTC()
					return &nextDate, nil
				}
			}
		}
	} else if chore.FrequencyType == "no_repeat" {
		return nil, nil
	} else if chore.FrequencyType == "trigger" {
		// if the chore is a trigger chore, then the next due date is nil
		return nil, nil
	} else {
		return nil, fmt.Errorf("invalid frequency type, cannot calculate next due date")
	}
	return &nextDueDate, nil

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
