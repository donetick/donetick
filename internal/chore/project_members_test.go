package chore

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
)

const membersTestColleagueID = 3

func createSharedTestProject(t *testing.T, h *Handler, db *gorm.DB, members ...int) *pModel.Project {
	t.Helper()
	for _, id := range []int{inheritTestOwnerID, inheritTestOtherID, membersTestColleagueID} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: inheritTestCircleID, IsActive: true}).Error)
	}
	project := &pModel.Project{
		Name: "work", CircleID: inheritTestCircleID, CreatedBy: inheritTestOwnerID,
		IsPrivate: true, SharedWithMembers: true, MemberIDs: members,
	}
	require.NoError(t, h.pjRepo.CreateProject(t.Context(), project))
	return project
}

func TestSharedProjectAcceptsOnlyItsAudienceAsAssignees(t *testing.T) {
	h, db := newInheritPrivacyTestHandler(t)
	project := createSharedTestProject(t, h, db, membersTestColleagueID)

	t.Run("a member can be assigned and the chore turns private", func(t *testing.T) {
		isPrivate := false
		req := &ChoreReq{ID: 1, ProjectID: &project.ID, IsPrivate: &isPrivate,
			Assignees: []chModel.ChoreAssignees{{UserID: inheritTestOwnerID}, {UserID: membersTestColleagueID}}}
		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID))
		require.True(t, *req.IsPrivate)
		require.Len(t, req.Assignees, 2, "members are kept, not narrowed to the owner")
	})

	t.Run("somebody outside the project is rejected", func(t *testing.T) {
		req := &ChoreReq{ID: 1, ProjectID: &project.ID, Assignees: []chModel.ChoreAssignees{{UserID: inheritTestOtherID}}}
		err := h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID)
		require.ErrorIs(t, err, chRepo.ErrAssigneeOutsideProject)

		outsider := inheritTestOtherID
		req = &ChoreReq{ID: 1, ProjectID: &project.ID, AssignedTo: &outsider}
		require.ErrorIs(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID), chRepo.ErrAssigneeOutsideProject)
	})

	t.Run("anyone stays anyone", func(t *testing.T) {
		req := &ChoreReq{ID: 1, ProjectID: &project.ID, AssignStrategy: chModel.AssignmentStrategyRandom}
		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, inheritTestOwnerID, inheritTestCircleID))
		require.Empty(t, req.Assignees)
	})

	t.Run("a member may create tasks in the shared project, an outsider may not", func(t *testing.T) {
		req := &ChoreReq{ID: 1, ProjectID: &project.ID}
		require.NoError(t, h.inheritProjectPrivacy(&gin.Context{}, req, membersTestColleagueID, inheritTestCircleID))
		err := h.inheritProjectPrivacy(&gin.Context{}, &ChoreReq{ID: 1, ProjectID: &project.ID}, inheritTestOtherID, inheritTestCircleID)
		require.EqualError(t, err, "project not found")
	})

	t.Run("the repository enforces the same rule under its lock", func(t *testing.T) {
		chore := &chModel.Chore{Name: "x", CircleID: inheritTestCircleID, CreatedBy: inheritTestOwnerID, ProjectID: &project.ID, AssignStrategy: chModel.AssignmentStrategyRandom}
		_, err := h.choreRepo.CreateChoreWithAssignees(t.Context(), chore, []int{inheritTestOtherID}, inheritTestOwnerID)
		require.ErrorIs(t, err, chRepo.ErrAssigneeOutsideProject)
		_, err = h.choreRepo.CreateChoreWithAssignees(t.Context(), &chModel.Chore{Name: "y", CircleID: inheritTestCircleID, CreatedBy: inheritTestOtherID, ProjectID: &project.ID}, nil, inheritTestOtherID)
		require.EqualError(t, err, "project not found")
	})
}

func TestAnyoneRotationStaysInsideSharedProject(t *testing.T) {
	project := &pModel.Project{ID: 9, CreatedBy: 1, IsPrivate: true, SharedWithMembers: true}
	project.SetMembers([]int{3})
	chore := &chModel.Chore{
		ProjectID:      &project.ID,
		Project:        project,
		AssignStrategy: chModel.AssignmentStrategyRoundRobin,
		AssignedTo:     intPtr(1),
	}
	for i := 0; i < 6; i++ {
		next, err := checkNextAssignee(chore, nil, 1, circleOf(1, 2, 3))
		require.NoError(t, err)
		require.NotNil(t, next)
		require.NotEqual(t, 2, *next, "user 2 cannot see the project and must never get a turn")
		chore.AssignedTo = next
	}
}

func TestCanCompleteInSharedProject(t *testing.T) {
	project := &pModel.Project{ID: 9, CreatedBy: 1, IsPrivate: true, SharedWithMembers: true}
	project.SetMembers([]int{3})
	unassigned := &chModel.Chore{ProjectID: &project.ID, Project: project, IsPrivate: true, CreatedBy: 1, AssignStrategy: chModel.AssignmentStrategyNoAssignee}
	require.True(t, unassigned.CanComplete(3, nil), "a member can pick up an unassigned task")
	require.False(t, unassigned.CanComplete(2, nil), "an outsider can't")

	assigned := &chModel.Chore{ProjectID: &project.ID, Project: project, IsPrivate: true, CreatedBy: 1,
		AssignStrategy: chModel.AssignmentStrategyRandom, AssignedTo: intPtr(3), Assignees: []chModel.ChoreAssignees{{UserID: 3}}}
	require.True(t, assigned.CanComplete(3, nil))
	admin := []*cModel.UserCircleDetail{{UserCircle: cModel.UserCircle{UserID: 2, Role: "admin"}}}
	require.False(t, assigned.CanComplete(2, admin), "circle roles do not bypass a restricted project")
}
