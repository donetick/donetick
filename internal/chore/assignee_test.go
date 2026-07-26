package chore

import (
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	circle "donetick.com/core/internal/circle/model"
)

// circleOf builds the circle-member list checkNextAssignee falls back to when a chore
// has no explicit assignees (an "Anyone" chore).
func circleOf(userIDs ...int) []*circle.UserCircleDetail {
	members := make([]*circle.UserCircleDetail, 0, len(userIDs))
	for _, userID := range userIDs {
		members = append(members, &circle.UserCircleDetail{
			UserCircle: circle.UserCircle{UserID: userID},
		})
	}
	return members
}

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

	nextAssignee, err := checkNextAssignee(chore, history, assignee, nil)
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

	nextAssignee, err := checkNextAssignee(chore, history, assigneeB, nil)
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

	nextAssignee, err := checkNextAssignee(chore, history, assignee, nil)
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

func TestCheckNextAssigneeAnyoneChoreRotatesAcrossCircle(t *testing.T) {
	// An "Anyone" chore has no explicit assignees. Rather than dropping the assignment
	// (which would also silence notifications, since the planner only emits for an
	// assigned user), round_robin should rotate across the whole circle.

	userA, userB, userC := 1, 2, 3

	chore := &chModel.Chore{
		ID:             7,
		AssignedTo:     intPtr(userB),
		AssignStrategy: chModel.AssignmentStrategyRoundRobin,
		Assignees:      nil,
	}

	nextAssignee, err := checkNextAssignee(chore, nil, userB, circleOf(userA, userB, userC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextAssignee == nil {
		t.Fatal("expected a next assignee, got nil")
	}
	if *nextAssignee != userC {
		t.Errorf("expected rotation to the next circle member %d, got %d", userC, *nextAssignee)
	}

	// The fallback pool must not leak back onto the chore: an "Anyone" chore stays
	// assignee-less in the database.
	if len(chore.Assignees) != 0 {
		t.Errorf("checkNextAssignee mutated chore.Assignees to %d entries", len(chore.Assignees))
	}
}

func TestCheckNextAssigneeAnyoneChoreDoesNotDropAssignment(t *testing.T) {
	// Regression test: previously these strategies left nextAssignee nil for an
	// assignee-less chore, and the nil was persisted verbatim, wiping assigned_to.
	strategies := []chModel.AssignmentStrategy{
		chModel.AssignmentStrategyLeastAssigned,
		chModel.AssignmentStrategyLeastCompleted,
		chModel.AssignmentStrategyRandom,
		chModel.AssignmentStrategyRandomExceptLastAssigned,
		chModel.AssignmentStrategyRoundRobin,
	}

	assigned := 2
	members := circleOf(1, assigned, 3)

	for _, strategy := range strategies {
		chore := &chModel.Chore{
			ID:             7,
			AssignedTo:     intPtr(assigned),
			AssignStrategy: strategy,
			Assignees:      nil,
		}

		nextAssignee, err := checkNextAssignee(chore, nil, assigned, members)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", strategy, err)
			continue
		}
		if nextAssignee == nil {
			t.Errorf("%s: assignment was dropped for an Anyone chore", strategy)
			continue
		}
		// Whichever member is picked, it must be someone in the circle.
		inCircle := false
		for _, member := range members {
			if member.UserID == *nextAssignee {
				inCircle = true
				break
			}
		}
		if !inCircle {
			t.Errorf("%s: next assignee %d is not a circle member", strategy, *nextAssignee)
		}
	}
}

func TestCheckNextAssigneeNoAssigneeStrategyStaysUnassigned(t *testing.T) {
	// no_assignee must keep returning nil: the circle fallback does not apply to it.
	chore := &chModel.Chore{
		ID:             7,
		AssignStrategy: chModel.AssignmentStrategyNoAssignee,
		Assignees:      nil,
	}

	nextAssignee, err := checkNextAssignee(chore, nil, 1, circleOf(1, 2, 3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextAssignee != nil {
		t.Errorf("expected no_assignee to stay unassigned, got %d", *nextAssignee)
	}
}

func TestCheckNextAssigneeRandomExceptLastKeepsSoleCandidate(t *testing.T) {
	// When the last assignee is the only candidate there is nobody else to rotate to,
	// so the assignment must be kept rather than dropped. Covers both a single-member
	// circle ("Anyone" chore) and a chore with a single explicit assignee.
	assigned := 2

	t.Run("single member circle", func(t *testing.T) {
		chore := &chModel.Chore{
			ID:             7,
			AssignedTo:     intPtr(assigned),
			AssignStrategy: chModel.AssignmentStrategyRandomExceptLastAssigned,
			Assignees:      nil,
		}

		nextAssignee, err := checkNextAssignee(chore, nil, assigned, circleOf(assigned))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if nextAssignee == nil || *nextAssignee != assigned {
			t.Errorf("expected sole circle member %d to be kept, got %v", assigned, nextAssignee)
		}
	})

	t.Run("single explicit assignee", func(t *testing.T) {
		chore := &chModel.Chore{
			ID:             7,
			AssignedTo:     intPtr(assigned),
			AssignStrategy: chModel.AssignmentStrategyRandomExceptLastAssigned,
			Assignees: []chModel.ChoreAssignees{
				{ChoreID: 7, UserID: assigned},
			},
		}

		nextAssignee, err := checkNextAssignee(chore, nil, assigned, circleOf(1, assigned, 3))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if nextAssignee == nil || *nextAssignee != assigned {
			t.Errorf("expected sole assignee %d to be kept, got %v", assigned, nextAssignee)
		}
	})
}
