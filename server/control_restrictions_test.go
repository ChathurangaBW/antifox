package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Only the rejected path is exercised: no terminal rerun or engine loop runs.
func TestRerunRejectsNonterminalWithoutMutation(t *testing.T) {
	for _, status := range []string{"created", "running", "paused", "", "unknown"} {
		t.Run("status_"+status, func(t *testing.T) {
			connector := &telemetryConnector{}
			s, task := telemetryFixture(t, connector)
			task.Status = status
			task.Paused = status == "paused"
			paused := task.Paused
			// Any attempted execution/cancellation would alter these inert handles.
			req := httptest.NewRequest(http.MethodPost, "/api/tasks/5/control", strings.NewReader(`{"action":"rerun"}`))
			req.SetPathValue("id", "5")
			rec := httptest.NewRecorder()
			s.control(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("status=%d, want 409; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "rerun is only available for finished tasks") {
				t.Fatalf("unexpected response: %s", rec.Body.String())
			}
			if task.Status != status || task.Paused != paused || task.FirstRunAt != 100 || task.DeadlineAt != 700 || task.CompletedAt != 0 {
				t.Fatal("rejected rerun modified task lifecycle or clocks")
			}
			if connector.writes.Load() != 0 {
				t.Fatal("rejected rerun attempted a database write")
			}
		})
	}
}
