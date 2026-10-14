package chore

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
	uModel "donetick.com/core/internal/user/model"
)

// A circle admin or manager may impersonate any member. That must not open a
// project the admin cannot see: acting as the member inside it is refused unless
// the member is the admin's own sub-account.
func TestImpersonationGuardKeepsRestrictedProjectsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, db := newInheritPrivacyTestHandler(t)
	const adminID, memberID, childID = 1, 2, 3
	for _, id := range []int{adminID, memberID, childID} {
		role := cModel.UserRoleMember
		if id == adminID {
			role = cModel.UserRoleAdmin
		}
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: inheritTestCircleID, IsActive: true, Role: role}).Error)
	}
	// The member's private project; the child's private project.
	memberProject := &pModel.Project{Name: "member's", CircleID: inheritTestCircleID, CreatedBy: memberID, IsPrivate: true}
	require.NoError(t, h.pjRepo.CreateProject(t.Context(), memberProject))
	childProject := &pModel.Project{Name: "child's", CircleID: inheritTestCircleID, CreatedBy: childID, IsPrivate: true}
	require.NoError(t, h.pjRepo.CreateProject(t.Context(), childProject))
	memberChore := &chModel.Chore{Name: "secret", CircleID: inheritTestCircleID, CreatedBy: memberID, ProjectID: &memberProject.ID, IsPrivate: true, IsActive: true}
	require.NoError(t, db.Create(memberChore).Error)
	childChore := &chModel.Chore{Name: "homework", CircleID: inheritTestCircleID, CreatedBy: childID, ProjectID: &childProject.ID, IsPrivate: true, IsActive: true}
	require.NoError(t, db.Create(childChore).Error)
	publicChore := &chModel.Chore{Name: "trash", CircleID: inheritTestCircleID, CreatedBy: memberID, IsActive: true}
	require.NoError(t, db.Create(publicChore).Error)

	admin := &uModel.UserDetails{User: uModel.User{ID: adminID, CircleID: inheritTestCircleID}}
	member := &uModel.UserDetails{User: uModel.User{ID: memberID, CircleID: inheritTestCircleID}}
	parent := adminID
	child := &uModel.UserDetails{User: uModel.User{ID: childID, CircleID: inheritTestCircleID, ParentUserID: &parent}}

	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", nil)
		return c
	}
	loaded := func(id int, as int) *chModel.Chore {
		ch, err := h.choreRepo.GetChore(t.Context(), id, as, inheritTestCircleID)
		require.NoError(t, err)
		return ch
	}

	c := newCtx()
	require.False(t, h.impersonationGuard(c, loaded(memberChore.ID, memberID), admin, member, true), "admin acting as member inside the member's private project")
	require.Equal(t, 403, c.Writer.Status())

	require.True(t, h.impersonationGuard(newCtx(), loaded(childChore.ID, childID), admin, child, true), "parent acting for their own sub-account")
	require.True(t, h.impersonationGuard(newCtx(), loaded(publicChore.ID, memberID), admin, member, true), "public chore stays usable")
	require.True(t, h.impersonationGuard(newCtx(), loaded(memberChore.ID, memberID), admin, member, false), "no impersonation, no guard")
}
