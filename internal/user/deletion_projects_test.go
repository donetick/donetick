package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
	uModel "donetick.com/core/internal/user/model"
)

// Deleting an account removes the projects it owned. Chores others created in
// them are kept without a project, so a project shared with selected members
// does not stay behind with nobody able to change it.
func TestDeleteAccountRemovesOwnedProjectsAndKeepsOthersChores(t *testing.T) {
	db := setupTestDeletionDB(t)
	const circleID, ownerID, memberID = 1, 1, 2
	require.NoError(t, db.Create(&uModel.User{ID: ownerID, Username: "owner", DisplayName: "Owner", CircleID: circleID}).Error)
	require.NoError(t, db.Create(&uModel.User{ID: memberID, Username: "member", DisplayName: "Member", CircleID: circleID}).Error)
	require.NoError(t, db.Create(&cModel.Circle{ID: circleID, Name: "c", CreatedBy: memberID, InviteCode: "x"}).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: ownerID, CircleID: circleID, IsActive: true, Role: cModel.UserRoleMember}).Error)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: memberID, CircleID: circleID, IsActive: true, Role: cModel.UserRoleAdmin}).Error)

	shared := &pModel.Project{Name: "shared", CircleID: circleID, CreatedBy: ownerID, IsPrivate: true, SharedWithMembers: true}
	require.NoError(t, db.Create(shared).Error)
	require.NoError(t, db.Create(&pModel.ProjectMember{ProjectID: shared.ID, UserID: memberID, CircleID: circleID, AddedBy: ownerID}).Error)
	membersProject := &pModel.Project{Name: "member's", CircleID: circleID, CreatedBy: memberID, IsPrivate: true, SharedWithMembers: true}
	require.NoError(t, db.Create(membersProject).Error)
	require.NoError(t, db.Create(&pModel.ProjectMember{ProjectID: membersProject.ID, UserID: ownerID, CircleID: circleID, AddedBy: memberID}).Error)
	byMember := &chModel.Chore{Name: "by member", CircleID: circleID, CreatedBy: memberID, ProjectID: &shared.ID, IsPrivate: true, IsActive: true}
	require.NoError(t, db.Create(byMember).Error)
	circleWide := &pModel.Project{Name: "circle-wide", CircleID: circleID, CreatedBy: ownerID}
	require.NoError(t, db.Create(circleWide).Error)
	deletedUser, otherUser := ownerID, memberID
	require.NoError(t, db.Create(&syncModel.Tombstone{CircleID: circleID, EntityType: syncModel.EntityTypeChore, EntityID: 99, UserID: &deletedUser, SyncVersion: 5}).Error)
	require.NoError(t, db.Create(&syncModel.Tombstone{CircleID: circleID, EntityType: syncModel.EntityTypeChore, EntityID: 98, UserID: &otherUser, SyncVersion: 6}).Error)
	require.NoError(t, db.Create(&syncModel.Tombstone{CircleID: circleID, EntityType: syncModel.EntityTypeChore, EntityID: 97, SyncVersion: 7}).Error)

	service := NewDeletionService(db, &mockStorage{})
	result, err := service.DeleteUserAccount(context.Background(), ownerID, nil)
	require.NoError(t, err)
	require.True(t, result.Success, result.Message)

	var projects []pModel.Project
	require.NoError(t, db.Order("id").Find(&projects).Error)
	require.Len(t, projects, 2)
	require.Equal(t, membersProject.ID, projects[0].ID, "the other user's project remains")
	require.Equal(t, circleWide.ID, projects[1].ID, "the deleted user's circle-wide project stays, as upstream keeps it")
	var tombs []syncModel.Tombstone
	require.NoError(t, db.Order("entity_id").Find(&tombs).Error)
	require.Len(t, tombs, 2, "only the deleted user's own revocation tombstone is pruned")
	require.Equal(t, 97, tombs[0].EntityID)
	require.Equal(t, 98, tombs[1].EntityID)
	var members int64
	require.NoError(t, db.Model(&pModel.ProjectMember{}).Count(&members).Error)
	require.Zero(t, members, "the deleted user's own shares and the shares of their projects are gone")
	var kept chModel.Chore
	require.NoError(t, db.First(&kept, byMember.ID).Error)
	require.Nil(t, kept.ProjectID)
}
