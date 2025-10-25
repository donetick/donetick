package database

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/plugin/opentelemetry/tracing"

	// "gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"gorm.io/gorm/logger"

	"donetick.com/core/config"
	"donetick.com/core/logging"
	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"
)

// convertLogLevelToGorm converts application log level to GORM log level
func convertLogLevelToGorm(appLogLevel string) logger.LogLevel {
	switch strings.ToLower(appLogLevel) {
	case "debug":
		return logger.Info // GORM's most verbose level for debugging
	case "info":
		return logger.Warn // Show warnings and errors, but not all queries
	case "warn", "warning", "error", "dpanic", "panic", "fatal":
		return logger.Error // Only show errors
	case "silent":
		return logger.Silent // No logging from GORM
	default:
		return logger.Error // Default to error level for production safety
	}
}

func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	switch cfg.Database.Type {
	case "postgres":
		dsn := os.Getenv("DT_POSTGRES_DSN")
		if dsn == "" {
			dsn = fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name)
		}
		for i := 0; i <= 30; i++ {
			// Use GORM logger level based on application log level from config
			gormLogLevel := convertLogLevelToGorm(cfg.Logging.Level)

			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger: logger.Default.LogMode(gormLogLevel),
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
			path = "donetick.db"
		}
		db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})

	}

	if err != nil {
		return nil, err
	}

	// Add OpenTelemetry tracing to GORM
	if err := db.Use(tracing.NewPlugin()); err != nil {
		logging.DefaultLogger().Warnf("failed to enable OpenTelemetry tracing for GORM: %v", err)
	}

	return db, nil
}
