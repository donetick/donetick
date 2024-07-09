package database

import (
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"gorm.io/gorm/logger"

	// "github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"donetick.com/core/config"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	switch cfg.Database.Type {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name)
		for i := 0; i <= 30; i++ {
			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Info),
			})
			if err == nil {
				break
			}
			logging.DefaultLogger().Warnf("failed to open database: %v", err)
			time.Sleep(500 * time.Millisecond)
		}

	default:

		path := os.Getenv("DT_SQLITE_PATH")
		if path == "" {
			db, err = gorm.Open(sqlite.Open("donetick.db"), &gorm.Config{})
		} else {
			db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
		}

	}

	if err != nil {
		return nil, err
	}
	return db, nil
}
