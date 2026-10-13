package repo

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
)

// Leaving or being removed from a circle drops the user's project shares there, so
// rejoining later does not quietly restore access to projects shared before.
func TestLeavingACircleDropsProjectShares(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "c.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&cModel.UserCircle{}, &pModel.ProjectMember{}))
	r := &CircleRepository{db: db}

	for _, id := range []int{2, 3} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: 1, IsActive: true, Role: "member"}).Error)
		require.NoError(t, db.Create(&pModel.ProjectMember{ProjectID: 10, UserID: id, CircleID: 1, AddedBy: 1}).Error)
	}
	require.NoError(t, db.Create(&pModel.ProjectMember{ProjectID: 11, UserID: 2, CircleID: 5, AddedBy: 1}).Error)

	require.NoError(t, r.LeaveCircleByUserID(t.Context(), 1, 2))
	require.NoError(t, r.DeleteMemberByID(t.Context(), 1, 3))

	var left []pModel.ProjectMember
	require.NoError(t, db.Find(&left).Error)
	require.Len(t, left, 1)
	require.Equal(t, 5, left[0].CircleID, "shares in other circles are untouched")
}
