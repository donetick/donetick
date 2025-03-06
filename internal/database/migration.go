package database

import (
	"embed"
	"fmt"

	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/gorm"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/points"
	stModel "donetick.com/core/internal/subtask/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"donetick.com/core/migrations"
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

	var dialect string
	switch cfg.Database.Type {
	case "postgres":
		dialect = "postgres"
	case "sqlite":
		dialect = "sqlite3"
	default:
		return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	db, err := gormDB.DB()
	if err != nil {
		return err
	}

	n, err := migrate.Exec(db, dialect, migrations, migrate.Up)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %d migrations!\n", n)
	return nil
}
