package auth

import (
	"net/http"

	uModel "donetick.com/core/internal/user/model"
	"github.com/gin-gonic/gin"
)

const apiTokenContextKey = "auth.api_token"

// APITokenScopes is the explicit, independent scope catalog. No scope implies another.
func APITokenScopes() []string {
	return []string{"chores:read", "chores:write", "chores:execute", "chores:delete", "chores:history:write", "chores:approve", "circle:read", "profile:read", "labels:read", "labels:write", "labels:delete", "filters:read", "filters:write", "filters:delete", "projects:read", "projects:write", "projects:delete", "things:read", "things:write", "rewards:read", "rewards:write", "rewards:delete", "rewards:redeem", "rewards:approve"}
}

// CurrentAPIToken returns the API token used to authenticate the current request, if any.
// Requests authenticated via session (JWT) have no associated token.
func CurrentAPIToken(c *gin.Context) (*uModel.APIToken, bool) {
	data, ok := c.Get(apiTokenContextKey)
	if !ok {
		return nil, false
	}
	token, ok := data.(*uModel.APIToken)
	return token, ok
}

func ValidAPITokenScopes(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	known := map[string]bool{}
	for _, scope := range APITokenScopes() {
		known[scope] = true
	}
	for _, scope := range scopes {
		if !known[scope] {
			return false
		}
		known[scope] = false
	}
	return true
}

// Explicit route allowlist: newly added routes remain session-only until reviewed.
func requiredTokenScope(method, route string) string {
	if method == http.MethodGet {
		switch route {
		case "/api/v1/labels":
			return "labels:read"
		case "/api/v1/filters", "/api/v1/filters/pinned", "/api/v1/filters/by-usage", "/api/v1/filters/:id":
			return "filters:read"
		case "/api/v1/projects", "/api/v1/projects/:id/sections":
			return "projects:read"
		case "/api/v1/users/profile":
			return "profile:read"
		case "/eapi/v1/circle/members", "/api/v1/users/", "/api/v1/circles/members", "/api/v1/circles/":
			return "circle:read"
		case "/eapi/v1/chore", "/api/v1/chores/", "/api/v1/chores/archived", "/api/v1/chores/history", "/api/v1/chores/:id", "/api/v1/chores/:id/details", "/api/v1/chores/:id/history", "/api/v1/chores/:id/timer":
			return "chores:read"
		case "/eapi/v1/things/:id/state/change", "/eapi/v1/things/:id/state":
			// These GET endpoints mutate the thing's state, so they require write access.
			return "things:write"
		case "/eapi/v1/things/:id", "/eapi/v1/things/":
			return "things:read"
		case "/api/v1/things", "/api/v1/things/:id/history":
			return "things:read"
		case "/api/v1/rewards", "/api/v1/rewards/redemptions":
			return "rewards:read"
		}
	}
	switch method + " " + route {
	case "POST /api/v1/labels", "PUT /api/v1/labels":
		return "labels:write"
	case "DELETE /api/v1/labels/:id":
		return "labels:delete"
	case "POST /api/v1/filters", "PUT /api/v1/filters/:id", "POST /api/v1/filters/:id/toggle-pin":
		return "filters:write"
	case "DELETE /api/v1/filters/:id":
		return "filters:delete"
	case "POST /api/v1/projects/:id/access-preview":
		return "projects:read"
	case "POST /api/v1/things", "PUT /api/v1/things", "PUT /api/v1/things/:id/state":
		return "things:write"
	case "DELETE /api/v1/things/:id":
		return "things:write"
	case "POST /api/v1/rewards", "PUT /api/v1/rewards/:id":
		return "rewards:write"
	case "DELETE /api/v1/rewards/:id":
		return "rewards:delete"
	case "POST /api/v1/rewards/:id/redeem":
		return "rewards:redeem"
	case "POST /api/v1/rewards/redemptions/:id/approve", "POST /api/v1/rewards/redemptions/:id/reject":
		return "rewards:approve"
	case "POST /api/v1/projects", "PUT /api/v1/projects/:id", "POST /api/v1/projects/:id/sections", "PUT /api/v1/projects/:id/sections/:sectionId", "PUT /api/v1/projects/:id/sections/:sectionId/move", "PUT /api/v1/projects/:id/board/chores/:choreId/move":
		return "projects:write"
	case "DELETE /api/v1/projects/:id", "DELETE /api/v1/projects/:id/sections/:sectionId":
		return "projects:delete"
	case "POST /eapi/v1/chore", "PUT /eapi/v1/chore/:id", "POST /api/v1/chores/", "POST /api/v1/chores/bulk", "PUT /api/v1/chores/", "PUT /api/v1/chores/:id/priority", "PUT /api/v1/chores/:id/assignee", "PUT /api/v1/chores/:id/dueDate", "PUT /api/v1/chores/:id/archive", "PUT /api/v1/chores/:id/unarchive":
		return "chores:write"
	case "POST /eapi/v1/chore/:id/complete", "POST /api/v1/chores/:id/do", "POST /api/v1/chores/:id/skip", "PUT /api/v1/chores/:id/start", "PUT /api/v1/chores/:id/pause", "PUT /api/v1/chores/:id/timer/reset", "PUT /api/v1/chores/:id/subtask":
		return "chores:execute"
	case "DELETE /eapi/v1/chore/:id", "DELETE /api/v1/chores/:id":
		return "chores:delete"
	case "PUT /api/v1/chores/:id/history/:history_id", "DELETE /api/v1/chores/:id/history/:history_id", "POST /api/v1/chores/:id/undo", "PUT /api/v1/chores/:id/timer/:session_id", "DELETE /api/v1/chores/:id/timer/:session_id":
		return "chores:history:write"
	case "POST /api/v1/chores/:id/approve", "POST /api/v1/chores/:id/reject":
		return "chores:approve"
	}
	return ""
}

func tokenHasScope(scopes []string, required string) bool {
	if required == "" {
		return false
	}
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

// TokenHasScope reports whether scopes contains the given scope. Unlike
// tokenHasScope, an empty required scope is treated as always satisfied,
// since callers use this to check for optional, additive scopes rather than
// the route-gating scope.
func TokenHasScope(scopes []string, required string) bool {
	if required == "" {
		return true
	}
	return tokenHasScope(scopes, required)
}

// Scope policies apply only to API tokens; session authorization remains unchanged.
func denyTokenScope(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API token has insufficient scope or this route requires a session", "code": "insufficient_scope"})
}
