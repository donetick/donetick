package chore

import (
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	uModel "donetick.com/core/internal/user/model"
)

// TestAPIDeleteChorePermission covers the authorization rule used by
// (*API).DeleteChore for the eapi/v1/chore endpoint: the chore creator may
// always delete, and circle admins/managers may delete chores created by
// other members.
func TestAPIDeleteChorePermission(t *testing.T) {
	const (
		creatorID = 1
		otherID   = 2
	)

	circleUsers := func(role cModel.UserRole) []*cModel.UserCircleDetail {
		return []*cModel.UserCircleDetail{
			{UserCircle: cModel.UserCircle{UserID: creatorID, Role: cModel.UserRoleAdmin}},
			{UserCircle: cModel.UserCircle{UserID: otherID, Role: role}},
		}
	}

	tests := []struct {
		name        string
		userID      int
		role        cModel.UserRole
		wantAllowed bool
	}{
		{name: "creator can delete own chore", userID: creatorID, role: cModel.UserRoleAdmin, wantAllowed: true},
		{name: "admin can delete another member's chore", userID: otherID, role: cModel.UserRoleAdmin, wantAllowed: true},
		{name: "manager can delete another member's chore", userID: otherID, role: cModel.UserRoleManager, wantAllowed: true},
		{name: "member cannot delete another member's chore", userID: otherID, role: cModel.UserRoleMember, wantAllowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := circleUsers(test.role)
			user := uModel.UserDetails{User: uModel.User{ID: test.userID}}

			// Mirrors the handler: creator short-circuits, otherwise the
			// admin/manager role decides.
			allowed := creatorID == test.userID || user.IsAdminOrManager(users)

			if allowed != test.wantAllowed {
				t.Fatalf("allowed = %v, want %v", allowed, test.wantAllowed)
			}
		})
	}
}
