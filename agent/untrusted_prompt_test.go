package agent

import (
	"regexp"
	"strings"
	"testing"
)

// TestUntrustedRulesSurvivePromptOverride: the injection rules are code-owned,
// so an operator-edited prompt (the DB override path) must still carry them.
func TestUntrustedRulesSurvivePromptOverride(t *testing.T) {
	prev := PromptOverride
	t.Cleanup(func() { PromptOverride = prev })
	PromptOverride = func(string) (string, bool) { return "Edited prompt with no safety text.", true }

	cases := map[string]struct {
		system string
		want   string
	}{
		"planner":   {plannerSystem("goal", "/w"), untrustedDataRules},
		"worker":    {workerSystem("", "/w"), untrustedContentRules},
		"mainagent": {mainAgentSystem("goal", "/w"), untrustedContentRules},
		"chat":      {chatSystem("pentest", "/w"), untrustedContentRules},
	}
	for name, c := range cases {
		if !strings.Contains(c.system, "Edited prompt with no safety text.") {
			t.Errorf("%s: override not applied, test is not exercising the edit path", name)
		}
		if strings.Count(c.system, c.want) != 1 {
			t.Errorf("%s: injection rules present %d times, want exactly 1", name, strings.Count(c.system, c.want))
		}
	}
}

// TestUntrustedRulesNotDuplicated: a prompt that already contains the rules
// (pasted in by an operator) must not get a second copy.
func TestUntrustedRulesNotDuplicated(t *testing.T) {
	prev := PromptOverride
	t.Cleanup(func() { PromptOverride = prev })
	PromptOverride = func(key string) (string, bool) {
		if key == "planner" {
			return "Body.\n\n" + untrustedDataRules, true
		}
		return "Body.\n\n" + untrustedContentRules, true
	}
	if n := strings.Count(plannerSystem("g", "/w"), untrustedDataRules); n != 1 {
		t.Errorf("planner rules appear %d times", n)
	}
	if n := strings.Count(workerSystem("", "/w"), untrustedContentRules); n != 1 {
		t.Errorf("worker rules appear %d times", n)
	}
}

// TestPlannerRulesNameRealTools: every tool-like name in the planner rules must
// be a tool the planner actually has, so the rules cannot introduce a phantom
// tool reference (the defect behind the planner's list_assets failures).
func TestPlannerRulesNameRealTools(t *testing.T) {
	have := map[string]bool{}
	for _, tl := range NewToolSet(nil, "").PlannerTools() {
		have[tl.Name()] = true
	}
	for _, tok := range regexp.MustCompile(`\b[a-z]+(?:_[a-z]+)+\b`).FindAllString(untrustedDataRules, -1) {
		if tok == "recent_facts" {
			continue // a graph_overview field, not a tool
		}
		if !have[tok] {
			t.Errorf("planner rules name %q, which the planner does not have", tok)
		}
	}
}
