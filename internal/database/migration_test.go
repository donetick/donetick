package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"donetick.com/core/config"
	"donetick.com/core/migrations"
)

func TestMigrationScriptsSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_donetick.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := Migration(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	// main.go ignores the error from migrations.Run; some legacy Go migrations
	// (e.g. 20241123_migrate_label_to_labels_v2) fail on a fresh database.
	_ = migrations.Run(context.Background(), db)

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	if err := MigrationScripts(db, cfg); err != nil {
		t.Fatalf("MigrationScripts failed: %v", err)
	}

	// Running again must be idempotent (already-applied migrations skipped).
	if err := MigrationScripts(db, cfg); err != nil {
		t.Fatalf("MigrationScripts second run failed: %v", err)
	}
}
