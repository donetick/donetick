package database

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestItemSharingMigration(t *testing.T) {
	for _, legacyPrivacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("privacy_column_%v", legacyPrivacy), func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sharing.db")), &gorm.Config{})
			require.NoError(t, err)
			for _, table := range []string{"labels", "filters"} {
				columns := "id integer PRIMARY KEY"
				if legacyPrivacy {
					columns += ", is_private numeric DEFAULT false"
				}
				require.NoError(t, db.Exec("CREATE TABLE "+table+" ("+columns+")").Error)
				if legacyPrivacy {
					require.NoError(t, db.Exec("INSERT INTO "+table+" (id, is_private) VALUES (1, true), (2, false)").Error)
				} else {
					require.NoError(t, db.Exec("INSERT INTO "+table+" (id) VALUES (1), (2)").Error)
				}
			}
			require.NoError(t, migrateItemSharing(db))
			for _, table := range []string{"labels", "filters"} {
				var rows []struct {
					ID       int
					IsShared bool
				}
				require.NoError(t, db.Table(table).Order("id").Find(&rows).Error)
				require.Len(t, rows, 2)
				require.Equal(t, !legacyPrivacy, rows[0].IsShared)
				require.True(t, rows[1].IsShared)
				// Defaults for newly inserted records stay private.
				require.NoError(t, db.Exec("INSERT INTO "+table+" (id) VALUES (3)").Error)
				var shared bool
				require.NoError(t, db.Table(table).Select("is_shared").Where("id = 3").Scan(&shared).Error)
				require.False(t, shared)
				// Re-running must not re-share an item that the owner made private.
				require.NoError(t, db.Table(table).Where("id = 2").Update("is_shared", false).Error)
			}
			require.NoError(t, migrateItemSharing(db))
			for _, table := range []string{"labels", "filters"} {
				var shared bool
				require.NoError(t, db.Table(table).Select("is_shared").Where("id = 2").Scan(&shared).Error)
				require.False(t, shared)
			}
		})
	}
}
