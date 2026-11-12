package server

import (
	"net/http"
	"sort"
	"strings"

	"antifox/db"
	"golang.org/x/crypto/bcrypt"
)

// Platform authority management API(/api/rbac/*).

type rbacUserView struct {
	db.RBACUser
	RoleIDs []string `json:"role_ids"`
}

type rbacRoleView struct {
	db.RBACRole
	Permissions []string `json:"permissions"`
}

// GET /api/rbac/me —— Identity of current login user + Role + Permission.
func (s *Server) rbacMe(w http.ResponseWriter, r *http.Request) {
	acc := CurrentAccess(r)
	if acc == nil {
		writeErr(w, 401, "Session invalid")
		return
	}
	perms := make([]string, 0, len(acc.Permissions))
	for k, v := range acc.Permissions {
		if v {
			perms = append(perms, k)
		}
	}
	sort.Strings(perms)
	writeJSON(w, 200, map[string]any{
		"user":        acc.User,
		"roles":       acc.Roles,
		"permissions": perms,
		"scope":       acc.Scope,
	})
}

// GET /api/rbac/metadata —— Permission Directory + Role (management) UI Use it.
func (s *Server) rbacMetadata(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	roles, err := pg.ListRBACRoles()
	if err != nil {
		writeErr(w, 500, "Failed to fetch role")
		return
	}
	writeJSON(w, 200, map[string]any{
		"permissions": db.RBACPermissions,
		"roles":       roles,
		"scopes":      []string{db.RBACScopeAll, db.RBACScopeAssigned, db.RBACScopeOwn},
	})
}

// GET /api/rbac/users
func (s *Server) rbacListUsers(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	users, err := pg.ListRBACUsers()
	if err != nil {
		writeErr(w, 500, "Failed to fetch user")
		return
	}
	out := make([]rbacUserView, 0, len(users))
	for _, u := range users {
		ids, _ := pg.ListRBACUserRoleIDs(u.ID)
		if ids == nil {
			ids = []string{}
		}
		out = append(out, rbacUserView{RBACUser: u, RoleIDs: ids})
	}
	writeJSON(w, 200, map[string]any{"users": out})
}

// POST /api/rbac/users
func (s *Server) rbacCreateUser(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	var req struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Password    string   `json:"password"`
		Enabled     *bool    `json:"enabled"`
		RoleIDs     []string `json:"role_ids"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Invalid Parameter")
		return
	}
	username := strings.TrimSpace(strings.ToLower(req.Username))
	if username == "" || len(req.Password) < 8 {
		writeErr(w, 400, "User name must be filled. Password at least. 8 bit")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "Password processing failed")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	u, err := pg.CreateRBACUser(username, req.DisplayName, string(hash), enabled, req.RoleIDs)
	if err != nil {
		msg := "Failed to create user"
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(err.Error(), "duplicate") {
			msg = "Username already exists"
		}
		writeErr(w, 400, msg)
		return
	}
	pg.RecordAudit(auditActor(r), "rbac", "create_user", "success", clientIP(r), "Create User "+username, map[string]any{"user_id": u.ID})
	writeJSON(w, 200, map[string]any{"user": u})
}

// PUT /api/rbac/users/{id}
func (s *Server) rbacUpdateUser(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeErr(w, 400, "Missing users ID")
		return
	}
	var req struct {
		DisplayName string    `json:"display_name"`
		Enabled     *bool     `json:"enabled"`
		RoleIDs     *[]string `json:"role_ids"`
		Password    string    `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Invalid Parameter")
		return
	}
	if err := pg.UpdateRBACUser(id, req.DisplayName, req.Enabled, req.RoleIDs); err != nil {
		writeErr(w, 500, "Failed to update user")
		return
	}
	if pw := strings.TrimSpace(req.Password); pw != "" {
		if len(pw) < 8 {
			writeErr(w, 400, "Password at least 8 bit")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, 500, "Password processing failed")
			return
		}
		if err := pg.UpdateRBACUserPassword(id, string(hash)); err != nil {
			writeErr(w, 500, "Failed to reset password")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/rbac/users/{id}
func (s *Server) rbacDeleteUser(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if err := pg.DeleteRBACUser(id); err != nil {
		writeErr(w, 400, "Failed to delete: "+err.Error())
		return
	}
	pg.RecordAudit(auditActor(r), "rbac", "delete_user", "success", clientIP(r), "Remove User "+id, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// GET /api/rbac/roles
func (s *Server) rbacListRoles(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	roles, err := pg.ListRBACRoles()
	if err != nil {
		writeErr(w, 500, "Failed to fetch role")
		return
	}
	out := make([]rbacRoleView, 0, len(roles))
	for _, r := range roles {
		keys, _ := pg.ListRBACRolePermissionKeys(r.ID)
		if keys == nil {
			keys = []string{}
		}
		out = append(out, rbacRoleView{RBACRole: r, Permissions: keys})
	}
	writeJSON(w, 200, map[string]any{"roles": out})
}

type roleReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// POST /api/rbac/roles
func (s *Server) rbacCreateRole(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	var req roleReq
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeErr(w, 400, "Role name cannot be empty")
		return
	}
	role, err := pg.UpsertRBACRole("", req.Name, req.Description, req.Scope, req.Permissions)
	if err != nil {
		writeErr(w, 400, "Creating Role Failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"role": role})
}

// PUT /api/rbac/roles/{id}
func (s *Server) rbacUpdateRole(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req roleReq
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeErr(w, 400, "Role name cannot be empty")
		return
	}
	role, err := pg.UpsertRBACRole(id, req.Name, req.Description, req.Scope, req.Permissions)
	if err != nil {
		writeErr(w, 400, "Failed to update role: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"role": role})
}

// DELETE /api/rbac/roles/{id}
func (s *Server) rbacDeleteRole(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if err := pg.DeleteRBACRole(id); err != nil {
		writeErr(w, 400, "Failed to delete character: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
