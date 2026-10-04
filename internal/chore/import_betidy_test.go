package chore

import (
	"testing"
	"time"
)

func TestBetidyPriority(t *testing.T) {
	cases := map[int]int{0: 0, 1: 2, 2: 1}
	for important, want := range cases {
		if got := betidyPriority(important); got != want {
			t.Errorf("betidyPriority(%d) = %d, want %d", important, got, want)
		}
	}
}

func TestBetidyFrequency(t *testing.T) {
	loc := time.UTC

	ft, fq, meta := betidyFrequency(betidyTask{Type: "INTERVAL", IntervalUnit: "week", IntervalCount: 2}, "UTC", loc)
	if ft != "interval" || fq != 2 || meta.Unit == nil || *meta.Unit != "weeks" {
		t.Fatalf("interval mapping = (%v, %d, %+v)", ft, fq, meta)
	}

	// intervalCount < 1 defaults to 1
	_, fq, _ = betidyFrequency(betidyTask{Type: "INTERVAL", IntervalUnit: "month", IntervalCount: 0}, "UTC", loc)
	if fq != 1 {
		t.Fatalf("default frequency = %d, want 1", fq)
	}

	ft, _, meta = betidyFrequency(betidyTask{Type: "DATE"}, "UTC", loc)
	if ft != "once" || meta.Unit != nil {
		t.Fatalf("once mapping = (%v, %+v)", ft, meta)
	}
}

func TestBetidyNextDueRollsForwardAndSnapsWeekday(t *testing.T) {
	loc := time.UTC
	today := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC) // a Thursday

	// Weekly on Tuesday (BeTidy day 2), anchored well in the past.
	task := betidyTask{Type: "INTERVAL", IntervalUnit: "week", IntervalCount: 1, TodoDate: "2024-09-23", Days: []int{2}}
	due := betidyNextDue(task, today, loc)
	if due == nil {
		t.Fatal("nextDue is nil")
	}
	if due.Before(today) {
		t.Errorf("due %v was not rolled forward past %v", due, today)
	}
	if due.Weekday() != time.Tuesday {
		t.Errorf("due weekday = %v, want Tuesday", due.Weekday())
	}
}

func TestBetidyNextDueOnceInPastMovesToToday(t *testing.T) {
	loc := time.UTC
	today := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	due := betidyNextDue(betidyTask{Type: "DATE", TodoDate: "2024-01-01"}, today, loc)
	if due == nil || due.Before(today) {
		t.Fatalf("once-in-past should move to today, got %v", due)
	}
}

func TestBetidyDescription(t *testing.T) {
	profiles := map[string]string{"p1": "Alice", "p2": "Bob"}
	task := betidyTask{Type: "INTERVAL", IntervalUnit: "week", IntervalCount: 1, Description: "note", Assigned: []string{"p1", "p2"}, Effort: 2}
	got := betidyDescription(task, "Kitchen", profiles)
	want := "note\n[BeTidy] Room: Kitchen · Repeats: every week · Assignee: Alice, Bob · Effort: 2"
	if got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}
