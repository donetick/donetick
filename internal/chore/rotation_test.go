package chore

import (
	"encoding/json"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/require"
)

func TestValidateAndNormalizeAssignmentRotation(t *testing.T) {
	due := time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)
	count := 3
	policy, err := validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{
		Mode:  chModel.AssignmentRotationModeCompletions,
		Count: &count,
		Order: []int{42, 999, 42},
	}, chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, []chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}, {UserID: 91}}, &due)
	require.NoError(t, err)
	require.Equal(t, []int{42, 67, 91}, policy.Order)

	_, err = validateAndNormalizeAssignmentRotation(policy, chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeOnce,
		[]chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}}, &due)
	require.Error(t, err)

	_, err = validateAndNormalizeAssignmentRotation(policy, chModel.AssignmentStrategyRandomExceptLastAssigned, chModel.FrequencyTypeDaily,
		[]chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}}, &due)
	require.NoError(t, err)

	_, err = validateAndNormalizeAssignmentRotation(policy, chModel.AssignmentStrategyLeastAssigned, chModel.FrequencyTypeDaily,
		[]chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}}, &due)
	require.Error(t, err)
}

func TestValidateAssignmentRotationCalendarFieldsAndUnscheduledOrder(t *testing.T) {
	due := time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)
	assignees := []chModel.ChoreAssignees{{UserID: 1}, {UserID: 2}}
	weekday := "thursday"
	day := 31
	count := 3

	weekly, err := validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{
		Mode: chModel.AssignmentRotationModeWeek, Weekday: &weekday, DayOfMonth: &day,
	}, chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, assignees, &due)
	require.NoError(t, err)
	require.Equal(t, &weekday, weekly.Weekday)
	require.Nil(t, weekly.DayOfMonth)
	require.Nil(t, weekly.Count)

	monthly, err := validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{
		Mode: chModel.AssignmentRotationModeMonth, Weekday: &weekday, DayOfMonth: &day,
	}, chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, assignees, &due)
	require.NoError(t, err)
	require.Equal(t, &day, monthly.DayOfMonth)
	require.Nil(t, monthly.Weekday)
	require.Nil(t, monthly.Count)

	for _, mode := range []chModel.AssignmentRotationMode{chModel.AssignmentRotationModeWeek, chModel.AssignmentRotationModeMonth} {
		_, err := validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{Mode: mode, Count: &count},
			chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, assignees, &due)
		require.Error(t, err)
	}

	invalidWeekday := "Thursday"
	_, err = validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeWeek, Weekday: &invalidWeekday},
		chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, assignees, &due)
	require.Error(t, err)
	invalidDay := 32
	_, err = validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeMonth, DayOfMonth: &invalidDay},
		chModel.AssignmentStrategyRoundRobin, chModel.FrequencyTypeDaily, assignees, &due)
	require.Error(t, err)

	orderOnly := &chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeDefault, Order: []int{2, 1}}
	for _, frequency := range []chModel.FrequencyType{chModel.FrequencyTypeAlways, chModel.FrequencyTypeTrigger} {
		normalized, err := validateAndNormalizeAssignmentRotation(orderOnly, chModel.AssignmentStrategyRoundRobin, frequency, assignees, nil)
		require.NoError(t, err)
		require.Equal(t, []int{2, 1}, normalized.Order)

		_, err = validateAndNormalizeAssignmentRotation(&chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeWeek, Weekday: &weekday},
			chModel.AssignmentStrategyRoundRobin, frequency, assignees, nil)
		require.Error(t, err)
	}
}

func TestAssignmentRotationJSONDoesNotExposeProgress(t *testing.T) {
	count := 3
	payload, err := json.Marshal(chModel.Chore{
		AssignmentRotation:   &chModel.AssignmentRotation{Mode: chModel.AssignmentRotationModeCompletions, Count: &count, Order: []int{1, 2}},
		RotationAdvanceSlots: 99,
	})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, map[string]any{"mode": "completions", "count": float64(3), "order": []any{float64(1), float64(2)}}, decoded["assignmentRotation"])
	require.NotContains(t, string(payload), "counter")
	require.NotContains(t, string(payload), "rotationProgress")
}
