package server

import (
	"net/http"
	"strings"

	"antifox/db"
)

// RBAC Permission to enforce interception: route → requires permission maps.
// Safety valve: Unmagnified routers released (not completed) 403,Avoiding leakage to destroy platforms;admin All rights will never be stopped.
// Switches:settings Keys rbac.enforce(Default true).

// isAdmin Report whether the current request is built-in admin Activate. For additional tightening to "capable of executing commands at the host."
// This type of high-risk operationstdio MCP, command/script/shell Custom tool)——These are equivalent to the implementation of the host order.
// Should only admin Use, leave tool:write / mcp:write Delegation of authority operator. If you're not allowed to paint admin
// Processfail-closed).
func isAdmin(r *http.Request) bool {
	acc := CurrentAccess(r)
	if acc == nil {
		return false
	}
	for _, role := range acc.Roles {
		if role.ID == db.RBACRoleAdmin {
			return true
		}
	}
	return false
}

// hostExecToolKind Report whether a custom tool type will execute the command on the host/Script.
func hostExecToolKind(kind string) bool {
	switch kind {
	case "command", "script", "shell":
		return true
	}
	return false
}

// crudPerm Press HTTP Method given read/write/delete Permission.
func crudPerm(method, module string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return module + ":read"
	case http.MethodDelete:
		// Some modules have no ":delete" permission in the catalog (llm/agent/mcp/
		// tool/intercept) — for those, delete is gated by ":write". Without this
		// fallback even admin (whose perm set == the catalog) hits a nonexistent
		// "<module>:delete" and gets 403 on every delete.
		if p := module + ":delete"; db.RBACPermissions[p] != "" {
			return p
		}
		return module + ":write"
	default:
		return module + ":write"
	}
}

// permissionForRequest Permission to return the request; return "" indicates that the map is unmapped.
func permissionForRequest(method, fullPath string) string {
	p := strings.TrimPrefix(fullPath, "/api")
	switch {
	case p == "/rbac/me":
		return "auth:self"
	case strings.HasPrefix(p, "/rbac"):
		if method == http.MethodGet {
			return "rbac:read"
		}
		return "rbac:write"
	case strings.HasPrefix(p, "/audit"):
		return "log:read"
	case strings.HasPrefix(p, "/logs"):
		return "log:read"
	case p == "/stats" || strings.HasPrefix(p, "/tokens"):
		return "dashboard:read"
	case strings.HasPrefix(p, "/tasks"), p == "/active", strings.HasPrefix(p, "/exploration"),
		strings.HasPrefix(p, "/chat"), strings.HasPrefix(p, "/conversations"):
		return crudPerm(method, "task")
	case strings.HasPrefix(p, "/findings"):
		return crudPerm(method, "finding")
	case strings.HasPrefix(p, "/traffic"):
		return "traffic:read"
	case strings.HasPrefix(p, "/assets"), strings.HasPrefix(p, "/companies"), strings.HasPrefix(p, "/sync"):
		return crudPerm(method, "asset")
	case strings.HasPrefix(p, "/llm"):
		return crudPerm(method, "llm")
	case strings.HasPrefix(p, "/agents"), strings.HasPrefix(p, "/triggers"), strings.HasPrefix(p, "/visibility"):
		return crudPerm(method, "agent")
	case strings.HasPrefix(p, "/mcp"):
		return crudPerm(method, "mcp")
	case strings.HasPrefix(p, "/skills"):
		return crudPerm(method, "skill")
	case strings.HasPrefix(p, "/tools"):
		return crudPerm(method, "tool")
	case strings.HasPrefix(p, "/intercept"):
		return crudPerm(method, "intercept")
	case strings.HasPrefix(p, "/knowledge"):
		// search/categories/logs are read actions even though search is POST.
		if p == "/knowledge/search" || p == "/knowledge/categories" || p == "/knowledge/logs" {
			return "knowledge:read"
		}
		return crudPerm(method, "knowledge")
	case strings.HasPrefix(p, "/settings"):
		return crudPerm(method, "config")
	case strings.HasPrefix(p, "/update"):
		// One key update under system configuration: check/Progress = config:read;apply/rollback(POST) = config:write
		// (config:write Only for admin,So high-risk downloads and reloads/♪ Rolling back ♪ admin It can be triggered.
		return crudPerm(method, "config")
	case strings.HasPrefix(p, "/report"):
		// Overwrite /report, /reports, /report/generate, /report-settings,
		// /report-templates, /report-assets (other than /report at the top level).
		// GET=read(View/Download/Status),POST/PUT=write(Generate Report/Manage Template Settings),
		// DELETE=delete(Delete Report/Template). report Module read/write/delete Full.
		return crudPerm(method, "report")
	default:
		return "" // Unmapped → Let go!
	}
}
