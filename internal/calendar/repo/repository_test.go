package repo

import (
	"path/filepath"
	"testing"
	"time"

	calModel "donetick.com/core/internal/calendar/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const testUserID = 7

func newTestRepo(t *testing.T) (*CalendarRepository, *gorm.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test_calendar_repo.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&calModel.CalendarToken{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	return NewCalendarRepository(db), db
}

func TestGetByUserID_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.GetByUserID(t.Context(), testUserID)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestGetByToken_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.GetByToken(t.Context(), "nope")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestReplace_KeepsOneTokenPerUser(t *testing.T) {
	repo, db := newTestRepo(t)

	if _, err := repo.Replace(t.Context(), testUserID, "first"); err != nil {
		t.Fatalf("first Replace failed: %v", err)
	}
	if _, err := repo.Replace(t.Context(), testUserID, "second"); err != nil {
		t.Fatalf("second Replace failed: %v", err)
	}

	var count int64
	if err := db.Model(&calModel.CalendarToken{}).Where("user_id = ?", testUserID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 token per user, got %d", count)
	}

	// The superseded token must no longer resolve.
	if _, err := repo.GetByToken(t.Context(), "first"); err != gorm.ErrRecordNotFound {
		t.Errorf("expected the replaced token to be gone, got %v", err)
	}
	current, err := repo.GetByUserID(t.Context(), testUserID)
	if err != nil {
		t.Fatalf("GetByUserID failed: %v", err)
	}
	if current.Token != "second" {
		t.Errorf("expected the newest token, got %q", current.Token)
	}
}

func TestReplace_DoesNotTouchOtherUsers(t *testing.T) {
	repo, _ := newTestRepo(t)

	if _, err := repo.Replace(t.Context(), testUserID, "mine"); err != nil {
		t.Fatalf("Replace failed: %v", err)
	}
	if _, err := repo.Replace(t.Context(), testUserID+1, "theirs"); err != nil {
		t.Fatalf("Replace for other user failed: %v", err)
	}

	if _, err := repo.GetByToken(t.Context(), "mine"); err != nil {
		t.Errorf("another user's rotation revoked this token: %v", err)
	}
}

func TestDeleteByUserID(t *testing.T) {
	repo, _ := newTestRepo(t)

	if _, err := repo.Replace(t.Context(), testUserID, "tok"); err != nil {
		t.Fatalf("Replace failed: %v", err)
	}
	if err := repo.DeleteByUserID(t.Context(), testUserID); err != nil {
		t.Fatalf("DeleteByUserID failed: %v", err)
	}

	if _, err := repo.GetByToken(t.Context(), "tok"); err != gorm.ErrRecordNotFound {
		t.Errorf("expected the revoked token to be gone, got %v", err)
	}
	// Revoking again is a no-op, not an error.
	if err := repo.DeleteByUserID(t.Context(), testUserID); err != nil {
		t.Errorf("repeat DeleteByUserID failed: %v", err)
	}
}

func TestTouchLastUsed_SetsTimestampWhenUnset(t *testing.T) {
	repo, _ := newTestRepo(t)

	stored, err := repo.Replace(t.Context(), testUserID, "tok")
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}
	if err := repo.TouchLastUsed(t.Context(), stored.ID); err != nil {
		t.Fatalf("TouchLastUsed failed: %v", err)
	}

	reloaded, err := repo.GetByToken(t.Context(), "tok")
	if err != nil {
		t.Fatalf("GetByToken failed: %v", err)
	}
	if reloaded.LastUsedAt == nil {
		t.Fatal("expected last_used_at to be set")
	}
}

func TestTouchLastUsed_ThrottlesWithinInterval(t *testing.T) {
	repo, db := newTestRepo(t)

	stored, err := repo.Replace(t.Context(), testUserID, "tok")
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}

	// A recent timestamp must be left alone so polling clients don't cause a
	// write on every feed read.
	recent := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&calModel.CalendarToken{}).Where("id = ?", stored.ID).
		Update("last_used_at", recent).Error; err != nil {
		t.Fatalf("seeding last_used_at failed: %v", err)
	}

	if err := repo.TouchLastUsed(t.Context(), stored.ID); err != nil {
		t.Fatalf("TouchLastUsed failed: %v", err)
	}

	reloaded, err := repo.GetByToken(t.Context(), "tok")
	if err != nil {
		t.Fatalf("GetByToken failed: %v", err)
	}
	if !reloaded.LastUsedAt.Truncate(time.Second).Equal(recent.Truncate(time.Second)) {
		t.Errorf("expected last_used_at to be throttled at %v, got %v", recent, reloaded.LastUsedAt)
	}
}

func TestTouchLastUsed_UpdatesOnceStale(t *testing.T) {
	repo, db := newTestRepo(t)

	stored, err := repo.Replace(t.Context(), testUserID, "tok")
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}

	stale := time.Now().UTC().Add(-2 * TouchInterval)
	if err := db.Model(&calModel.CalendarToken{}).Where("id = ?", stored.ID).
		Update("last_used_at", stale).Error; err != nil {
		t.Fatalf("seeding last_used_at failed: %v", err)
	}

	if err := repo.TouchLastUsed(t.Context(), stored.ID); err != nil {
		t.Fatalf("TouchLastUsed failed: %v", err)
	}

	reloaded, err := repo.GetByToken(t.Context(), "tok")
	if err != nil {
		t.Fatalf("GetByToken failed: %v", err)
	}
	if !reloaded.LastUsedAt.After(stale) {
		t.Errorf("expected a stale last_used_at to be refreshed, still %v", reloaded.LastUsedAt)
	}
}
