package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizationFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, failure string
		want          int
	}{
		{"user_lookup", "auth_user/query", 503},
		{"role_lookup", "auth_roles/query", 503},
		{"permission_lookup", "auth_permissions/query", 503},
		{"missing_user", "auth_user/missing", 503},
		{"missing_manager", "", 503},
		{"missing_database", "", 503},
		{"empty_subject", "", 401},
		{"valid_access", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := telemetryFixture(t, &telemetryConnector{fail: tc.failure})
			s.jwtKey = []byte("isolated-auth-regression-key")
			subject := "fixture"
			switch tc.name {
			case "missing_manager":
				s.m = nil
			case "missing_database":
				s.m.pg = nil
			case "empty_subject":
				subject = ""
			}
			token, err := signJWT(s.jwtKey, subject, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("GET", "/api/stats", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			called := false
			s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(204)
			})).ServeHTTP(rec, req)
			if rec.Code != tc.want || called != (tc.want == 204) {
				t.Fatalf("status=%d want=%d handler_called=%v", rec.Code, tc.want, called)
			}
			if strings.Contains(rec.Body.String(), "private") {
				t.Fatal("database detail leaked")
			}
			t.Logf("HTTP %d; handler_called=%v", rec.Code, called)
		})
	}
}
