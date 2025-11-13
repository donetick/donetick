package chore

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"donetick.com/core/logging"
)

func scheduleNextDueDate(ctx context.Context, chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
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
			log := logging.FromContext(ctx)
			log.Error("error parsing time in frequency metadata", "error", err, "chore_id", chore.ID)
			log.Warn("falling back to current time for next due date calculation")

			// fallback to use the next due date time if available:
			if chore.NextDueDate != nil {
				t = chore.NextDueDate.UTC()
			} else {
				t = time.Now().UTC()
			}

		}
		t = t.UTC()
		baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
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

		// Handle different week patterns
		weekPattern := chore.FrequencyMetadataV2.WeekPattern

		// Default to every_week if no pattern specified
		if weekPattern == nil || *weekPattern == "" || *weekPattern == "every_week" {
			// Get timezone for calculations - prefer chore timezone, fallback to UTC
			var loc *time.Location
			var err error
			if chore.FrequencyMetadataV2.Timezone != "" {
				loc, err = time.LoadLocation(chore.FrequencyMetadataV2.Timezone)
				if err != nil {
					log := logging.FromContext(ctx)
					log.Error("error loading timezone from frequency metadata", "error", err, "timezone", chore.FrequencyMetadataV2.Timezone, "chore_id", chore.ID)
					loc = time.UTC // fallback to UTC
				}
			} else {
				loc = time.UTC
			}

			// Convert baseDate to the target timezone for weekday calculations
			baseDateInTimezone := baseDate.In(loc)

			// Find the next valid day of the week in the target timezone
			for i := 1; i <= 7; i++ {
				nextDueDateInTimezone := baseDateInTimezone.AddDate(0, 0, i)
				nextDay := strings.ToLower(nextDueDateInTimezone.Weekday().String())
				for _, day := range chore.FrequencyMetadataV2.Days {
					if strings.ToLower(*day) == nextDay {
						// Convert back to UTC for storage
						nextDueDateUTC := nextDueDateInTimezone.UTC()
						return &nextDueDateUTC, nil
					}
				}
			}
			return nil, fmt.Errorf("no matching day of the week found")
		}

		// Handle week_of_month pattern
		if *weekPattern == chModel.WeekPatternWeekOfMonth {
			occurrences := getOccurrences(chore.FrequencyMetadataV2)
			if len(occurrences) == 0 {
				return nil, fmt.Errorf("week_of_month requires at least one occurrence")
			}
			return findNextDueDateForOccurrencePattern(baseDate, chore.FrequencyMetadataV2.Days, occurrences, true)
		}

		// Handle week_of_quarter pattern
		if *weekPattern == chModel.WeekPatternWeekOfQuarter {
			occurrences := getOccurrences(chore.FrequencyMetadataV2)
			if len(occurrences) == 0 {
				return nil, fmt.Errorf("week_of_quarter requires at least one occurrence")
			}
			return findNextDueDateForOccurrencePattern(baseDate, chore.FrequencyMetadataV2.Days, occurrences, false)
		}

		return nil, fmt.Errorf("invalid week pattern: %s", *weekPattern)
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
				if strings.EqualFold(*month, time.Month(nextMonth).String()) {
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

// getOccurrences returns the occurrences from metadata, supporting both new and legacy formats
func getOccurrences(metadata *chModel.FrequencyMetadata) []string {
	// Prefer new Occurrences field
	if len(metadata.Occurrences) > 0 {
		occurrences := make([]string, len(metadata.Occurrences))
		for i, occ := range metadata.Occurrences {
			if *occ == -1 {
				occurrences[i] = "last"
			} else {
				occurrences[i] = fmt.Sprintf("%d", *occ)
			}
		}
		return occurrences
	}

	// Fall back to legacy WeekNumbers for backward compatibility
	if len(metadata.WeekNumbers) > 0 {
		occurrences := make([]string, len(metadata.WeekNumbers))
		for i, week := range metadata.WeekNumbers {
			occurrences[i] = fmt.Sprintf("%d", week)
		}
		return occurrences
	}

	return []string{}
}

// findNextDueDateForOccurrencePattern finds the next due date for occurrence-based patterns
// isMonthly: true for week_of_month, false for week_of_quarter
func findNextDueDateForOccurrencePattern(baseDate time.Time, days []*string, occurrences []string, isMonthly bool) (*time.Time, error) {
	// Convert days to a map for faster lookup
	dayMap := make(map[string]bool)
	for _, day := range days {
		dayMap[strings.ToLower(*day)] = true
	}

	// Convert occurrences to a map for faster lookup
	occurrenceMap := make(map[string]bool)
	for _, occ := range occurrences {
		occurrenceMap[strings.ToLower(occ)] = true
	}

	// Start searching from the next day
	currentDate := baseDate.AddDate(0, 0, 1)

	// Limit search to avoid infinite loops (search for up to 2 years)
	maxSearchDays := 730

	for i := 0; i < maxSearchDays; i++ {
		dayName := strings.ToLower(currentDate.Weekday().String())

		// Check if this day matches one of our target days
		if dayMap[dayName] {
			if isMonthly {
				// Calculate occurrence within month
				occurrence := getNthOccurrenceInMonth(currentDate, currentDate.Weekday())
				if occurrenceMap[fmt.Sprintf("%d", occurrence)] ||
					(occurrenceMap["last"] && isLastOccurrenceInMonth(currentDate, currentDate.Weekday())) {
					return &currentDate, nil
				}
			} else {
				// Calculate occurrence within quarter
				occurrence := getNthOccurrenceInQuarter(currentDate, currentDate.Weekday())
				if occurrenceMap[fmt.Sprintf("%d", occurrence)] ||
					(occurrenceMap["last"] && isLastOccurrenceInQuarter(currentDate, currentDate.Weekday())) {
					return &currentDate, nil
				}
			}
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil, fmt.Errorf("no matching date found for the specified occurrence pattern")
}

// getNthOccurrenceInMonth returns which occurrence this is within the month (1-based)
func getNthOccurrenceInMonth(date time.Time, weekday time.Weekday) int {
	// Start from the first day of the month
	firstOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())

	// Count occurrences of this weekday up to the given date
	occurrence := 0
	for d := firstOfMonth; d.Before(date) || d.Equal(date); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == weekday {
			occurrence++
		}
	}

	return occurrence
}

// isLastOccurrenceInMonth checks if this is the last occurrence of the weekday in the month
func isLastOccurrenceInMonth(date time.Time, weekday time.Weekday) bool {
	// Check if there's another occurrence of this weekday in the same month
	nextWeek := date.AddDate(0, 0, 7)
	return nextWeek.Month() != date.Month()
}

// getNthOccurrenceInQuarter returns which occurrence this is within the quarter (1-based)
func getNthOccurrenceInQuarter(date time.Time, weekday time.Weekday) int {
	// Get the first day of the quarter
	year := date.Year()
	month := int(date.Month())

	var quarterStartMonth int
	if month <= 3 {
		quarterStartMonth = 1 // Q1: Jan-Mar
	} else if month <= 6 {
		quarterStartMonth = 4 // Q2: Apr-Jun
	} else if month <= 9 {
		quarterStartMonth = 7 // Q3: Jul-Sep
	} else {
		quarterStartMonth = 10 // Q4: Oct-Dec
	}

	firstOfQuarter := time.Date(year, time.Month(quarterStartMonth), 1, 0, 0, 0, 0, date.Location())

	// Count occurrences of this weekday up to the given date
	occurrence := 0
	for d := firstOfQuarter; d.Before(date) || d.Equal(date); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == weekday {
			occurrence++
		}
	}

	return occurrence
}

