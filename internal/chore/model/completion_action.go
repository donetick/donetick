package model

import (
	"fmt"
	"math/big"
	"strconv"

	tModel "donetick.com/core/internal/thing/model"
)

// CompletionAction changes a Thing when this task is successfully completed.
// Add accepts a signed integer, allowing both increments and decrements.
type CompletionAction struct {
	ThingID   int    `json:"thingId"`
	Operation string `json:"operation"`
	Value     string `json:"value"`
}

func (a CompletionAction) NextState(thing *tModel.Thing) (string, error) {
	switch a.Operation {
	case "set":
		switch thing.Type {
		case "text":
			return a.Value, nil
		case "boolean":
			if a.Value == "true" || a.Value == "false" {
				return a.Value, nil
			}
		case "number":
			if _, err := strconv.Atoi(a.Value); err == nil {
				return a.Value, nil
			}
		}
	case "add":
		if thing.Type != "number" {
			return "", fmt.Errorf("Only numeric Things support increase/decrease")
		}
		current, err := strconv.Atoi(thing.State)
		if err != nil {
			return "", fmt.Errorf("Thing has an invalid numeric state")
		}
		delta, err := strconv.Atoi(a.Value)
		if err != nil {
			return "", fmt.Errorf("Increase/decrease must be a whole number")
		}
		next := new(big.Int).Add(big.NewInt(int64(current)), big.NewInt(int64(delta))).String()
		if _, err := strconv.Atoi(next); err != nil {
			return "", fmt.Errorf("Thing value would overflow")
		}
		return next, nil
	default:
		return "", fmt.Errorf("Unknown completion action operation")
	}
	return "", fmt.Errorf("Invalid completion action value for %s Thing", thing.Type)
}
