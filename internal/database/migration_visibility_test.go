package database

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	pModel "donetick.com/core/internal/project/model"
)

// Upgrading a database created by v0.1.80 must not change who can see anything:
// the new shared_with_members column defaults to false, so private projects stay
// owner-only and circle projects stay circle-wide.
func TestUpgradeKeepsExistingProjectVisibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "old.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Schema of the projects table as v0.1.80 created it.
	if err := db.Exec(`CREATE TABLE projects (
		id integer PRIMARY KEY AUTOINCREMENT, name text NOT NULL, description text, color text, icon text,
		circle_id integer NOT NULL, created_by integer NOT NULL, created_at datetime, updated_at datetime,
		is_default numeric DEFAULT false, is_private numeric DEFAULT false)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO projects (id, name, circle_id, created_by, is_private) VALUES
		(1, 'mine', 1, 1, true), (2, 'shared', 1, 1, false)`).Error; err != nil {
		t.Fatal(err)
	}

	if err := Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	var projects []pModel.Project
	if err := db.Order("id").Find(&projects).Error; err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects", len(projects))
	}
	if v := projects[0].Visibility(); v != pModel.ProjectVisibilityPrivate {
		t.Errorf("private project became %q", v)
	}
	if v := projects[1].Visibility(); v != pModel.ProjectVisibilityCircle {
		t.Errorf("circle project became %q", v)
	}
	var members int64
	if err := db.Model(&pModel.ProjectMember{}).Count(&members).Error; err != nil {
		t.Fatalf("project_members table missing: %v", err)
	}
	if members != 0 {
		t.Errorf("migration created %d member rows", members)
	}
}
