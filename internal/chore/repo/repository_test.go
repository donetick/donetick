package chore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/project/model"
)

const (
	testCircleID = 1
	testOwnerID  = 1
	testOtherID  = 2
)

func newTestChoreRepo(t *testing.T) (*ChoreRepository, *gorm.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test_chores.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return NewChoreRepository(db, cfg), db
}

func createProject(t *testing.T, db *gorm.DB, createdBy int, isPrivate bool) *pModel.Project {
	t.Helper()

	project := &pModel.Project{
		Name:      "project",
		CircleID:  testCircleID,
		CreatedBy: createdBy,
		IsPrivate: isPrivate,
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	return project
}

// createChore inserts a chore, optionally inside a project, with the given assignees.
func createChore(t *testing.T, db *gorm.DB, name string, createdBy int, isPrivate bool, projectID *int, assignees ...int) *chModel.Chore {
	t.Helper()

	chore := &chModel.Chore{
		Name:      name,
		CircleID:  testCircleID,
		CreatedBy: createdBy,
		IsPrivate: isPrivate,
		ProjectID: projectID,
		IsActive:  true,
	}
	if err := db.Create(chore).Error; err != nil {
		t.Fatalf("failed to create chore %q: %v", name, err)
	}
	for _, userID := range assignees {
		if err := db.Create(&chModel.ChoreAssignees{ChoreID: chore.ID, UserID: userID}).Error; err != nil {
			t.Fatalf("failed to assign chore %q to %d: %v", name, userID, err)
		}
	}
	return chore
}

func visibleChoreNames(t *testing.T, r *ChoreRepository, userID int) []string {
	t.Helper()

	chores, err := r.GetChores(context.Background(), testCircleID, userID, false, nil, false)
	if err != nil {
		t.Fatalf("GetChores failed: %v", err)
	}
	names := make([]string, 0, len(chores))
	for _, chore := range chores {
		names = append(names, chore.Name)
	}
	return names
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// A user who can't see the project can't see its chores either, not even the ones
// assigned to them: otherwise the API hands out a chore pointing at a project that
// isn't in the user's project list, and it silently disappears from every view.
func TestGetChoresHidesChoresInSomeoneElsesPrivateProject(t *testing.T) {
	r, db := newTestChoreRepo(t)

	project := createProject(t, db, testOtherID, true)
	createChore(t, db, "assigned-to-me", testOtherID, true, &project.ID, testOwnerID)
	createChore(t, db, "not-mine", testOtherID, true, &project.ID, testOtherID)

	names := visibleChoreNames(t, r, testOwnerID)
	if len(names) != 0 {
		t.Errorf("got %v, want no chore from a private project owned by someone else", names)
	}
}

// The flip side: the owner of a private project sees everything in it, including
// chores somebody else created there while the project was still public.
func TestGetChoresShowsEveryChoreInOwnPrivateProject(t *testing.T) {
	r, db := newTestChoreRepo(t)

	project := createProject(t, db, testOwnerID, true)
	createChore(t, db, "created-by-other", testOtherID, true, &project.ID, testOtherID)
	createChore(t, db, "created-by-me", testOwnerID, true, &project.ID, testOwnerID)

	names := visibleChoreNames(t, r, testOwnerID)
	if !contains(names, "created-by-other") || !contains(names, "created-by-me") {
		t.Errorf("got %v, want both chores of the user's own private project", names)
	}
}

// Chore-level privacy keeps working as before outside private projects.
func TestGetChoresKeepsChoreLevelPrivacyOutsidePrivateProjects(t *testing.T) {
	r, db := newTestChoreRepo(t)

	publicProject := createProject(t, db, testOtherID, false)
	createChore(t, db, "public-no-project", testOtherID, false, nil)
	createChore(t, db, "public-in-project", testOtherID, false, &publicProject.ID)
	createChore(t, db, "private-assigned", testOtherID, true, &publicProject.ID, testOwnerID)
	createChore(t, db, "private-theirs", testOtherID, true, &publicProject.ID, testOtherID)

	names := visibleChoreNames(t, r, testOwnerID)
	for _, want := range []string{"public-no-project", "public-in-project", "private-assigned"} {
		if !contains(names, want) {
			t.Errorf("got %v, want it to contain %q", names, want)
		}
	}
	if contains(names, "private-theirs") {
		t.Errorf("got %v, want it to hide a private chore of another user", names)
	}
}

func TestGetChoreAppliesProjectPrivacy(t *testing.T) {
	r, db := newTestChoreRepo(t)
	ctx := context.Background()

	project := createProject(t, db, testOtherID, true)
	chore := createChore(t, db, "theirs", testOtherID, true, &project.ID, testOwnerID)

	if _, err := r.GetChore(ctx, chore.ID, testOwnerID, testCircleID); err == nil {
		t.Error("GetChore should not return a chore from someone else's private project")
	}
	if _, err := r.GetChore(ctx, chore.ID, testOtherID, testCircleID); err != nil {
		t.Errorf("GetChore failed for the project owner: %v", err)
	}
}

func TestSetProjectChoresPrivacyNarrowsAssigneesToOwner(t *testing.T) {
	r, db := newTestChoreRepo(t)
	ctx := context.Background()

	project := createProject(t, db, testOwnerID, false)
	shared := createChore(t, db, "shared", testOwnerID, false, &project.ID, testOwnerID, testOtherID)
	anyone := createChore(t, db, "anyone", testOwnerID, false, &project.ID)
	if err := db.Model(&chModel.Chore{}).Where("id = ?", shared.ID).Update("assigned_to", testOtherID).Error; err != nil {
		t.Fatalf("failed to set assigned_to: %v", err)
	}

	if err := r.SetProjectChoresPrivacy(ctx, db, testCircleID, project.ID, testOwnerID, true); err != nil {
		t.Fatalf("SetProjectChoresPrivacy failed: %v", err)
	}

	assertAssignees(t, db, shared.ID, []int{testOwnerID})
	assertAssignees(t, db, anyone.ID, []int{testOwnerID})

	var updated chModel.Chore
	if err := db.First(&updated, shared.ID).Error; err != nil {
		t.Fatalf("failed to reload chore: %v", err)
	}
	if updated.AssignedTo == nil || *updated.AssignedTo != testOwnerID {
		t.Errorf("assigned_to = %v, want the project owner", updated.AssignedTo)
	}
}

func assertAssignees(t *testing.T, db *gorm.DB, choreID int, want []int) {
	t.Helper()

	var assignees []chModel.ChoreAssignees
	if err := db.Where("chore_id = ?", choreID).Order("user_id asc").Find(&assignees).Error; err != nil {
		t.Fatalf("failed to load assignees: %v", err)
	}
	got := make([]int, 0, len(assignees))
	for _, assignee := range assignees {
		got = append(got, assignee.UserID)
	}
	if len(got) != len(want) {
		t.Fatalf("assignees of chore %d = %v, want %v", choreID, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("assignees of chore %d = %v, want %v", choreID, got, want)
		}
	}
}
