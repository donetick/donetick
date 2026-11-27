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

	_, err = validateAndNormalizeAssignmentRotation(policy, chModel.AssignmentStrategyLeastAssigned, chModel.FrequencyTypeDaily,
		[]chModel.ChoreAssignees{{UserID: 42}, {UserID: 67}}, &due)
	require.Error(t, err)
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
