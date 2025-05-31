package database

import (
	"embed"
	"fmt"
	"os"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/points"
	storageModel "donetick.com/core/internal/storage/model"
	stModel "donetick.com/core/internal/subtask/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	migrations "donetick.com/core/migrations"
	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func Migration(db *gorm.DB) error {
	if err := db.AutoMigrate(uModel.User{}, chModel.Chore{},
		chModel.ChoreHistory{},
		cModel.Circle{},
		cModel.UserCircle{},
		chModel.ChoreAssignees{},
		nModel.Notification{},
		uModel.UserPasswordReset{},
		uModel.MFASession{}, // Add MFA session model
		tModel.Thing{},
		tModel.ThingChore{},
		tModel.ThingHistory{},
		uModel.APIToken{},
		uModel.UserNotificationTarget{},
		chModel.Label{},
		chModel.ChoreLabels{},
		migrations.Migration{},
		pModel.PointsHistory{},
		stModel.SubTask{},
		storageModel.StorageFile{},
		storageModel.StorageUsage{},
	); err != nil {
		return err
	}

	return nil
}

func MigrationScripts(gormDB *gorm.DB, cfg *config.Config) error {
	migrations := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: embeddedMigrations,
		Root:       "migrations",
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
