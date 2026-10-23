package database

import (
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	xpRepo "donetick.com/core/internal/points/repo"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestXPMigrationPreservesLegacyBalancesAndIsRepeatable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	// Actual pre-XP tables lack the new mode/category columns and entry tables.
	require.NoError(t, db.Exec(`CREATE TABLE circles (id integer PRIMARY KEY, name text);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE user_circles (id integer PRIMARY KEY AUTOINCREMENT, circle_id integer, user_id integer, is_active numeric, points integer NOT NULL DEFAULT 0, points_redeemed integer NOT NULL DEFAULT 0);`).Error)
	require.NoError(t, db.Exec(`INSERT INTO circles (id, name) VALUES (1, 'Existing circle');`).Error)
	require.NoError(t, db.Exec(`INSERT INTO user_circles (circle_id,user_id,is_active,points,points_redeemed) VALUES (1,1,1,100,25);`).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, Migration(db))
	}
	var circle cModel.Circle
	require.NoError(t, db.First(&circle, 1).Error)
	require.False(t, circle.XPMode)
	stats, err := xpRepo.Stats(db, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 75, stats.TotalXP)
	require.EqualValues(t, 75, stats.Categories["0"])
	var entries int64
	require.NoError(t, db.Model(&pModel.XPEntry{}).Count(&entries).Error)
	require.Zero(t, entries, "migration must not invent historical category awards")
}
