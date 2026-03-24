package model

import (
	"encoding/json"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// TestFilterConditionValidation tests that FilterCondition accepts null values for operators that don't need them
func TestFilterConditionValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name        string
		jsonInput   string
		shouldError bool
		description string
	}{
		{
			name:        "Due date condition with null value (isDueTomorrow)",
			jsonInput:   `{"type":"dueDate","operator":"isDueTomorrow","value":null}`,
			shouldError: false,
			description: "Operators like isDueTomorrow don't need a value",
		},
		{
			name:        "Due date condition with null value (isDueToday)",
			jsonInput:   `{"type":"dueDate","operator":"isDueToday","value":null}`,
			shouldError: false,
			description: "Operators like isDueToday don't need a value",
		},
		{
			name:        "Due date condition with null value (isOverdue)",
			jsonInput:   `{"type":"dueDate","operator":"isOverdue","value":null}`,
			shouldError: false,
			description: "Operators like isOverdue don't need a value",
		},
		{
			name:        "Due date condition with null value (hasNoDueDate)",
			jsonInput:   `{"type":"dueDate","operator":"hasNoDueDate","value":null}`,
			shouldError: false,
			description: "Operators like hasNoDueDate don't need a value",
		},
		{
			name:        "Assignee condition with value",
			jsonInput:   `{"type":"assignee","operator":"is","value":"user1"}`,
			shouldError: false,
			description: "Assignee conditions with value should work",
		},
		{
			name:        "Status condition with value",
			jsonInput:   `{"type":"status","operator":"is","value":"active"}`,
			shouldError: false,
			description: "Status conditions with value should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var condition FilterCondition
			err := json.Unmarshal([]byte(tt.jsonInput), &condition)
			assert.NoError(t, err, "JSON unmarshaling should succeed")

			err = validate.Struct(condition)
			if tt.shouldError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestFilterConditionsArrayValidation tests the full filter with conditions
func TestFilterConditionsArrayValidation(t *testing.T) {
	validate := validator.New()

	// Test the exact JSON from the issue
	jsonInput := `{
		"name":"assignee",
		"description":"",
		"color":"#26a69a",
		"conditions":[{"type":"dueDate","operator":"isDueTomorrow","value":null}],
		"operator":"AND"
	}`

	var req FilterReq
	err := json.Unmarshal([]byte(jsonInput), &req)
	assert.NoError(t, err, "JSON unmarshaling should succeed")

	err = validate.Struct(req)
	assert.NoError(t, err, "Validation should succeed for due date conditions with null value")

	// Verify the condition was parsed correctly
	assert.Len(t, req.Conditions, 1)
	assert.Equal(t, "dueDate", req.Conditions[0].Type)
	assert.Equal(t, "isDueTomorrow", req.Conditions[0].Operator)
	assert.Nil(t, req.Conditions[0].Value)
}
