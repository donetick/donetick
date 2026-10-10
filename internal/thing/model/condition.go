package model

import "strconv"

func (tchore *ThingChore) Matches(newState string) bool {
	if tchore.Condition == "" {
		return newState == tchore.TriggerState
	}

	switch tchore.Condition {
	case "eq":
		return newState == tchore.TriggerState
	case "neq":
		return newState != tchore.TriggerState
	}

	newStateInt, err := strconv.Atoi(newState)
	if err != nil {
		return false
	}
	TargetStateInt, err := strconv.Atoi(tchore.TriggerState)
	if err != nil {
		return false
	}

	switch tchore.Condition {
	case "gt":
		return newStateInt > TargetStateInt
	case "lt":
		return newStateInt < TargetStateInt
	case "gte":
		return newStateInt >= TargetStateInt
	case "lte":
		return newStateInt <= TargetStateInt
	default:
		return newState == tchore.TriggerState
	}

}
