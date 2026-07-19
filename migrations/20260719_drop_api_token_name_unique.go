package migrations

import (
	"context"
	"fmt"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type DropAPITokenNameUnique20260719 struct{}

func (m DropAPITokenNameUnique20260719) ID() string {
	return "20260719_drop_api_token_name_unique"
}

func (m DropAPITokenNameUnique20260719) Description() string {
	return `Drop the global unique constraint on api_tokens.name in a dialect-aware way. Replaces the SQL migration 20260718_scope_api_token_name_uniqueness_to_user which used DROP CONSTRAINT syntax unsupported by SQLite (issue #729).`
}

func (m DropAPITokenNameUnique20260719) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: the uniqueness requirement was removed from the model and is not restored.
	return nil
}

func (m DropAPITokenNameUnique20260719) Up(ctx context.Context, db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "postgres":
		return m.upPostgres(ctx, db)
	case "sqlite":
		return m.upSQLite(ctx, db)
	default:
		return nil
	}
}

func (m DropAPITokenNameUnique20260719) upPostgres(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, stmt := range []string{
			// Constraint name used by GORM >= 1.25.10
			`ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS uni_api_tokens_name`,
			// Auto-generated constraint name from older GORM versions (inline UNIQUE)
			`ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS api_tokens_name_key`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (m DropAPITokenNameUnique20260719) upSQLite(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	if !db.Migrator().HasTable("api_tokens") {
		return nil
	}

	// Depending on which GORM version created the table, the uniqueness on name
	// may exist as an inline or table-level UNIQUE constraint, or not at all
	// (fresh databases created after the model change).
	type indexListRow struct {
		Name   string `gorm:"column:name"`
		Unique int    `gorm:"column:unique"`
		Origin string `gorm:"column:origin"`
	}
	var indexes []indexListRow
	if err := db.WithContext(ctx).Raw(`PRAGMA index_list('api_tokens')`).Scan(&indexes).Error; err != nil {
		return err
	}

	needsRebuild := false
	for _, idx := range indexes {
		if idx.Unique != 1 || idx.Origin == "pk" {
			continue
		}
		type indexInfoRow struct {
			Name string `gorm:"column:name"`
		}
		var cols []indexInfoRow
		if err := db.WithContext(ctx).Raw(fmt.Sprintf(`PRAGMA index_info('%s')`, idx.Name)).Scan(&cols).Error; err != nil {
			return err
		}
		if len(cols) == 1 && cols[0].Name == "name" {
			needsRebuild = true
			break
		}
	}

	if !needsRebuild {
		log.Info("api_tokens.name has no unique constraint or index; nothing to do")
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A UNIQUE constraint cannot be dropped in SQLite; rebuild the table
		// with the schema matching the current APIToken model.
		log.Info("Rebuilding api_tokens to remove UNIQUE constraint on name")
		for _, stmt := range []string{
			`CREATE TABLE api_tokens__rebuild (
				"id" integer PRIMARY KEY AUTOINCREMENT,
				"name" text,
				"user_id" integer,
				"token" text,
				"created_at" datetime
			)`,
			`INSERT INTO api_tokens__rebuild ("id","name","user_id","token","created_at")
				SELECT "id","name","user_id","token","created_at" FROM api_tokens`,
			`DROP TABLE api_tokens`,
			`ALTER TABLE api_tokens__rebuild RENAME TO api_tokens`,
			`CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_api_tokens_token ON api_tokens(token)`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func init() {
	Register(DropAPITokenNameUnique20260719{})
}
