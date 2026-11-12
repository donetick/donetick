package chore

import (
	"encoding/json"
	"errors"
)

// Preserve a category when older clients omit the new field; explicit null or
// zero clears it. A pointer alone cannot distinguish those requests.
type optionalXPCategory struct {
	Value *int
	Set   bool
}

func (v *optionalXPCategory) UnmarshalJSON(data []byte) error {
	v.Set = true
	if string(data) == "null" {
		v.Value = nil
		return nil
	}
	var id int
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	if id < 0 {
		return errors.New("XP category ID cannot be negative")
	}
	if id == 0 {
		v.Value = nil
	} else {
		v.Value = &id
	}
	return nil
}

func (v optionalXPCategory) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }
