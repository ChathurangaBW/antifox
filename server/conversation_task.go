package server

import (
	"net/http"
	"strconv"
	"strings"

	"antifox/agent"
	"antifox/db"
)

// conversationTaskBinding resolves a conversation's attached task into the
// binding its chat agent runs with, or nil when it is not attached or the task
// no longer exists (deleted tasks fall back to a plain conversation).
func (s *Server) conversationTaskBinding(c *db.Conversation) *agent.TaskBinding {
	if c == nil || c.TaskID == nil || *c.TaskID <= 0 {
		return nil
	}
	t := s.m.ResolveTask(strconv.FormatInt(*c.TaskID, 10))
	if t == nil || t.Store == nil {
		return nil
	}
	return &agent.TaskBinding{
		TaskID:          *c.TaskID,
		Name:            t.Name,
		Goal:            t.Goal,
		Store:           t.Store,
		Assets:          s.m.Assets(),
		TrafficResolver: s.resolveFindingTraffic,
		Verifier:        s.recordFindingVerification,
		Notify:          t.Notify,
	}
}

// parseTaskRef reads an optional task reference sent as a JSON number, a numeric
// string, or null. ok=false means a value was sent but is not a task id.
func parseTaskRef(raw any) (id *int64, ok bool) {
	switch v := raw.(type) {
	case nil:
		return nil, true
	case float64:
		n := int64(v)
		if float64(n) != v || n <= 0 {
			return nil, false
		}
		return &n, true
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, true
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, false
		}
		return &n, true
	}
	return nil, false
}

// PATCH /api/conversations/{id}/task — body {task_id: <id>|null}. Attaches the
// conversation to a task, or detaches it with null.
func (s *Server) pgSetConversationTask(w http.ResponseWriter, r *http.Request) {
	pg, c, ok := s.convByID(w, r)
	if !ok {
		return
	}
	var req struct {
		TaskID any `json:"task_id"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	tid, ok := parseTaskRef(req.TaskID)
	if !ok {
		writeErr(w, 400, "task_id must be a task id or null")
		return
	}
	if tid != nil && s.m.ResolveTask(strconv.FormatInt(*tid, 10)) == nil {
		writeErr(w, 404, "task not found")
		return
	}
	if err := pg.SetConversationTask(c.ID, tid); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "task_id": tid})
}
