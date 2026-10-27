package reward

import (
	"testing"

	"github.com/stretchr/testify/require"

	cModel "donetick.com/core/internal/circle/model"
)

// Everybody who can resolve a redemption is told about it, and nobody else --
// in particular not the admin who requested it themselves.
func TestRedemptionApproversAreTheOtherAdminsAndManagers(t *testing.T) {
	const requester, admin, manager, member, inactiveAdmin = 1, 2, 3, 4, 5
	circleUser := func(id int, role cModel.UserRole, isActive bool) *cModel.UserCircleDetail {
		return &cModel.UserCircleDetail{UserCircle: cModel.UserCircle{UserID: id, CircleID: 1, Role: role, IsActive: isActive}}
	}
	circleUsers := []*cModel.UserCircleDetail{
		circleUser(requester, cModel.UserRoleAdmin, true),
		circleUser(admin, cModel.UserRoleAdmin, true),
		circleUser(manager, cModel.UserRoleManager, true),
		circleUser(member, cModel.UserRoleMember, true),
		circleUser(inactiveAdmin, cModel.UserRoleAdmin, false),
	}

	ids := make([]int, 0)
	for _, approver := range redemptionApprovers(circleUsers, requester) {
		ids = append(ids, approver.UserID)
	}
	require.ElementsMatch(t, []int{admin, manager}, ids)

	// A member's request still reaches every admin and manager.
	ids = ids[:0]
	for _, approver := range redemptionApprovers(circleUsers, member) {
		ids = append(ids, approver.UserID)
	}
	require.ElementsMatch(t, []int{requester, admin, manager}, ids)
}
