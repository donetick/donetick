package auth

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

const apiTokenContextKey = "auth.api_token"

// APITokenScopes is the explicit, independent scope catalog. No scope implies another.
func APITokenScopes() []string {
	return []string{"chores:read", "chores:write", "chores:execute", "chores:delete", "chores:history:write", "chores:approve", "circle:read", "profile:read", "labels:read", "labels:write", "labels:delete", "filters:read", "filters:write", "filters:delete", "projects:read", "projects:write", "projects:delete"}
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

// Scope policies apply only to API tokens; session authorization remains unchanged.
func denyTokenScope(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API token has insufficient scope or this route requires a session", "code": "insufficient_scope"})
}
