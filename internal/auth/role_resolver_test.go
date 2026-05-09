package auth

import (
	"testing"

	cModel "donetick.com/core/internal/circle/model"
)

func TestResolveRoleFromGroups(t *testing.T) {
	tests := []struct {
		name          string
		userGroups    []string
		adminGroups   []string
		managerGroups []string
		wantRole      cModel.Role
		wantActive    bool
	}{
		{
			name:          "no config returns inactive",
			userGroups:    []string{"some-group"},
			adminGroups:   nil,
			managerGroups: nil,
			wantRole:      "",
			wantActive:    false,
		},
		{
			name:          "empty config slices returns inactive",
			userGroups:    []string{"some-group"},
			adminGroups:   []string{},
			managerGroups: []string{},
			wantRole:      "",
			wantActive:    false,
		},
		{
			name:          "admin match returns admin",
			userGroups:    []string{"household-admins", "users"},
			adminGroups:   []string{"household-admins"},
			managerGroups: []string{"household-managers"},
			wantRole:      cModel.RoleAdmin,
			wantActive:    true,
		},
		{
			name:          "manager match returns manager",
			userGroups:    []string{"household-managers", "users"},
			adminGroups:   []string{"household-admins"},
			managerGroups: []string{"household-managers"},
			wantRole:      cModel.RoleManager,
			wantActive:    true,
		},
		{
			name:          "admin wins over manager when user in both",
			userGroups:    []string{"household-admins", "household-managers"},
			adminGroups:   []string{"household-admins"},
			managerGroups: []string{"household-managers"},
			wantRole:      cModel.RoleAdmin,
			wantActive:    true,
		},
		{
			name:          "no match returns member when config present",
			userGroups:    []string{"unrelated-group"},
			adminGroups:   []string{"household-admins"},
			managerGroups: []string{"household-managers"},
			wantRole:      cModel.RoleMember,
			wantActive:    true,
		},
		{
			name:          "empty user groups returns member when config present",
			userGroups:    nil,
			adminGroups:   []string{"household-admins"},
			managerGroups: []string{},
			wantRole:      cModel.RoleMember,
			wantActive:    true,
		},
		{
			name:          "case sensitive matching",
			userGroups:    []string{"Household-Admins"},
			adminGroups:   []string{"household-admins"},
			managerGroups: nil,
			wantRole:      cModel.RoleMember,
			wantActive:    true,
		},
		{
			name:          "only admin groups configured no match",
			userGroups:    []string{"users"},
			adminGroups:   []string{"admins"},
			managerGroups: nil,
			wantRole:      cModel.RoleMember,
			wantActive:    true,
		},
		{
			name:          "only manager groups configured with match",
			userGroups:    []string{"managers"},
			adminGroups:   nil,
			managerGroups: []string{"managers"},
			wantRole:      cModel.RoleManager,
			wantActive:    true,
		},
		{
			name:          "multiple admin groups second matches",
			userGroups:    []string{"super-admins"},
			adminGroups:   []string{"admins", "super-admins"},
			managerGroups: nil,
			wantRole:      cModel.RoleAdmin,
			wantActive:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, active := ResolveRoleFromGroups(tt.userGroups, tt.adminGroups, tt.managerGroups)
			if active != tt.wantActive {
				t.Errorf("active = %v, want %v", active, tt.wantActive)
			}
			if role != tt.wantRole {
				t.Errorf("role = %q, want %q", role, tt.wantRole)
			}
		})
	}
}
