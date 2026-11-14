package server

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"antifox/agent"
	"antifox/db"
	actool "antifox/engine/tool"
	"antifox/traffic"
)

// snakeToken matches identifiers like list_assets or update_skill_file.
var snakeToken = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)

// toolVerb matches identifiers that read as a tool call rather than a field
// name. A token like this that is not a real tool is almost certainly a phantom
// reference the model will try, and fail, to call.
var toolVerb = regexp.MustCompile(`^(create|update|delete|list|get|add|search|insert|report|record|prove|kill|steer|spawn|pause|set|ensure)_`)

// retiredToolNames were once named in prompts but never existed in code.
var retiredToolNames = []string{"asset_neighbors", "update_skill"}

// knownPromptGaps are prompt references that only work in some contexts. The
// pentest agent can record and list findings only when its conversation is
// attached to a task (TaskBinding); an unattached conversation gets a clear error.
// Anything not listed here fails the test.
var knownPromptGaps = map[string][]string{
	"pentest": {"report_finding", "list_findings"},
}

func toolNames(tools []actool.CoreTool) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Name())
	}
	return out
}

func nameSet(lists ...[]string) map[string]bool {
	m := map[string]bool{}
	for _, l := range lists {
		for _, n := range l {
			m[n] = true
		}
	}
	return m
}

// seededFor returns the built-in tools whose default binding includes agentKey.
func seededFor(agentKey string) []string {
	var out []string
	for _, sd := range agent.BuiltinToolSeeds() {
		for _, a := range sd.Agents {
			if a == agentKey {
				out = append(out, sd.Key)
				break
			}
		}
	}
	return out
}

// TestPromptToolReferencesMatchBindings fails when a built-in prompt tells an
// agent to call a tool that agent does not get. That drift caused the planner's
// repeated "unknown tool list_assets" failures in production: the prompt named
// the tool, but it was neither in the planner's code tool set nor bound to it.
func TestPromptToolReferencesMatchBindings(t *testing.T) {
	s := &Server{}
	orchestration := toolNames(s.orchestrationTools())
	platform := toolNames(s.platformTools())
	kali := []string{s.toolEnsureKaliTool().Name()}
	trafficTools := toolNames((&traffic.Traffic{}).Tools())
	general := toolNames(actool.DefaultTools())
	// set_goals is built inline by agent.DecomposeGoals for the one-shot goal
	// decomposition run, so no shared tool set lists it.
	goalsSubmit := []string{"set_goals"}
	ts := agent.NewToolSet(nil, "")

	// What each agent can actually call: its code tool set for task agents, its
	// default bindings for chat agents, plus the general tools everyone gets.
	have := map[string]map[string]bool{
		"planner":   nameSet(general, toolNames(ts.PlannerTools())),
		"worker":    nameSet(general, toolNames(ts.WorkerTools()), trafficTools, kali),
		"mainagent": nameSet(general, toolNames(ts.MainAgentTools())),
		"auto":      nameSet(general, seededFor("auto"), orchestration, platform, kali),
		"pentest":   nameSet(general, seededFor("pentest"), kali),
		"goals":     nameSet(general, goalsSubmit),
	}
	chatAgents := map[string]bool{"auto": true, "pentest": true}

	universe := nameSet(general, toolNames(ts.AllDomainTools()), orchestration, platform, kali, trafficTools, goalsSubmit)
	retired := nameSet(retiredToolNames)

	for key, prompt := range agent.BuiltinPromptSeeds() {
		avail, ok := have[key]
		if !ok {
			t.Errorf("agent %q has a built-in prompt but no expected tool set in this test; add one", key)
			continue
		}
		gaps := nameSet(knownPromptGaps[key])
		var problems []string
		seen := map[string]bool{}
		for _, tok := range snakeToken.FindAllString(prompt, -1) {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			switch {
			case retired[tok]:
				problems = append(problems, tok+": retired name, the tool does not exist")
			case universe[tok] && !avail[tok]:
				problems = append(problems, tok+": named in the prompt but not available to this agent")
			case universe[tok] && chatAgents[key] && agent.IsTaskOnlyTool(tok) && !gaps[tok]:
				problems = append(problems, tok+": needs a task, but this agent runs without one")
			case !universe[tok] && toolVerb.MatchString(tok):
				problems = append(problems, tok+": looks like a tool call, but no tool has this name")
			}
		}
		if len(problems) > 0 {
			sort.Strings(problems)
			t.Errorf("prompt for %q:\n  %s", key, strings.Join(problems, "\n  "))
		}
	}
}

// TestPromptToolDriftFindsLiveBindingGap replays the binding row found in the
// production database: list_assets bound to every agent except the planner,
// whose prompt tells it to call list_assets.
func TestPromptToolDriftFindsLiveBindingGap(t *testing.T) {
	rows := []*db.Tool{
		{Key: "list_assets", Enabled: true, Agents: []string{"mainagent", "worker", "auto", "pentest"}},
		{Key: "list_facts", Enabled: true, Agents: []string{"mainagent", "planner", "worker"}},
		{Key: "prove_goal", Enabled: false, Agents: []string{"planner"}},
	}
	prompt := "Call list_assets on demand, list_facts for facts, prove_goal when proven. Pass asset_ids from list_assets."
	got := promptToolDrift("planner", prompt, rows)
	want := []string{"list_assets (not bound)", "prove_goal (disabled)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("drift = %v, want %v", got, want)
	}
	// The worker has list_assets and list_facts; a disabled tool is reported as
	// disabled before its binding is checked.
	if got := promptToolDrift("worker", prompt, rows); strings.Join(got, "|") != "prove_goal (disabled)" {
		t.Fatalf("worker drift = %v, want [prove_goal (disabled)]", got)
	}
}
