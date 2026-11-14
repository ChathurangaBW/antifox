package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	actool "antifox/engine/tool"
)

// nilStoreInput is a deliberately broad argument object: every field a domain
// tool might read is populated, so each handler gets past input validation and
// actually reaches its exploration-store code path.
const nilStoreInput = `{
	"id": 1, "ids": [1], "intent_id": 1, "goal_id": 1, "evidence_id": 1,
	"text": "x", "summary": "x", "reason": "x", "detail": "x", "evidence": "x",
	"vulnclass": "x", "severity": "low", "query": "x", "q": "x", "message": "x",
	"intents": [{"summary": "x"}], "facts": [{"summary": "x"}], "hints": [{"text": "x"}]
}`

// callNoPanic runs one tool call and converts a panic into a returned error, so
// a single bad handler is reported by name instead of aborting the whole test.
func callNoPanic(tl actool.CoreTool, in string) (res actool.Result, err error, panicked any) {
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()
	res, err = tl.Call(context.Background(), json.RawMessage(in), &actool.ToolContext{})
	return
}

// TestDomainToolsWithoutExplorationStore covers the server-level domain tool
// registry, which is built with no exploration store and injected into chat
// agents (auto, pentest, custom). Tools run on their own goroutine without
// recovery, so a nil dereference here would crash the entire server process.
// Every tool must return normally; task-only tools must say why they cannot run.
func TestDomainToolsWithoutExplorationStore(t *testing.T) {
	ts := NewToolSet(nil, "")
	for _, tl := range ts.AllDomainTools() {
		tl := tl
		t.Run(tl.Name(), func(t *testing.T) {
			res, err, p := callNoPanic(tl, nilStoreInput)
			if p != nil {
				t.Fatalf("panicked without an exploration store: %v", p)
			}
			if err != nil {
				return // a returned error is a handled failure, not a crash
			}
			if _, taskOnly := taskOnlyTools[tl.Name()]; taskOnly {
				if !res.IsError || !strings.Contains(res.Flatten(), "task context") {
					t.Fatalf("task-only tool must return a task-context error, got error=%v %q", res.IsError, res.Flatten())
				}
			}
		})
	}
}

// TestTaskOnlyToolsListIsCurrent keeps taskOnlyTools honest: every name in it
// must be a real domain tool, so a rename cannot silently drop its guard.
func TestTaskOnlyToolsListIsCurrent(t *testing.T) {
	names := map[string]bool{}
	for _, tl := range NewToolSet(nil, "").AllDomainTools() {
		names[tl.Name()] = true
	}
	var missing []string
	for n := range taskOnlyTools {
		if !names[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		t.Fatal(fmt.Sprintf("taskOnlyTools names tools that do not exist: %v", missing))
	}
}
