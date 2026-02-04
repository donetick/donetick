package database

import (
	"embed"
	"fmt"
	"os"

	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/gorm"

	"donetick.com/core/config"
	sModel "donetick.com/core/external/payment/model"
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	filterModel "donetick.com/core/internal/filter/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/points"
	projModel "donetick.com/core/internal/project/model"
	storageModel "donetick.com/core/internal/storage/model"
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
		sModel.StripeCustomer{},
		sModel.StripeSubscription{},
		sModel.StripeSession{},
		sModel.StripeInvoice{},
		sModel.RevenueCatEvent{},
		sModel.RevenueCatSubscription{},
		sModel.Subscription{},
		uModel.MFASession{},
		uModel.UserSession{},
		tModel.Thing{},
		tModel.ThingChore{},
		tModel.ThingHistory{},
		uModel.APIToken{},
		uModel.UserNotificationTarget{},
		chModel.Label{},
		chModel.ChoreLabels{},
		projModel.Project{},
		filterModel.Filter{},
		migrations.Migration{},
		pModel.PointsHistory{},
		stModel.SubTask{},
		storageModel.StorageFile{},
		storageModel.StorageUsage{},
		chModel.TimeSession{},
		uModel.UserDeviceToken{},
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
	var n int
	if cfg.Database.Type == "sqlite" {

		path := os.Getenv("DT_SQLITE_PATH")
		if path == "" {
			path = "donetick.db"
		}
		n, err = migrate.Exec(db, "sqlite3", migrations, migrate.Up)
	} else if cfg.Database.Type == "postgres" {
		n, err = migrate.Exec(db, "postgres", migrations, migrate.Up)
	}

	n, err = migrate.Exec(db, dialect, migrations, migrate.Up)
	if err != nil {
		return err
	}
	fmt.Printf("Applied %d migrations!\n", n)
	return nil
}
