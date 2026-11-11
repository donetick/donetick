package realtime

import (
	chModel "donetick.com/core/internal/chore/model"
	lModel "donetick.com/core/internal/label/model"
)

// forUser copies only the event data that needs redaction. Events are shared by
// multiple connections, so never mutate the original chore or its label slice.
func (e *Event) forUser(userID int) *Event {
	var chore *chModel.Chore
	switch data := e.Data.(type) {
	case *ChoreCreatedData:
		chore = data.Chore
	case *ChoreEventData:
		chore = data.Chore
	default:
		return e
	}
	if chore == nil || chore.LabelsV2 == nil {
		return e
	}
	labels := make([]lModel.Label, 0, len(*chore.LabelsV2))
	for _, label := range *chore.LabelsV2 {
		if label.IsShared || label.CreatedBy == userID {
			labels = append(labels, label)
		}
	}
	if len(labels) == len(*chore.LabelsV2) {
		return e
	}
	visibleChore := *chore
	visibleChore.LabelsV2 = &labels
	event := *e
	switch data := e.Data.(type) {
	case *ChoreCreatedData:
		visibleData := *data
		visibleData.Chore = &visibleChore
		event.Data = &visibleData
	case *ChoreEventData:
		visibleData := *data
		visibleData.Chore = &visibleChore
		event.Data = &visibleData
	}
	return &event
}
