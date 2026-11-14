package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"antifox/agent"
	actool "antifox/engine/tool"
)

func patchJSON(t *testing.T, h http.HandlerFunc, id int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(string(b)))
	req.SetPathValue("id", fmt.Sprint(id))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// TestConversationTaskBinding: a pentest conversation attached to a task can
// record findings into it. Before, report_finding in a conversation always
// failed with "requires a task context" (in all three codebases).
func TestConversationTaskBinding(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	pg := m.pg
	seedPrompts(pg)
	wireTools(pg, buildDomainReg(pg.Assets()))
	t.Cleanup(func() { agent.ToolResolve = nil })
	s := &Server{m: m}
	s.seedOrchestrationTools()

	task, err := m.CreateTask("t-conv-bind", "desc", "find the admin panel", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	taskID := mustInt64(t, task.ID)
	t.Cleanup(func() {
		_, _ = pg.Exec(`DELETE FROM findings WHERE task_id=$1`, taskID)
		_, _ = pg.Exec(`DELETE FROM conversations WHERE task_id=$1 OR title='bind-test'`, taskID)
	})
	conv, err := pg.CreateConversation("pentest", "bind-test", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Attach through the API; reject a task that does not exist.
	if rec := patchJSON(t, s.pgSetConversationTask, conv.ID, map[string]any{"task_id": 999999999}); rec.Code != 404 {
		t.Fatalf("attach to missing task: got %d, want 404", rec.Code)
	}
	if rec := patchJSON(t, s.pgSetConversationTask, conv.ID, map[string]any{"task_id": "abc"}); rec.Code != 400 {
		t.Fatalf("attach with bad id: got %d, want 400", rec.Code)
	}
	if rec := patchJSON(t, s.pgSetConversationTask, conv.ID, map[string]any{"task_id": task.ID}); rec.Code != 200 {
		t.Fatalf("attach: got %d %s", rec.Code, rec.Body.String())
	}
	c, _ := pg.GetConversation(conv.ID)
	bind := s.conversationTaskBinding(c)
	if bind == nil || bind.TaskID != taskID || !strings.Contains(bind.Goal, "admin panel") {
		t.Fatalf("binding = %+v", bind)
	}

	// The resolved pentest tool list must use the task-bound report_finding,
	// not the store-less copy that refuses without a task.
	base := append(bind.Tools("pentest"), actool.DefaultTools()...)
	var rf actool.CoreTool
	for _, tl := range agent.ToolResolve(context.Background(), "pentest", base) {
		if tl.Name() == "report_finding" {
			rf = tl
		}
	}
	if rf == nil {
		t.Fatal("pentest has no report_finding after binding")
	}
	in, _ := json.Marshal(map[string]any{"vulnclass": "exposure", "severity": "medium", "summary": "admin panel reachable", "evidence": "GET /admin 200"})
	res, err := rf.Call(context.Background(), in, &actool.ToolContext{})
	if err != nil || res.IsError || !strings.HasPrefix(res.Flatten(), "finding recorded: ") {
		t.Fatalf("report_finding from an attached conversation failed: %v %s", err, res.Flatten())
	}
	var n int
	_ = pg.QueryRow(`SELECT count(*) FROM findings WHERE task_id=$1 AND worker='pentest'`, taskID).Scan(&n)
	if n != 1 {
		t.Fatalf("finding not saved to the task: %d rows", n)
	}

	// Detach with null: the conversation is plain again.
	if rec := patchJSON(t, s.pgSetConversationTask, conv.ID, map[string]any{"task_id": nil}); rec.Code != 200 {
		t.Fatalf("detach: got %d", rec.Code)
	}
	c, _ = pg.GetConversation(conv.ID)
	if c.TaskID != nil || s.conversationTaskBinding(c) != nil {
		t.Fatal("conversation still attached after detach")
	}
}

func mustInt64(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		t.Fatal(err)
	}
	return n
}
