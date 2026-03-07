package chore

import (
	"testing"

	chModel "donetick.com/core/internal/chore/model"
)

func TestCheckNextAssigneeLeastCompletedIgnoresNonAssignees(t *testing.T) {
	// Regression test: least_completed strategy must only consider users
	// who are current assignees. Previously, any user who had ever completed
	// the chore would be added to the candidate map during counting,
	// causing non-assignees with fewer completions to be selected.

	assignee := 2
	nonAssignee := 1

	chore := &chModel.Chore{
		AssignedTo:     intPtr(assignee),
		AssignStrategy: chModel.AssignmentStrategyLeastCompleted,
		Assignees: []chModel.ChoreAssignees{
			{ChoreID: 1, UserID: assignee},
		},
	}

	// Non-assignee completed once, assignee completed twice.
	// Without the fix, non-assignee wins due to fewer completions.
	history := []*chModel.ChoreHistory{
		{CompletedBy: nonAssignee, AssignedTo: intPtr(assignee)},
		{CompletedBy: assignee, AssignedTo: intPtr(assignee)},
		{CompletedBy: assignee, AssignedTo: intPtr(assignee)},
	}

	nextAssignee, err := checkNextAssignee(chore, history, assignee)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextAssignee == nil {
		t.Fatal("expected a next assignee, got nil")
	}
	if *nextAssignee != assignee {
		t.Errorf("expected next assignee to be %d (the only assignee), got %d (a non-assignee)",
			assignee, *nextAssignee)
	}
}

func TestCheckNextAssigneeLeastCompletedPicksFewestAmongAssignees(t *testing.T) {
	// With multiple valid assignees, least_completed should pick the one
	// with fewer completions, ignoring completions by non-assignees.

	assigneeA := 1
	assigneeB := 2
	nonAssignee := 99

	chore := &chModel.Chore{
		AssignedTo:     intPtr(assigneeB),
		AssignStrategy: chModel.AssignmentStrategyLeastCompleted,
		Assignees: []chModel.ChoreAssignees{
			{ChoreID: 1, UserID: assigneeA},
			{ChoreID: 1, UserID: assigneeB},
		},
	}

	// assigneeA: 1 completion, assigneeB: 2 completions, nonAssignee: 1 completion (ignored)
	history := []*chModel.ChoreHistory{
		{CompletedBy: nonAssignee, AssignedTo: intPtr(assigneeA)},
		{CompletedBy: assigneeA, AssignedTo: intPtr(assigneeA)},
		{CompletedBy: assigneeB, AssignedTo: intPtr(assigneeB)},
		{CompletedBy: assigneeB, AssignedTo: intPtr(assigneeB)},
	}

	nextAssignee, err := checkNextAssignee(chore, history, assigneeB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextAssignee == nil {
		t.Fatal("expected a next assignee, got nil")
	}
	if *nextAssignee != assigneeA {
		t.Errorf("expected next assignee to be %d (1 completion), got %d", assigneeA, *nextAssignee)
	}
}

func TestCheckNextAssigneeLeastAssignedIgnoresNonAssignees(t *testing.T) {
	// Verify that least_assigned also correctly ignores non-assignees.

	assignee := 2
	nonAssignee := 1

	chore := &chModel.Chore{
		AssignedTo:     intPtr(nonAssignee),
		AssignStrategy: chModel.AssignmentStrategyLeastAssigned,
		Assignees: []chModel.ChoreAssignees{
			{ChoreID: 1, UserID: assignee},
		},
	}

	history := []*chModel.ChoreHistory{
		{CompletedBy: nonAssignee, AssignedTo: intPtr(nonAssignee)},
		{CompletedBy: assignee, AssignedTo: intPtr(assignee)},
		{CompletedBy: assignee, AssignedTo: intPtr(assignee)},
	}

	nextAssignee, err := checkNextAssignee(chore, history, assignee)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextAssignee == nil {
		t.Fatal("expected a next assignee, got nil")
	}
	if *nextAssignee != assignee {
		t.Errorf("expected next assignee to be %d (the only assignee), got %d",
			assignee, *nextAssignee)
	}
}
