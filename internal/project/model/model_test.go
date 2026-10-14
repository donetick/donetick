package model

import "testing"

func TestProjectCanView(t *testing.T) {
	const owner = 1
	const otherMember = 2

	tests := []struct {
		name    string
		project Project
		userID  int
		canView bool
	}{
		{
			name:    "public project is visible to any circle member",
			project: Project{CreatedBy: owner, IsPrivate: false},
			userID:  otherMember,
			canView: true,
		},
		{
			name:    "private project is visible to its creator",
			project: Project{CreatedBy: owner, IsPrivate: true},
			userID:  owner,
			canView: true,
		},
		{
			name:    "private project is hidden from other circle members",
			project: Project{CreatedBy: owner, IsPrivate: true},
			userID:  otherMember,
			canView: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.project.CanView(tt.userID); got != tt.canView {
				t.Errorf("CanView(%d) = %v, want %v", tt.userID, got, tt.canView)
			}
		})
	}
}
