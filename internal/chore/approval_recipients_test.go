package chore

import (
	"testing"

	"github.com/stretchr/testify/require"

	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/project/model"
	uModel "donetick.com/core/internal/user/model"
)

// A completion waiting for approval must not announce the task's name to admins
// and managers who cannot see it, such as those outside a restricted project.
func TestPendingApprovalRecipientsStayInsideTheAudience(t *testing.T) {
	const owner, colleague, outsideAdmin, insideManager = 1, 2, 3, 4
	member := func(id int, role cModel.UserRole) *cModel.UserCircleDetail {
		return &cModel.UserCircleDetail{UserCircle: cModel.UserCircle{UserID: id, CircleID: 1, Role: role, IsActive: true}}
	}
	circleUsers := []*cModel.UserCircleDetail{
		member(owner, cModel.UserRoleMember), member(colleague, cModel.UserRoleMember),
		member(outsideAdmin, cModel.UserRoleAdmin), member(insideManager, cModel.UserRoleManager),
	}
	submitter := &uModel.User{ID: colleague}

	project := &pModel.Project{ID: 7, CircleID: 1, CreatedBy: owner, IsPrivate: true, SharedWithMembers: true}
	project.SetMembers([]int{colleague, insideManager})
	inProject := &chModel.Chore{ID: 1, CircleID: 1, CreatedBy: owner, ProjectID: &project.ID, Project: project, IsPrivate: true,
		AssignedTo: intPtr(colleague), Assignees: []chModel.ChoreAssignees{{UserID: colleague}}}
	require.Equal(t, []int{insideManager}, pendingApprovalRecipients(inProject, submitter, circleUsers))

	project.SetMembers([]int{colleague})
	require.Empty(t, pendingApprovalRecipients(inProject, submitter, circleUsers), "nobody outside the project hears about it")

	public := &chModel.Chore{ID: 2, CircleID: 1, CreatedBy: owner,
		AssignedTo: intPtr(colleague), Assignees: []chModel.ChoreAssignees{{UserID: colleague}}}
	require.ElementsMatch(t, []int{outsideAdmin, insideManager}, pendingApprovalRecipients(public, submitter, circleUsers),
		"chores everybody can see keep the fallback to every admin and manager")
}
