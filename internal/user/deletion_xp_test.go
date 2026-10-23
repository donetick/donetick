package user

import (
	"context"
	"testing"

	pModel "donetick.com/core/internal/points"
	uModel "donetick.com/core/internal/user/model"
	"github.com/stretchr/testify/require"
)

func TestDeleteUserAccountRemovesOnlyTheirXPEntries(t *testing.T) {
	db := setupTestDeletionDB(t)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, db.Create(&uModel.User{ID: 1, Username: "deleted", Email: "deleted@example.invalid"}).Error)
	require.NoError(t, db.Create(&[]pModel.XPEntry{
		{UserID: 1, CircleID: 1, MembershipID: 1, EventKey: "deleted-award", Earned: 20},
		{UserID: 2, CircleID: 1, MembershipID: 2, EventKey: "other-award", Earned: 30},
	}).Error)
	service := NewMockDeletionService(db, &mockStorage{})
	result, err := service.DeleteUserAccount(context.Background(), 1, nil)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 1, result.DeletedData["xp_entries"])
	var remaining []pModel.XPEntry
	require.NoError(t, db.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, 2, remaining[0].UserID)
}
