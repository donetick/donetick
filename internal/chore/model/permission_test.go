package model

import (
	"testing"

	cModel "donetick.com/core/internal/circle/model"
)

func TestCanDelete(t *testing.T) {
	chore := &Chore{CreatedBy: 1}

	tests := []struct {
		name        string
		userID      int
		circleUsers []*cModel.UserCircleDetail
		want        bool
	}{
		{
			name:   "creator can delete",
			userID: 1,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 1, Role: cModel.UserRoleMember}},
			},
			want: true,
		},
		{
			name:   "admin can delete non-owned chore",
			userID: 2,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 2, Role: cModel.UserRoleAdmin}},
			},
			want: true,
		},
		{
			name:   "manager can delete non-owned chore",
			userID: 3,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 3, Role: cModel.UserRoleManager}},
			},
			want: true,
		},
		{
			name:   "member cannot delete non-owned chore",
			userID: 4,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 4, Role: cModel.UserRoleMember}},
			},
			want: false,
		},
		{
			name:        "user not in circle cannot delete",
			userID:      99,
			circleUsers: []*cModel.UserCircleDetail{},
			want:        false,
		},
		{
			name:   "member with different user ID cannot delete",
			userID: 5,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 6, Role: cModel.UserRoleAdmin}},
				{UserCircle: cModel.UserCircle{UserID: 5, Role: cModel.UserRoleMember}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chore.CanDelete(tt.userID, tt.circleUsers); got != tt.want {
				t.Errorf("CanDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanArchive(t *testing.T) {
	chore := &Chore{CreatedBy: 1}

	tests := []struct {
		name        string
		userID      int
		circleUsers []*cModel.UserCircleDetail
		want        bool
	}{
		{
			name:   "creator can archive",
			userID: 1,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 1, Role: cModel.UserRoleMember}},
			},
			want: true,
		},
		{
			name:   "admin can archive non-owned chore",
			userID: 2,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 2, Role: cModel.UserRoleAdmin}},
			},
			want: true,
		},
		{
			name:   "manager can archive non-owned chore",
			userID: 3,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 3, Role: cModel.UserRoleManager}},
			},
			want: true,
		},
		{
			name:   "member cannot archive non-owned chore",
			userID: 4,
			circleUsers: []*cModel.UserCircleDetail{
				{UserCircle: cModel.UserCircle{UserID: 4, Role: cModel.UserRoleMember}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chore.CanArchive(tt.userID, tt.circleUsers); got != tt.want {
				t.Errorf("CanArchive() = %v, want %v", got, tt.want)
			}
		})
	}
}