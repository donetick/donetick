package model

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestThingTriggerUnmarshalNumericPreservesText(t *testing.T) {
	// 9007199254740993 is 2^53+1. A float64 round trip would turn it into 9007199254740992.
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "zero", raw: "0", want: "0"},
		{name: "positive integer", raw: "42", want: "42"},
		{name: "negative integer", raw: "-7", want: "-7"},
		{name: "fraction", raw: "1.5", want: "1.5"},
		{name: "exponent", raw: "1e2", want: "1e2"},
		{name: "above float64 exact integer range", raw: "9007199254740993", want: "9007199254740993"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"thingID":3,"triggerState":%s,"condition":"gte"}`, test.raw)
			var got ThingTrigger
			require.NoError(t, json.Unmarshal([]byte(payload), &got))
			require.Equal(t, 3, got.ID)
			require.Equal(t, "gte", got.Condition)
			require.Equal(t, test.want, got.TriggerState)

			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf(`{"thingID":3,"triggerState":%q,"condition":"gte"}`, test.want), string(encoded))
		})
	}
}

func TestThingTriggerUnmarshalStringState(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "threshold", raw: `"10"`, want: "10"},
		{name: "text", raw: `"hot"`, want: "hot"},
		{name: "string true", raw: `"true"`, want: "true"},
		{name: "string false", raw: `"false"`, want: "false"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"thingID":8,"triggerState":%s,"condition":"neq"}`, test.raw)
			var got ThingTrigger
			require.NoError(t, json.Unmarshal([]byte(payload), &got))
			require.Equal(t, 8, got.ID)
			require.Equal(t, "neq", got.Condition)
			require.Equal(t, test.want, got.TriggerState)

			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf(`{"thingID":8,"triggerState":%q,"condition":"neq"}`, test.want), string(encoded))
		})
	}
}

func TestThingTriggerRejectsNonScalarState(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		syntax bool
	}{
		{name: "boolean true", raw: `{"thingID":1,"triggerState":true,"condition":"eq"}`},
		{name: "boolean false", raw: `{"thingID":1,"triggerState":false,"condition":"eq"}`},
		{name: "object", raw: `{"thingID":1,"triggerState":{"value":1},"condition":"eq"}`},
		{name: "array", raw: `{"thingID":1,"triggerState":[1],"condition":"eq"}`},
		{name: "malformed", raw: `{"thingID":1,"triggerState":`, syntax: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got ThingTrigger
			err := json.Unmarshal([]byte(test.raw), &got)
			require.Error(t, err)
			require.Equal(t, ThingTrigger{}, got)
			if test.syntax {
				var syntax *json.SyntaxError
				require.ErrorAs(t, err, &syntax)
				return
			}
			require.ErrorIs(t, err, errTriggerStateType)
		})
	}
}

func TestThingTriggerRequiredState(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "missing", raw: `{"thingID":1,"condition":"eq"}`, wantErr: true},
		{name: "null", raw: `{"thingID":1,"triggerState":null,"condition":"eq"}`, wantErr: true},
		{name: "empty string", raw: `{"thingID":1,"triggerState":"","condition":"eq"}`, wantErr: true},
		{name: "numeric zero", raw: `{"thingID":1,"triggerState":0,"condition":"eq"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var decoded ThingTrigger
			require.NoError(t, json.Unmarshal([]byte(test.raw), &decoded))
			if test.wantErr {
				require.Empty(t, decoded.TriggerState)
			} else {
				require.Equal(t, "0", decoded.TriggerState)
			}

			var got ThingTrigger
			err := binding.JSON.BindBody([]byte(test.raw), &got)
			if !test.wantErr {
				require.NoError(t, err)
				require.Equal(t, "0", got.TriggerState)
				require.Equal(t, 1, got.ID)
				require.Equal(t, "eq", got.Condition)
				return
			}
			var verrs validator.ValidationErrors
			require.ErrorAs(t, err, &verrs)
			require.Len(t, verrs, 1)
			require.Equal(t, "TriggerState", verrs[0].Field())
			require.Equal(t, "required", verrs[0].Tag())
		})
	}
}

func TestThingChoreMatchesPersistedIntegerThreshold(t *testing.T) {
	var trigger ThingTrigger
	require.NoError(t, json.Unmarshal([]byte(`{"thingID":1,"triggerState":10,"condition":"eq"}`), &trigger))
	require.Equal(t, "10", trigger.TriggerState)

	chore := ThingChore{TriggerState: trigger.TriggerState, Condition: "eq"}
	require.True(t, chore.Matches("10"))
	require.False(t, chore.Matches("11"))
	require.False(t, chore.Matches("9"))

	chore.Condition = "gt"
	require.True(t, chore.Matches("11"))
	require.False(t, chore.Matches("10"))
	require.False(t, chore.Matches("9"))

	chore.Condition = "lt"
	require.True(t, chore.Matches("9"))
	require.False(t, chore.Matches("10"))

	chore.Condition = "gte"
	require.True(t, chore.Matches("10"))
	require.True(t, chore.Matches("11"))
	require.False(t, chore.Matches("9"))

	chore.Condition = "lte"
	require.True(t, chore.Matches("10"))
	require.True(t, chore.Matches("9"))
	require.False(t, chore.Matches("11"))
}
