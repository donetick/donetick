package chore

import (
	"testing"
	"time"
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
