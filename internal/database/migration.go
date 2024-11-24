package database

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	migrations "donetick.com/core/migrations"
	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/gorm"
)

func Migration(db *gorm.DB) error {
	if err := db.AutoMigrate(uModel.User{}, chModel.Chore{},
		chModel.ChoreHistory{},
		cModel.Circle{},
		cModel.UserCircle{},
		chModel.ChoreAssignees{},
		nModel.Notification{},
		uModel.UserPasswordReset{},
		tModel.Thing{},
		tModel.ThingChore{},
		tModel.ThingHistory{},
		uModel.APIToken{},
		uModel.UserNotificationTarget{},
		chModel.Label{},
		chModel.ChoreLabels{},
		migrations.Migration{},
	); err != nil {
		return err
	}

	return nil
}

func MigrationScripts(gormDB *gorm.DB, cfg *config.Config) error {
	migrations := &migrate.FileMigrationSource{
		Dir: migrationDir(),
	}

	path := os.Getenv("DT_SQLITE_PATH")
	if path == "" {
		path = "donetick.db"
	}

	db, err := gormDB.DB()
	if err != nil {
		return err
	}

	n, err := migrate.Exec(db, "sqlite3", migrations, migrate.Up)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %d migrations!\n", n)
	return nil
}

func migrationDir() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(filename), "../../migrations")
}