// isLastOccurrenceInQuarter checks if this is the last occurrence of the weekday in the quarter
func isLastOccurrenceInQuarter(date time.Time, weekday time.Weekday) bool {
	// Check if there's another occurrence of this weekday in the same quarter
	nextWeek := date.AddDate(0, 0, 7)
	if nextWeek.Year() != date.Year() {
		return true
	}

	month := int(date.Month())
	nextMonth := int(nextWeek.Month())

	if month <= 3 && nextMonth > 3 {
		return true
	} else if month <= 6 && nextMonth > 6 {
		return true
	} else if month <= 9 && nextMonth > 9 {
		return true
	} else if month <= 12 && nextMonth > 12 {
		return true
	}

	return false
}
func scheduleAdaptiveNextDueDate(chore *chModel.Chore, completedDate time.Time, history []*chModel.ChoreHistory) (*time.Time, error) {

	history = append([]*chModel.ChoreHistory{
		{
			PerformedAt: &completedDate,
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
		// Skip entries with nil PerformedAt
		if history[i].PerformedAt == nil || history[i+1].PerformedAt == nil {
			continue
		}
		delay := history[i].PerformedAt.UTC().Sub(history[i+1].PerformedAt.UTC()).Seconds()
		weight := math.Pow(decayFactor, float64(i))
		totalDelay += delay * weight
		totalWeight += weight
	}

	// If no valid history entries, fall back to default behavior
	if totalWeight == 0 {
		if chore.NextDueDate != nil {
			diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
			nextDueDate := completedDate.UTC().Add(diff)
			return &nextDueDate, nil
		}
		return nil, nil
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

	// Handle no assignee strategy
	if chore.AssignStrategy == chModel.AssignmentStrategyNoAssignee {
		chore.AssignedTo = nil // Set to nil to indicate no assignee
	} else if len(chore.Assignees) == 0 {
		createdBy := chore.CreatedBy
		chore.AssignedTo = &createdBy
	} else {
		userID := chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
		chore.AssignedTo = &userID
	}
	chore.UpdatedAt = time.Now()
}
