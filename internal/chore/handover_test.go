package chore

import (
	"testing"

	"github.com/stretchr/testify/require"

	chModel "donetick.com/core/internal/chore/model"
)

func TestHandOverToProjectOwner(t *testing.T) {
	const leaving, owner, colleague = 2, 1, 3
	t.Run("sole assignee: owner takes over", func(t *testing.T) {
		ch := &chModel.Chore{ID: 1, CreatedBy: leaving, AssignStrategy: chModel.AssignmentStrategyRandom,
			AssignedTo: intPtr(leaving), Assignees: []chModel.ChoreAssignees{{ChoreID: 1, UserID: leaving}}}
		HandOverToProjectOwner(ch, leaving, owner)
		require.Equal(t, owner, ch.CreatedBy)
		require.Equal(t, owner, *ch.AssignedTo)
		require.Equal(t, []chModel.ChoreAssignees{{ChoreID: 1, UserID: owner}}, ch.Assignees)
	})
	t.Run("other assignees stay, current assignee moves to owner", func(t *testing.T) {
		ch := &chModel.Chore{ID: 1, CreatedBy: leaving, AssignStrategy: chModel.AssignmentStrategyRoundRobin,
			AssignedTo: intPtr(leaving), Assignees: []chModel.ChoreAssignees{{ChoreID: 1, UserID: leaving}, {ChoreID: 1, UserID: colleague}}}
		HandOverToProjectOwner(ch, leaving, owner)
		require.Equal(t, []chModel.ChoreAssignees{{ChoreID: 1, UserID: colleague}}, ch.Assignees)
		require.Equal(t, owner, *ch.AssignedTo)
	})
	t.Run("assigned to a colleague: untouched", func(t *testing.T) {
		ch := &chModel.Chore{ID: 1, CreatedBy: leaving, AssignStrategy: chModel.AssignmentStrategyRandom,
			AssignedTo: intPtr(colleague), Assignees: []chModel.ChoreAssignees{{ChoreID: 1, UserID: colleague}}}
		HandOverToProjectOwner(ch, leaving, owner)
		require.Equal(t, colleague, *ch.AssignedTo)
		require.Len(t, ch.Assignees, 1)
	})
	t.Run("deliberately unassigned stays unassigned", func(t *testing.T) {
		ch := &chModel.Chore{ID: 1, CreatedBy: leaving, AssignStrategy: chModel.AssignmentStrategyNoAssignee}
		HandOverToProjectOwner(ch, leaving, owner)
		require.Nil(t, ch.AssignedTo)
		require.Empty(t, ch.Assignees)
		require.Equal(t, owner, ch.CreatedBy)
	})
	t.Run("completion actions on the leaving user's Things are dropped", func(t *testing.T) {
		ch := &chModel.Chore{ID: 1, CreatedBy: leaving, AssignStrategy: chModel.AssignmentStrategyRandom,
			AssignedTo: intPtr(leaving), Assignees: []chModel.ChoreAssignees{{ChoreID: 1, UserID: leaving}},
			CompletionActions: []chModel.CompletionAction{{ThingID: 7, Operation: "set", Value: "true"}}}
		HandOverToProjectOwner(ch, leaving, owner)
		require.Empty(t, ch.CompletionActions)
	})
}
