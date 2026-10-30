package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type Thing struct {
	ID          int          `json:"id" gorm:"primary_key"`
	UserID      int          `json:"userID" gorm:"column:user_id"`
	CircleID    int          `json:"circleId" gorm:"column:circle_id"`
	Name        string       `json:"name" gorm:"column:name"`
	State       string       `json:"state" gorm:"column:state"`
	Type        string       `json:"type" gorm:"column:type"`
	ThingChores []ThingChore `json:"thingChores" gorm:"foreignkey:ThingID;references:ID"`
	UpdatedAt   *time.Time   `json:"updatedAt" gorm:"column:updated_at"`
	CreatedAt   *time.Time   `json:"createdAt" gorm:"column:created_at"`
}

type ThingHistory struct {
	ID        int        `json:"id" gorm:"primary_key"`
	ThingID   int        `json:"thingId" gorm:"column:thing_id"`
	State     string     `json:"state" gorm:"column:state"`
	UpdatedAt *time.Time `json:"updatedAt" gorm:"column:updated_at"`
	CreatedAt *time.Time `json:"createdAt" gorm:"column:created_at"`
}

type ThingChore struct {
	ThingID      int    `json:"thingId" gorm:"column:thing_id;primaryKey;uniqueIndex:idx_thing_user"`
	ChoreID      int    `json:"choreId" gorm:"column:chore_id;primaryKey;uniqueIndex:idx_thing_user"`
	TriggerState string `json:"triggerState" gorm:"column:trigger_state"`
	Condition    string `json:"condition" gorm:"column:condition"`
}

type ThingTrigger struct {
	ID           int    `json:"thingID" binding:"required"`
	TriggerState string `json:"triggerState" binding:"required"`
	Condition    string `json:"condition"`
}

// errTriggerStateType is returned when triggerState is JSON other than a string or number.
var errTriggerStateType = errors.New("triggerState must be a string or number")

// UnmarshalJSON decodes thingID and condition normally and accepts triggerState
// as either a JSON string or a JSON number. Numeric text is preserved without
// a float64 round trip. Missing, null, and empty trigger states stay empty so
// required-field validation is unchanged.
func (t *ThingTrigger) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID           int             `json:"thingID"`
		TriggerState json.RawMessage `json:"triggerState"`
		Condition    string          `json:"condition"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	state, err := decodeTriggerState(raw.TriggerState)
	if err != nil {
		return err
	}
	t.ID = raw.ID
	t.TriggerState = state
	t.Condition = raw.Condition
	return nil
}

func decodeTriggerState(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "", err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", errTriggerStateType
		}
		return "", err
	}

	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", errTriggerStateType
	}
}

type ThingType string

const (
	ThingTypeText    ThingType = "text"
	ThingTypeNumber  ThingType = "number"
	ThingTypeBoolean ThingType = "boolean"
	ThingTypeAction  ThingType = "action"
)
