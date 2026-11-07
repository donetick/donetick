package migrations

import (
	"context"
	"testing"

	uModel "donetick.com/core/internal/user/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&uModel.User{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	return db
}

// seedLegacyUser inserts a canonical user (via the normalising hook) and then
// rewrites its identity columns with raw SQL to simulate a row that predates the
// hook — UpdateColumn skips hooks so the mixed case survives.
func seedLegacyUser(t *testing.T, db *gorm.DB, username, email, legacyUsername, legacyEmail string) int {
	t.Helper()
	u := &uModel.User{Username: username, DisplayName: username, Email: email}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
	if err := db.Model(&uModel.User{}).Where("id = ?", u.ID).
		UpdateColumns(map[string]interface{}{"username": legacyUsername, "email": legacyEmail}).Error; err != nil {
		t.Fatalf("rewrite legacy columns for %q: %v", username, err)
	}
	return u.ID
}

func TestCanonicalizeUserIdentityBackfill(t *testing.T) {
	db := newMigrationTestDB(t)

	// Non-colliding legacy row with mixed-case username and email.
	keepID := seedLegacyUser(t, db, "keep", "keep@example.com", "Keep", "Keep@Example.com")
	// An already-canonical row that a mixed-case row will collide with.
	dupID := seedLegacyUser(t, db, "dup", "dup@example.com", "dup", "dup@example.com")
	// Legacy row that lowercases onto dup@example.com — must be left untouched.
	collideID := seedLegacyUser(t, db, "collide", "placeholder@example.com", "Collide", "DUP@example.com")

	if err := (CanonicalizeUserIdentity20260620{}).Up(context.Background(), db); err != nil {
		t.Fatalf("migration Up: %v", err)
	}

	get := func(id int) uModel.User {
		var u uModel.User
		if err := db.First(&u, id).Error; err != nil {
			t.Fatalf("reload %d: %v", id, err)
		}
		return u
	}

	if u := get(keepID); u.Email != "keep@example.com" || u.Username != "keep" {
		t.Fatalf("non-colliding row not canonicalised: username=%q email=%q", u.Username, u.Email)
	}
	if u := get(dupID); u.Email != "dup@example.com" {
		t.Fatalf("canonical row changed unexpectedly: email=%q", u.Email)
	}
	if u := get(collideID); u.Email != "DUP@example.com" {
		t.Fatalf("colliding row was altered (would violate uniqueness): email=%q", u.Email)
	}
}
