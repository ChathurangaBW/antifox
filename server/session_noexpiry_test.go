package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Auto-logout is disabled by default: tokens carry no exp claim, still
// authenticate, and are not refreshed. A configured TTL still expires tokens.
func TestSessionDoesNotExpireByDefault(t *testing.T) {
	key := []byte("isolated-session-test-key")
	tok, err := signJWTTTL(key, "fixture", "fixture", 0, defaultSessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	claims, ok := parseJWT(tok, key)
	if !ok || claims.ExpiresAt != nil {
		t.Fatalf("default token must be valid without exp: ok=%v exp=%v", ok, claims)
	}

	s, _ := telemetryFixture(t, &telemetryConnector{})
	s.jwtKey = key
	req := httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("X-Refresh-Token") != "" {
		t.Fatalf("non-expiring token: status=%d refresh=%q", rec.Code, rec.Header().Get("X-Refresh-Token"))
	}

	expired, err := signJWTTTL(key, "fixture", "fixture", 0, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parseJWT(expired, key); !ok {
		// negative ttl is treated as "no expiry" too; must not produce an expired token
		t.Fatal("non-positive ttl must not produce an already-expired token")
	}
	short, _ := signJWTTTL(key, "fixture", "fixture", 0, time.Nanosecond)
	time.Sleep(1100 * time.Millisecond)
	if _, ok := parseJWT(short, key); ok {
		t.Fatal("explicitly configured TTL must still expire")
	}
}
