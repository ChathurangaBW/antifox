package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateAssetsRejectsMissingTask: POST /api/assets with a task_id that names
// no task is refused with 400 and writes nothing.
func TestCreateAssetsRejectsMissingTask(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	s := &Server{m: m}
	t.Cleanup(func() { _, _ = m.pg.Exec(`DELETE FROM assets WHERE ip IN ('10.254.9.1','10.254.9.2')`) })
	post := func(body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		s.insertAssets(rec, httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(b)))
		return rec
	}
	count := func(ip string) int {
		var n int
		_ = m.pg.QueryRow(`SELECT count(*) FROM assets WHERE type='ip' AND ip=$1`, ip).Scan(&n)
		return n
	}
	for _, bad := range []int64{77777777, -1} {
		rec := post(map[string]any{"task_id": bad, "assets": []any{map[string]any{"type": "ip", "ip": "10.254.9.1"}}})
		if rec.Code != 400 {
			t.Fatalf("task_id %d: got %d %s, want 400", bad, rec.Code, rec.Body.String())
		}
		if count("10.254.9.1") != 0 {
			t.Fatalf("task_id %d: rejected request wrote an asset", bad)
		}
	}
	if rec := post(map[string]any{"assets": []any{map[string]any{"type": "ip", "ip": "10.254.9.2"}}}); rec.Code != 200 || count("10.254.9.2") != 1 {
		t.Fatalf("no task_id: got %d %s", rec.Code, rec.Body.String())
	}
}
