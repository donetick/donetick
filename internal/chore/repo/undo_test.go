package chore

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"gorm.io/gorm"
)

func TestGetUserLastChoreActionFindsRecentActionOutsideUTC(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("test-local", -5*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	repo, db := newTestChoreRepo(t)
	now := time.Now()
	history := &chModel.ChoreHistory{
		ChoreID:     41,
		CompletedBy: testOwnerID,
		PerformedAt: &now,
		Status:      chModel.ChoreHistoryStatusCompleted,
	}
	if err := db.Create(history).Error; err != nil {
		t.Fatalf("failed to create history: %v", err)
	}

	got, err := repo.GetUserLastChoreAction(context.Background(), history.ChoreID, testOwnerID, 5*time.Minute)
	if err != nil {
		t.Fatalf("GetUserLastChoreAction returned error: %v", err)
	}
	if got.ID != history.ID {
		t.Fatalf("history ID = %d, want %d", got.ID, history.ID)
	}
}

func TestGetUserLastChoreActionFindsRecentlyCompletedTimerHistory(t *testing.T) {
	repo, db := newTestChoreRepo(t)
	now := time.Now()
	startedAt := now.Add(-time.Hour)

	history := &chModel.ChoreHistory{
		ChoreID:     42,
		CompletedBy: testOwnerID,
		PerformedAt: &startedAt,
		CreatedAt:   startedAt,
		UpdatedAt:   &startedAt,
		Status:      chModel.ChoreHistoryStatusStarted,
	}
	if err := db.Create(history).Error; err != nil {
		t.Fatalf("failed to create started history: %v", err)
	}

	// Completing a timed chore updates the existing history row instead of
	// creating a new one, so its creation time remains older than the undo
	// window even though the completion just happened.
	history.PerformedAt = &now
	history.Status = chModel.ChoreHistoryStatusCompleted
	if err := db.Save(history).Error; err != nil {
		t.Fatalf("failed to complete history: %v", err)
	}

	got, err := repo.GetUserLastChoreAction(context.Background(), history.ChoreID, testOwnerID, 5*time.Minute)
	if err != nil {
		t.Fatalf("GetUserLastChoreAction returned error: %v", err)
	}
	if got.ID != history.ID {
		t.Fatalf("history ID = %d, want %d", got.ID, history.ID)
	}
}

func TestGetUserLastChoreActionRejectsStaleAction(t *testing.T) {
	repo, db := newTestChoreRepo(t)
	staleTime := time.Now().Add(-time.Hour)

	history := &chModel.ChoreHistory{
		ChoreID:     42,
		CompletedBy: testOwnerID,
		PerformedAt: &staleTime,
		CreatedAt:   staleTime,
		UpdatedAt:   &staleTime,
		Status:      chModel.ChoreHistoryStatusCompleted,
	}
	if err := db.Create(history).Error; err != nil {
		t.Fatalf("failed to create stale history: %v", err)
	}

	_, err := repo.GetUserLastChoreAction(context.Background(), history.ChoreID, testOwnerID, 5*time.Minute)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}
