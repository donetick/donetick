package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabelFilterProjectScopes(t *testing.T) {
	cases := []struct{ method, route, scope string }{
		{"GET", "/api/v1/labels", "labels:read"},
		{"POST", "/api/v1/labels", "labels:write"},
		{"PUT", "/api/v1/labels", "labels:write"},
		{"DELETE", "/api/v1/labels/:id", "labels:delete"},
		{"GET", "/api/v1/filters", "filters:read"},
		{"GET", "/api/v1/filters/pinned", "filters:read"},
		{"GET", "/api/v1/filters/by-usage", "filters:read"},
		{"GET", "/api/v1/filters/:id", "filters:read"},
		{"POST", "/api/v1/filters", "filters:write"},
		{"PUT", "/api/v1/filters/:id", "filters:write"},
		{"POST", "/api/v1/filters/:id/toggle-pin", "filters:write"},
		{"DELETE", "/api/v1/filters/:id", "filters:delete"},
		{"GET", "/api/v1/projects", "projects:read"},
		{"GET", "/api/v1/projects/:id/sections", "projects:read"},
		{"POST", "/api/v1/projects/:id/access-preview", "projects:read"},
		{"POST", "/api/v1/projects", "projects:write"},
		{"PUT", "/api/v1/projects/:id", "projects:write"},
		{"POST", "/api/v1/projects/:id/sections", "projects:write"},
		{"PUT", "/api/v1/projects/:id/sections/:sectionId", "projects:write"},
		{"PUT", "/api/v1/projects/:id/sections/:sectionId/move", "projects:write"},
		{"PUT", "/api/v1/projects/:id/board/chores/:choreId/move", "projects:write"},
		{"DELETE", "/api/v1/projects/:id", "projects:delete"},
		{"DELETE", "/api/v1/projects/:id/sections/:sectionId", "projects:delete"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			require.True(t, ValidAPITokenScopes([]string{tc.scope}))
			required := requiredTokenScope(tc.method, tc.route)
			require.Equal(t, tc.scope, required)
			for _, scope := range APITokenScopes() {
				require.Equal(t, scope == tc.scope, tokenHasScope([]string{scope}, required), "scope %s", scope)
			}
		})
	}
	require.Empty(t, requiredTokenScope(http.MethodPost, "/api/v1/projects/:id/new-action"))
	require.Empty(t, requiredTokenScope(http.MethodGet, "/api/v1/filters/:id/new-action"))
}
