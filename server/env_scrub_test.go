package server

import (
	"os"
	"strings"
	"testing"

	"antifox/db"
)

// TestScrubSecretEnvRemovesSecrets checks that every startup secret is gone from
// the environment children inherit, and that unrelated variables survive.
func TestScrubSecretEnvRemovesSecrets(t *testing.T) {
	for _, k := range secretEnv {
		t.Setenv(k, "secret-"+k)
	}
	t.Setenv("ANTIFOX_UNRELATED", "keep")
	ScrubSecretEnv()
	for _, kv := range os.Environ() {
		for _, k := range secretEnv {
			if strings.HasPrefix(kv, k+"=") {
				t.Fatalf("%s still in the environment after scrub", k)
			}
		}
	}
	if os.Getenv("ANTIFOX_UNRELATED") != "keep" {
		t.Fatal("scrub removed an unrelated variable")
	}
}

// TestCaptureEnvLLMSurvivesScrub reproduces the production setup: the model is
// configured only through the environment and no profile is saved. After the
// scrub, the captured config must still back goal decomposition and the
// report/enrichment profile lookup, which previously turned themselves off.
func TestCaptureEnvLLMSurvivesScrub(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	if p, _ := m.pg.ActiveProfile(); p != nil {
		t.Skip("database has a saved active profile; the fallback does not apply")
	}
	dsn := os.Getenv("ANTIFOX_PG_DSN") // restore for later tests in this process
	t.Cleanup(func() {
		db.SetEnvFallbackProfile(nil)
		_ = os.Setenv("ANTIFOX_PG_DSN", dsn)
	})
	t.Setenv("ANTIFOX_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("ANTIFOX_LLM_MODEL", "deepseek-flash")
	t.Setenv("ANTIFOX_LLM_BASE_URL", "https://api.deepseek.com/v1")

	s := &Server{m: m}
	s.captureEnvLLM()
	ScrubSecretEnv()
	if os.Getenv("OPENAI_API_KEY") != "" {
		t.Fatal("key still in the environment after scrub")
	}

	cfg, ok := s.goalsLLMConfig(nil)
	if !ok || cfg.APIKey != "sk-test" || cfg.Model != "deepseek-flash" {
		t.Fatalf("goal decomposition config after scrub: ok=%v model=%q key set=%v", ok, cfg.Model, cfg.APIKey != "")
	}
	p, err := m.pg.ReportRoleProfile()
	if err != nil || p == nil || p.APIKey != "sk-test" || p.Format != "openai" || p.ID != 0 {
		t.Fatalf("report profile after scrub: %+v err=%v", p, err)
	}
	if ap, _ := m.pg.ActiveProfile(); ap != nil {
		t.Fatal("the fallback must not appear as a saved active profile")
	}
}
