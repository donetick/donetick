package user

import (
	"context"
	"testing"

	"donetick.com/core/config"
	"donetick.com/core/internal/database"
	uModel "donetick.com/core/internal/user/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestUserRepo(t *testing.T) *UserRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewUserRepository(db, &config.Config{IsDoneTickDotCom: false})
}

func TestCreateUserNormalizesEmailAndUsername(t *testing.T) {
	r := newTestUserRepo(t)
	created, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "  Alice  ",
		DisplayName: "Alice",
		Email:       "  Alice@Example.COM ",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.Username != "alice" {
		t.Fatalf("persisted username = %q, want alice", created.Username)
	}
	if created.Email != "alice@example.com" {
		t.Fatalf("persisted email = %q, want alice@example.com", created.Email)
	}

	// Confirm the row on disk is canonical, not just the returned struct.
	var stored uModel.User
	if err := r.db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Username != "alice" || stored.Email != "alice@example.com" {
		t.Fatalf("stored row not canonical: username=%q email=%q", stored.Username, stored.Email)
	}
}

func TestGetUserByUsernameCaseInsensitive(t *testing.T) {
	r := newTestUserRepo(t)
	if _, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	for _, in := range []string{"alice", "Alice", "ALICE", "AlIcE", " alice "} {
		u, err := r.GetUserByUsername(context.Background(), in)
		if err != nil {
			t.Fatalf("lookup %q: %v", in, err)
		}
		if u.Username != "alice" {
			t.Fatalf("lookup %q resolved to %q, want alice", in, u.Username)
		}
	}
}

func TestFindByEmailCaseInsensitive(t *testing.T) {
	r := newTestUserRepo(t)
	if _, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "bob",
		DisplayName: "Bob",
		Email:       "Bob@Example.com",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	for _, in := range []string{"bob@example.com", "BOB@EXAMPLE.COM", "Bob@Example.com", " bob@example.com "} {
		u, err := r.FindByEmail(context.Background(), in)
		if err != nil {
			t.Fatalf("lookup %q: %v", in, err)
		}
		if u.Username != "bob" {
			t.Fatalf("lookup %q resolved to %q, want bob", in, u.Username)
		}
	}
}

func TestCreateUserRejectsCaseVariantDuplicateEmail(t *testing.T) {
	r := newTestUserRepo(t)
	if _, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "carol",
		DisplayName: "Carol",
		Email:       "carol@example.com",
	}); err != nil {
		t.Fatalf("create first user: %v", err)
	}
	if _, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "carol2",
		DisplayName: "Carol Two",
		Email:       "CAROL@example.com",
	}); err == nil {
		t.Fatal("expected unique-constraint error creating case-variant duplicate email, got nil")
	}
}

func TestPasswordResetRoundTripCaseInsensitive(t *testing.T) {
	r := newTestUserRepo(t)
	if _, err := r.CreateUser(context.Background(), &uModel.User{
		Username:    "dave",
		DisplayName: "Dave",
		Email:       "dave@example.com",
		Password:    "old-hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := r.SetPasswordResetToken(context.Background(), "Dave@Example.com", "tok-123"); err != nil {
		t.Fatalf("set reset token: %v", err)
	}
	if err := r.UpdatePasswordByToken(context.Background(), "DAVE@EXAMPLE.COM", "tok-123", "new-hash"); err != nil {
		t.Fatalf("redeem reset token: %v", err)
	}

	var stored uModel.User
	if err := r.db.Where("email = ?", "dave@example.com").First(&stored).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Password != "new-hash" {
		t.Fatalf("password not updated, got %q", stored.Password)
	}
}
