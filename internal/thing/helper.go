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
	return tchore.Matches(newState)
}
