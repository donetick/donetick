package realtime

import (
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	lModel "donetick.com/core/internal/label/model"
	"github.com/stretchr/testify/require"
)

func TestEventsHidePrivateLabelsPerRecipient(t *testing.T) {
	labels := []lModel.Label{{ID: 1, CreatedBy: 1}, {ID: 2, CreatedBy: 1, IsShared: true}}
	chore := &chModel.Chore{ID: 1, LabelsV2: &labels}
	for _, event := range []*Event{
		NewChoreCreatedEvent(chore, nil),
		NewChoreUpdatedEvent(chore, nil, nil, nil),
	} {
		other := event.forUser(2)
		var visible *chModel.Chore
		switch data := other.Data.(type) {
		case *ChoreCreatedData:
			visible = data.Chore
		case *ChoreEventData:
			visible = data.Chore
		}
		require.Len(t, *visible.LabelsV2, 1)
		require.Equal(t, 2, (*visible.LabelsV2)[0].ID)
		require.Same(t, event, event.forUser(1))
		require.Len(t, *chore.LabelsV2, 2, "redacting one recipient must not mutate another's payload")
	}
}
