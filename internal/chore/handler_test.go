package chore

import (
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func TestDueDatesDiffer(t *testing.T) {
	dueDate := time.Date(2026, time.July, 28, 3, 59, 0, 0, time.UTC)
	sameInstantInAnotherLocation := dueDate.In(time.FixedZone("offset", -4*60*60))
	differentDueDate := dueDate.Add(time.Minute)

	tests := []struct {
		name       string
		oldDueDate *time.Time
		newDueDate *time.Time
		want       bool
	}{
		{name: "both nil", want: false},
		{name: "old nil", newDueDate: &dueDate, want: true},
		{name: "new nil", oldDueDate: &dueDate, want: true},
		{name: "same pointer", oldDueDate: &dueDate, newDueDate: &dueDate, want: false},
		{name: "equal values at different addresses", oldDueDate: &dueDate, newDueDate: &sameInstantInAnotherLocation, want: false},
		{name: "different values", oldDueDate: &dueDate, newDueDate: &differentDueDate, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dueDatesDiffer(test.oldDueDate, test.newDueDate); got != test.want {
				t.Fatalf("dueDatesDiffer() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsBeforeCompletionWindow(t *testing.T) {
	dueDate := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	window := 2

	tests := []struct {
		name          string
		chore         chModel.Chore
		completedDate time.Time
		want          bool
	}{
		{"no completion window", chModel.Chore{NextDueDate: &dueDate}, dueDate.Add(-48 * time.Hour), false},
		{"window but no due date", chModel.Chore{CompletionWindow: &window}, dueDate, false},
		{"completed before the window opens", chModel.Chore{CompletionWindow: &window, NextDueDate: &dueDate}, dueDate.Add(-3 * time.Hour), true},
		{"completed inside the window", chModel.Chore{CompletionWindow: &window, NextDueDate: &dueDate}, dueDate.Add(-time.Hour), false},
		{"completed after the due date", chModel.Chore{CompletionWindow: &window, NextDueDate: &dueDate}, dueDate.Add(time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBeforeCompletionWindow(&tt.chore, tt.completedDate); got != tt.want {
				t.Errorf("isBeforeCompletionWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}
