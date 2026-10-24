package database

import (
	ch "donetick.com/core/internal/chore/model"
	circle "donetick.com/core/internal/circle/model"
	user "donetick.com/core/internal/user/model"
	xp "donetick.com/core/internal/xp/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestXPMigrationPreservesLegacyData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	require.NoError(t, err)
	// Build the base models without the newly added nullable/defaulted columns.
	require.NoError(t, db.AutoMigrate(&circle.UserCircle{}, &ch.Chore{}, &user.User{}))
	require.NoError(t, db.Migrator().DropColumn(&ch.Chore{}, "xp_category_id"))
	require.NoError(t, db.Migrator().DropColumn(&user.User{}, "xp_mode"))
	require.NoError(t, db.Exec("INSERT INTO users (id, username) VALUES (1, 'legacy')").Error)
	require.NoError(t, db.Exec("INSERT INTO chores (id, circle_id, points) VALUES (1, 1, 25)").Error)
	require.NoError(t, db.Create(&circle.UserCircle{CircleID: 1, UserID: 1, IsActive: true, Points: 100, PointsRedeemed: 40}).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, Migration(db))
	}
	var member circle.UserCircle
	require.NoError(t, db.First(&member).Error)
	require.Equal(t, 100, member.Points)
	require.Equal(t, 40, member.PointsRedeemed)
	var task ch.Chore
	require.NoError(t, db.First(&task).Error)
	require.Nil(t, task.XPCategoryID)
	require.Equal(t, 25, *task.Points)
	var person user.User
	require.NoError(t, db.First(&person).Error)
	require.False(t, person.XPMode)
	var n int64
	require.NoError(t, db.Model(&xp.Award{}).Count(&n).Error)
	require.Zero(t, n, "must not invent historic categorization")
}
