package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Real JWT/RBAC middleware and handlers; no application constructors or engine loops.
func TestAuthenticatedObservations(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, failure, token string
		want                               int
	}{
		{"stats", "GET", "/api/stats?task=5", "", "valid", 200},
		{"tasks", "GET", "/api/tasks", "", "valid", 200},
		{"stats_failure", "GET", "/api/stats?task=5", "assets/query", "valid", 503},
		{"tasks_failure", "GET", "/api/tasks", "tokens/query", "valid", 503},
		{"rerun_rejected", "POST", "/api/tasks/5/control", "", "valid", 409},
		{"missing_token", "GET", "/api/stats", "", "", 401},
		{"invalid_token", "POST", "/api/tasks/5/control", "", "invalid", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := telemetryFixture(t, &telemetryConnector{fail: tc.failure})
			s.jwtKey = []byte("isolated-fixture-key-not-a-production-secret")
			// Verify fixture RBAC succeeds explicitly: middleware currently permits
			// fallthrough on lookup errors, which must not make this test pass.
			access, err := s.m.pg.ResolveRBACAccess("fixture")
			if err != nil || access == nil || !access.User.Enabled || !access.Permissions["task:write"] {
				t.Fatalf("RBAC fixture lookup failed: %v, %+v", err, access)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/stats", s.stats)
			mux.HandleFunc("GET /api/tasks", s.listTasks)
			mux.HandleFunc("POST /api/tasks/{id}/control", s.control)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"action":"rerun"}`))
			if tc.token != "" {
				token := tc.token
				if token == "valid" {
					token, err = signJWT(s.jwtKey, "fixture", "fixture")
					if err != nil {
						t.Fatal(err)
					}
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			s.requireAuth(mux).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("HTTP %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if task.Status != "created" || task.Paused {
				t.Fatal("task lifecycle changed")
			}
			t.Logf("%s: HTTP %d", tc.name, rec.Code)
		})
	}
}
