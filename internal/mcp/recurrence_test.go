package mcp

import (
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/require"
)

func TestUpdateChoreWithNullRecurrenceMetadata(t *testing.T) {
	engine, db := newServer(t)

	t.Run("one-time chore can become recurring", func(t *testing.T) {
		id := mustCall[int](t, engine, alexToken, "create_chore", map[string]any{
			"name": "Feed the cat", "dueDate": "2030-01-01T09:00:00Z",
		})
		require.Nil(t, storedChore(t, db, id).FrequencyMetadataV2)
		mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
			"id": id, "frequencyType": "interval", "frequency": 1,
			"frequencyMetadata": map[string]any{"unit": "days"},
		})
		after := storedChore(t, db, id)
		require.Equal(t, chModel.FrequencyTypeInterval, after.FrequencyType)
		require.NotNil(t, after.FrequencyMetadataV2)
		require.Equal(t, "days", *after.FrequencyMetadataV2.Unit)
		require.Equal(t, "2030-01-01T09:00:00Z", after.FrequencyMetadataV2.Time)
	})

	for _, recurrence := range []map[string]any{
		{"frequencyType": "interval", "frequency": 1, "frequencyMetadata": map[string]any{"unit": "days"}},
		{"frequencyType": "days_of_the_week", "frequencyMetadata": map[string]any{"days": []string{"monday"}}},
		{"frequencyType": "day_of_the_month", "frequency": 1, "frequencyMetadata": map[string]any{"months": []string{"january"}}},
	} {
		t.Run(recurrence["frequencyType"].(string), func(t *testing.T) {
			recurrence["name"] = "Recurring chore"
			recurrence["dueDate"] = "2030-01-01T09:00:00Z"
			id := mustCall[int](t, engine, alexToken, "create_chore", recurrence)
			before := storedChore(t, db, id)

			for _, changeDueDate := range []bool{false, true} {
				args := map[string]any{"id": id, "frequencyMetadata": nil}
				if changeDueDate {
					args["dueDate"] = "2030-01-02T09:00:00Z"
				}
				text := mustFail(t, engine, alexToken, "update_chore", args)
				require.Contains(t, text, "needs frequency")
				after := storedChore(t, db, id)
				require.Equal(t, before.FrequencyMetadataV2, after.FrequencyMetadataV2)
				require.Equal(t, before.NextDueDate, after.NextDueDate)
			}

			// Clearing metadata is valid when switching to a schedule that
			// does not require it.
			mustSucceed(t, engine, alexToken, "update_chore", map[string]any{
				"id": id, "frequencyType": "once", "frequencyMetadata": nil,
			})
			require.Nil(t, storedChore(t, db, id).FrequencyMetadataV2)
		})
	}
}

func TestFollowDueDateWithNullMetadata(t *testing.T) {
	dueDate := "2030-01-01T09:00:00Z"
	chore := object{
		"frequencyType": raw("interval"), "frequencyMetadata": raw(nil), "nextDueDate": raw(dueDate),
	}
	args := choreArguments{DueDate: &dueDate}
	args.followDueDate(chore)
	require.JSONEq(t, `{"time":"2030-01-01T09:00:00Z"}`, string(chore["frequencyMetadata"]))
}
