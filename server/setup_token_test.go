package server

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"net/http"
	"strings"
	"testing"
)

func TestCheckSetupToken(t *testing.T) {
	s := &Server{}
	if s.checkSetupToken("") || s.checkSetupToken("anything") {
		t.Fatal("no active token must reject every value, including empty")
	}
	s.setupToken = "abc123"
	if s.checkSetupToken("wrong") || s.checkSetupToken("") {
		t.Fatal("wrong or empty token accepted")
	}
	if !s.checkSetupToken("abc123") || !s.checkSetupToken("  abc123\n") {
		t.Fatal("correct token (surrounding whitespace allowed) rejected")
	}
	s.clearSetupToken()
	if s.checkSetupToken("abc123") {
		t.Fatal("token still accepted after it was retired")
	}
}

// TestAuthInitRequiresSetupToken: on a fresh database with no admin, init
// without the token is refused with 403, and init with it succeeds. It uses its
// own database so an admin created by another test cannot hide the first run.
func TestAuthInitRequiresSetupToken(t *testing.T) {
	base := os.Getenv("ANTIFOX_PG_DSN")
	if base == "" {
		t.Skip("postgres unavailable — skipping")
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Skipf("unparseable DSN: %v", err)
	}
	name := fmt.Sprintf("setup_token_%d", time.Now().UnixNano())
	fresh := *u
	fresh.Path = "/" + name
	t.Setenv("ANTIFOX_PG_DSN", fresh.String()) // db.Open creates the database
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skipf("could not open a fresh database: %v", err)
	}
	t.Cleanup(func() {
		m.Close()
		if admin, err := sql.Open("pgx", base); err == nil {
			_, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
			admin.Close()
		}
	})
	if needs, err := m.pg.RBACNeedsBootstrap(); err != nil || !needs {
		t.Fatalf("fresh database should need bootstrap: needs=%v err=%v", needs, err)
	}

	s := &Server{m: m, jwtKey: []byte("0123456789abcdef0123456789abcdef")}
	s.initSetupToken()
	if s.setupToken == "" {
		t.Fatal("no setup token generated on a database without an admin")
	}
	const pw = "Str0ng-Passw0rd!x"

	denied := postJSON(t, s.authInit, map[string]any{"password": pw})
	if denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), "setup token") {
		t.Fatalf("init without token: got %d %s, want 403", denied.Code, denied.Body.String())
	}
	wrong := postJSON(t, s.authInit, map[string]any{"password": pw, "setup_token": "nope"})
	if wrong.Code != http.StatusForbidden {
		t.Fatalf("init with wrong token: got %d, want 403", wrong.Code)
	}
	if needs, _ := m.pg.RBACNeedsBootstrap(); !needs {
		t.Fatal("an admin was created without the setup token")
	}

	ok := postJSON(t, s.authInit, map[string]any{"password": pw, "setup_token": s.setupToken})
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "token") {
		t.Fatalf("init with token: got %d %s, want 200 with a session token", ok.Code, ok.Body.String())
	}
	if s.setupToken != "" {
		t.Fatal("setup token not retired after a successful init")
	}
}
