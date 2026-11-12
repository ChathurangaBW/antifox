package agent

import (
	"strings"
	"testing"
)

func TestRenderSystemOverrideAndFallback(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })

	// no override → built-in default
	PromptOverride = nil
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "g"}); got != "DEFAULT" {
		t.Fatalf("no override should give default, got %q", got)
	}

	// override → rendered with vars
	PromptOverride = func(k string) (string, bool) {
		if k == "planner" {
			return "Objective:{{.Goal}} Scope:{{.Scope}}", true
		}
		return "", false
	}
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "Take it.X", Scope: "*.x.com"}); got != "Objective:Take it.X Scope:*.x.com" {
		t.Fatalf("override render: %q", got)
	}

	// override referencing a non-catalog var → execution error → fallback to default
	PromptOverride = func(k string) (string, bool) { return "{{.NotInCatalog}}", true }
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "x"}); got != "DEFAULT" {
		t.Fatalf("bad var should fall back to default, got %q", got)
	}

	// full plannerSystem path: DB body [A] is honored, then the code-owned tail
	// [C] (intermediate-artifact policy) is ALWAYS appended — editing the body can't drop it.
	PromptOverride = func(k string) (string, bool) { return "PLANNER {{.Goal}}", true }
	got := plannerSystem("Take it.X", "/data")
	if !strings.HasPrefix(got, "PLANNER Take it.X") {
		t.Fatalf("plannerSystem body not honored: %q", got)
	}
	if !strings.Contains(got, "Intermediate-artifact policy") || !strings.Contains(got, "/data") {
		t.Fatalf("plannerSystem missing code-owned artifact tail: %q", got)
	}

	// worker dual-text via {{if .ProxyAddr}} in a user template, plus the code tail:
	// [B] trafficTool present only with a proxy, [C] artifact spec always present.
	PromptOverride = func(k string) (string, bool) {
		return "{{if .ProxyAddr}}Go, Agent. {{.ProxyAddr}}{{else}}Manual{{end}}", true
	}
	withProxy := workerSystem("127.0.0.1:8080", "/data")
	if !strings.HasPrefix(withProxy, "Go, Agent. 127.0.0.1:8080") {
		t.Fatalf("worker proxy branch body: %q", withProxy)
	}
	if !strings.Contains(withProxy, "traffic_search") {
		t.Fatalf("worker with proxy should inject trafficTool: %q", withProxy)
	}
	if !strings.Contains(withProxy, "Intermediate-artifact policy") {
		t.Fatalf("worker missing artifact tail: %q", withProxy)
	}
	noProxy := workerSystem("", "/data")
	if !strings.HasPrefix(noProxy, "Manual") {
		t.Fatalf("worker no-proxy branch body: %q", noProxy)
	}
	if strings.Contains(noProxy, "traffic_search") {
		t.Fatalf("worker without proxy must NOT inject trafficTool: %q", noProxy)
	}
}

// TestEnglishOutputDirective locks the fix for workers answering in Chinese:
// every composed system prompt carries the code-owned output-language rule, and
// a DB prompt body (legacy Chinese, or a user edit) cannot drop it.
func TestEnglishOutputDirective(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })

	built := map[string]string{
		"planner":        plannerSystem("g", "/data"),
		"worker":         workerSystem("", "/data"),
		"worker-proxy":   workerSystem("127.0.0.1:8080", "/data"),
		"mainagent":      mainAgentSystem("g", "/data"),
		"chat":           chatSystem("custom", "/data"),
		"goals-rendered": withEnglish(renderSystem("goals", goalsDefaultTmpl, GoalsVars{})),
	}
	for name, got := range built {
		if !strings.Contains(got, "Write every reasoning summary, finding, fact, and final answer in English") {
			t.Fatalf("%s: missing English output directive", name)
		}
	}

	// A legacy Chinese body in the DB must still yield an English-language rule.
	PromptOverride = func(k string) (string, bool) { return "\u6211\u662f\u4e2d\u6587\u63d0\u793a", true }
	if got := workerSystem("", "/data"); !strings.Contains(got, "Write every reasoning summary") {
		t.Fatalf("DB body dropped the English directive: %q", got)
	}
}
