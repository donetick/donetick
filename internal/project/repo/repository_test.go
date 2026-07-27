package repo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/database"
	pModel "donetick.com/core/internal/project/model"
)

const (
	testCircleID = 1
	testOwnerID  = 1
	testOtherID  = 2
)

func newTestRepo(t *testing.T) (*ProjectRepository, *gorm.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test_projects.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	return NewProjectRepository(db, cfg, chRepo.NewChoreRepository(db, cfg)), db
}

func createTestProject(t *testing.T, db *gorm.DB, name string, createdBy int, isPrivate bool) *pModel.Project {
	t.Helper()

	project := &pModel.Project{
		Name:      name,
		CircleID:  testCircleID,
		CreatedBy: createdBy,
		IsPrivate: isPrivate,
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("failed to create project %q: %v", name, err)
	}
	return project
}

func createTestChore(t *testing.T, db *gorm.DB, projectID int, isPrivate bool) *chModel.Chore {
	t.Helper()

	chore := &chModel.Chore{
		Name:      "chore",
		CircleID:  testCircleID,
		CreatedBy: testOwnerID,
		ProjectID: &projectID,
		IsPrivate: isPrivate,
	}
	if err := db.Create(chore).Error; err != nil {
		t.Fatalf("failed to create chore: %v", err)
	}
	return chore
}

func reloadChore(t *testing.T, db *gorm.DB, choreID int) *chModel.Chore {
	t.Helper()

	var chore chModel.Chore
	if err := db.First(&chore, choreID).Error; err != nil {
		t.Fatalf("failed to reload chore %d: %v", choreID, err)
	}
	return &chore
}

func TestGetCircleProjectsHidesOtherUsersPrivateProjects(t *testing.T) {
	r, db := newTestRepo(t)

	createTestProject(t, db, "shared", testOwnerID, false)
	createTestProject(t, db, "mine", testOwnerID, true)
	createTestProject(t, db, "theirs", testOtherID, true)

	projects, err := r.GetCircleProjects(context.Background(), testCircleID, testOwnerID)
	if err != nil {
		t.Fatalf("GetCircleProjects failed: %v", err)
	}

	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	if len(names) != 2 || names[0] != "mine" || names[1] != "shared" {
		t.Errorf("got projects %v, want [mine shared]", names)
	}
}

func TestUpdateProjectPropagatesPrivacyToChores(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()

	project := createTestProject(t, db, "mine", testOwnerID, false)
	publicChore := createTestChore(t, db, project.ID, false)
	privateChore := createTestChore(t, db, project.ID, true)
	versionBefore := reloadChore(t, db, publicChore.ID).SyncVersion

	isPrivate := true
	if err := r.UpdateProject(ctx, &pModel.Project{ID: project.ID, Name: project.Name}, &isPrivate, testOwnerID, testCircleID); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	if updated := reloadChore(t, db, publicChore.ID); !updated.IsPrivate {
		t.Error("chore in a private project should have been made private")
	} else if updated.SyncVersion <= versionBefore {
		t.Errorf("sync_version = %d, want > %d so delta sync clients see the change", updated.SyncVersion, versionBefore)
	}

	// Turning the project public again syncs its chores back, the project wins over
	// the chore's own flag in both directions.
	isPrivate = false
	if err := r.UpdateProject(ctx, &pModel.Project{ID: project.ID, Name: project.Name}, &isPrivate, testOwnerID, testCircleID); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}
	if updated := reloadChore(t, db, privateChore.ID); updated.IsPrivate {
		t.Error("chore should have been made public with its project")
	}
}

func TestUpdateProjectKeepsPrivacyWhenFlagOmitted(t *testing.T) {
	r, db := newTestRepo(t)

	project := createTestProject(t, db, "mine", testOwnerID, true)
	chore := createTestChore(t, db, project.ID, true)

	if err := r.UpdateProject(context.Background(), &pModel.Project{ID: project.ID, Name: "renamed"}, nil, testOwnerID, testCircleID); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	updatedProject, err := r.GetProjectByID(context.Background(), project.ID, testCircleID)
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if !updatedProject.IsPrivate {
		t.Error("omitting isPrivate should keep the project private")
	}
	if updated := reloadChore(t, db, chore.ID); !updated.IsPrivate {
		t.Error("chores should be untouched when the project flag doesn't change")
	}
}
