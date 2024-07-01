package thing

import (
	"strconv"

	tModel "donetick.com/core/internal/thing/model"
)

func isValidThingState(thing *tModel.Thing) bool {
	switch thing.Type {
	case "number":
		_, err := strconv.Atoi(thing.State)
		return err == nil
	case "text":
		return true
	case "boolean":
		return thing.State == "true" || thing.State == "false"
	default:
		return false
	}
}

func EvaluateThingChore(tchore *tModel.ThingChore, newState string) bool {
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
