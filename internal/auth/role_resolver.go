package auth

import (
	cModel "donetick.com/core/internal/circle/model"
)

// ResolveRoleFromGroups determines a user's circle role based on their OIDC group
// membership. Group matching is exact-string and case-sensitive.
//
// Returns (role, true) when group-based role resolution is active (at least one of
// adminGroups or managerGroups is configured). The resolved role is:
//   - RoleAdmin if any user group matches an admin group
//   - RoleManager if any user group matches a manager group
//   - RoleMember otherwise (no match → member)
//
// Returns ("", false) when neither adminGroups nor managerGroups is configured,
// meaning the caller should skip role sync entirely.
func ResolveRoleFromGroups(userGroups, adminGroups, managerGroups []string) (cModel.Role, bool) {
	if len(adminGroups) == 0 && len(managerGroups) == 0 {
		return "", false
	}

	groupSet := make(map[string]struct{}, len(userGroups))
	for _, g := range userGroups {
		groupSet[g] = struct{}{}
	}

	for _, ag := range adminGroups {
		if _, ok := groupSet[ag]; ok {
			return cModel.RoleAdmin, true
		}
	}
	for _, mg := range managerGroups {
		if _, ok := groupSet[mg]; ok {
			return cModel.RoleManager, true
		}
	}

	return cModel.RoleMember, true
}
