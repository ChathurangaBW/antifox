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
