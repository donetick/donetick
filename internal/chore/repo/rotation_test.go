package chore

import (
	"errors"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func rotationTestChore(mode chModel.AssignmentRotationMode, count *int) *chModel.Chore {
	due := time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)
	assigned := 42
	return &chModel.Chore{
		Name: "rotation", CircleID: testCircleID, CreatedBy: testOwnerID, IsActive: true,
		FrequencyType: chModel.FrequencyTypeDaily, Frequency: 1, NextDueDate: &due,
		AssignedTo: &assigned, AssignStrategy: chModel.AssignmentStrategyRoundRobin,
		Assignees:          []chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}, {UserID: 91}},
		AssignmentRotation: &chModel.AssignmentRotation{Mode: mode, Count: count, Order: []int{42, 91, 67}},
	}
}

func TestConfiguredCompletionRotationThresholdAndOrder(t *testing.T) {
	_, db := newTestChoreRepo(t)
	count := 2
	chore := rotationTestChore(chModel.AssignmentRotationModeCompletions, &count)
	require.NoError(t, db.Create(chore).Error)
	require.NoError(t, db.Create(initialRotationProgress(chore)).Error)
	next := chore.NextDueDate.AddDate(0, 0, 1)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		assigned, _, err := nextConfiguredAssignee(tx, chore, &next, chore.AssignedTo, true)
		require.NoError(t, err)
		require.Equal(t, 42, *assigned)
		return nil
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		assigned, _, err := nextConfiguredAssignee(tx, chore, &next, chore.AssignedTo, false)
		require.NoError(t, err)
		require.Equal(t, 42, *assigned, "skip does not count as a completion")
		return nil
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		assigned, _, err := nextConfiguredAssignee(tx, chore, &next, chore.AssignedTo, true)
		require.NoError(t, err)
		require.Equal(t, 91, *assigned)
		return nil
	}))
}

func TestConfiguredRotationProgressRollsBackWithOccurrence(t *testing.T) {
	_, db := newTestChoreRepo(t)
	count := 1
	chore := rotationTestChore(chModel.AssignmentRotationModeCompletions, &count)
	require.NoError(t, db.Create(chore).Error)
	require.NoError(t, db.Create(initialRotationProgress(chore)).Error)
	next := chore.NextDueDate.AddDate(0, 0, 1)

	errExpected := errors.New("retry")
	err := db.Transaction(func(tx *gorm.DB) error {
		assigned, _, innerErr := nextConfiguredAssignee(tx, chore, &next, chore.AssignedTo, true)
		require.NoError(t, innerErr)
		require.Equal(t, 91, *assigned)
		return errExpected
	})
	require.ErrorIs(t, err, errExpected)

	var progress chModel.AssignmentRotationProgress
	require.NoError(t, db.First(&progress, "chore_id = ?", chore.ID).Error)
	require.Zero(t, progress.Counter)
}

func TestCalendarRotationBoundaryUsesConfiguredDayAndTimezone(t *testing.T) {
	day := 31
	chore := rotationTestChore(chModel.AssignmentRotationModeMonth, nil)
	chore.AssignmentRotation.DayOfMonth = &day
	chore.FrequencyMetadataV2 = &chModel.FrequencyMetadata{Timezone: "America/New_York"}

	// Midnight in New York is still the previous UTC date; day 31 clamps to leap day.
	februaryBoundary := time.Date(2028, time.February, 29, 5, 0, 0, 0, time.UTC)
	require.Equal(t, "2028-02-29", rotationBoundary(chore, februaryBoundary))
	beforeBoundary := time.Date(2028, time.February, 29, 4, 30, 0, 0, time.UTC)
	require.Equal(t, "2028-01-31", rotationBoundary(chore, beforeBoundary))

	weekday := "friday"
	chore.AssignmentRotation = &chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeWeek, Weekday: &weekday}
	fridayInNewYork := time.Date(2028, time.March, 3, 4, 30, 0, 0, time.UTC) // Thursday 23:30 local.
	require.Equal(t, "2028-02-25", rotationBoundary(chore, fridayInNewYork))
	fridayInNewYork = time.Date(2028, time.March, 3, 5, 30, 0, 0, time.UTC)
	require.Equal(t, "2028-03-03", rotationBoundary(chore, fridayInNewYork))
}

func TestCalendarRotationBoundaryDefaultsToMondayAndDayOne(t *testing.T) {
	chore := rotationTestChore(chModel.AssignmentRotationModeWeek, nil)
	require.Equal(t, "2026-01-05", rotationBoundary(chore, time.Date(2026, time.January, 7, 9, 0, 0, 0, time.UTC)))
	chore.AssignmentRotation = &chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeMonth}
	require.Equal(t, "2026-01-01", rotationBoundary(chore, time.Date(2026, time.January, 7, 9, 0, 0, 0, time.UTC)))
}

func TestRotationTimingIncludesCalendarDay(t *testing.T) {
	monday, friday := "monday", "friday"
	day1, day31 := 1, 31
	require.False(t, rotationTimingEqual(
		&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeWeek, Weekday: &monday},
		&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeWeek, Weekday: &friday},
	))
	require.False(t, rotationTimingEqual(
		&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeMonth, DayOfMonth: &day1},
		&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeMonth, DayOfMonth: &day31},
	))
}

func TestRandomRotationNeverRepeatsCurrentAssignee(t *testing.T) {
	current := 42
	for _, strategy := range []chModel.AssignmentStrategy{chModel.AssignmentStrategyRandom, chModel.AssignmentStrategyRandomExceptLastAssigned} {
		chore := &chModel.Chore{
			AssignStrategy:     strategy,
			Assignees:          []chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}, {UserID: 91}},
			AssignmentRotation: &chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeDefault},
		}
		for i := 0; i < 50; i++ {
			next := rotateConfiguredAssignee(chore, &current)
			require.NotNil(t, next)
			require.NotEqual(t, current, *next, string(strategy))
		}
	}
}
