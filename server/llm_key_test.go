package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"antifox/agent"
	"antifox/db"
)

func TestSameLLMEndpointNormalization(t *testing.T) {
	cases := []struct {
		pa, ba, pb, bb string
		want           bool
	}{
		{"openai", "https://api.deepseek.com/v1", "openai", "https://api.deepseek.com/v1/", true},
		{"openai", "https://api.deepseek.com/v1/chat/completions", "openai", "https://api.deepseek.com/v1", true},
		{"openai", "https://api.deepseek.com/v1", "openai", "https://attacker.example/v1", false},
		{"openai", "https://api.deepseek.com/v1", "anthropic", "https://api.deepseek.com/v1", false},
	}
	for _, c := range cases {
		if got := sameLLMEndpoint(c.pa, c.ba, c.pb, c.bb); got != c.want {
			t.Errorf("sameLLMEndpoint(%s %s, %s %s) = %v, want %v", c.pa, c.ba, c.pb, c.bb, got, c.want)
		}
	}
}

func postJSON(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b)))
	return rec
}

// TestTestLLMWithholdsGlobalKeyFromOtherHost: the connection test must not send
// the configured key to a base URL the request names. It answers before any
// network call, so the test needs no listener at the "attacker" address.
func TestTestLLMWithholdsGlobalKeyFromOtherHost(t *testing.T) {
	s := &Server{m: &Manager{}}
	s.llmCfg = agent.ConfigFrom("openai", "deepseek-flash", "https://api.deepseek.com/v1", "sk-secret", "")
	rec := postJSON(t, s.testLLM, map[string]any{"provider": "openai", "model": "m", "base_url": "https://attacker.example/v1"})
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["ok"] != false || !strings.Contains(out["error"].(string), "only sent to the endpoint") {
		t.Fatalf("expected the key to be withheld, got %v", out)
	}
}

// TestSetLLMRefusesKeyForOtherHost: saving the global config with a blank key
// must not move the existing key to a new endpoint.
func TestSetLLMRefusesKeyForOtherHost(t *testing.T) {
	s := &Server{m: &Manager{}}
	s.llmCfg = agent.ConfigFrom("openai", "deepseek-flash", "https://api.deepseek.com/v1", "sk-secret", "")
	rec := postJSON(t, s.setLLM, map[string]any{"provider": "openai", "model": "m", "base_url": "https://attacker.example/v1"})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "only sent to the endpoint") {
		t.Fatalf("expected 400 refusing the key move, got %d %s", rec.Code, rec.Body.String())
	}
	if s.llmCfg.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatal("the running config changed despite the refusal")
	}
}

// TestSaveProfileRefusesKeyForOtherHost: editing a saved profile's endpoint
// without re-entering the key is refused; editing other fields still works.
func TestSaveProfileRefusesKeyForOtherHost(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	id, err := m.pg.SaveProfile(&db.LLMProfile{Name: "keytest", Format: "openai", Model: "m", BaseURL: "https://api.deepseek.com/v1", APIKey: "sk-secret"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.pg.DeleteProfile(id) })
	s := &Server{m: m}

	moved := postJSON(t, s.pgSaveProfile, map[string]any{"id": id, "name": "keytest", "format": "openai", "model": "m", "base_url": "https://attacker.example/v1"})
	if moved.Code != 400 {
		t.Fatalf("endpoint change without key: got %d %s, want 400", moved.Code, moved.Body.String())
	}
	if p, _ := m.pg.ProfileByID(id); p == nil || p.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatal("the stored profile changed despite the refusal")
	}
	renamed := postJSON(t, s.pgSaveProfile, map[string]any{"id": id, "name": "keytest-renamed", "format": "openai", "model": "m2", "base_url": "https://api.deepseek.com/v1"})
	if renamed.Code != 200 {
		t.Fatalf("editing other fields: got %d %s, want 200", renamed.Code, renamed.Body.String())
	}
	if p, _ := m.pg.ProfileByID(id); p == nil || p.APIKey != "sk-secret" || p.Model != "m2" {
		t.Fatal("a same-endpoint edit must keep the key and apply the change")
	}
}
