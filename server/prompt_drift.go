package server

import (
	"log"
	"regexp"
	"sort"
	"strings"

	"antifox/db"
)

// promptToolToken matches identifiers like list_assets in a prompt.
var promptToolToken = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)

// promptToolDrift returns the tools a prompt names that the agent cannot call
// according to the tools table: the row exists but is disabled, or the agent is
// not in its binding. Names that are not tool keys (field names, prose) are
// ignored, so this only reports real tools.
func promptToolDrift(agentKey, prompt string, rows []*db.Tool) []string {
	byKey := make(map[string]*db.Tool, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r
	}
	seen := map[string]bool{}
	var out []string
	for _, tok := range promptToolToken.FindAllString(prompt, -1) {
		if seen[tok] {
			continue
		}
		seen[tok] = true
		row, ok := byKey[tok]
		if !ok {
			continue
		}
		switch {
		case !row.Enabled:
			out = append(out, tok+" (disabled)")
		case !contains(row.Agents, agentKey):
			out = append(out, tok+" (not bound)")
		}
	}
	sort.Strings(out)
	return out
}

// promptDriftAgents are the built-in agents whose prompts instruct tool use.
var promptDriftAgents = []string{"planner", "worker", "mainagent", "auto", "pentest"}

// warnPromptToolDrift logs, once at startup, every tool an agent's active prompt
// tells it to call that the tools table does not let it call. Prompts and
// bindings are both editable in the UI, so they can drift apart after deploy.
// That drift produced the planner's repeated "unknown tool list_assets" errors.
// It only logs: an operator may unbind a tool on purpose, and the fix is theirs
// to choose (rebind the tool or edit the prompt).
func (s *Server) warnPromptToolDrift() {
	rows, err := s.m.pg.ListTools()
	if err != nil {
		log.Printf("[prompt-check] could not load tools: %v", err)
		return
	}
	for _, key := range promptDriftAgents {
		a, err := s.m.pg.GetAgentByKey(key)
		if err != nil || a == nil {
			continue
		}
		prompt, err := s.m.pg.CurrentPrompt(a.ID)
		if err != nil || prompt == "" {
			continue
		}
		if drift := promptToolDrift(key, prompt, rows); len(drift) > 0 {
			log.Printf("[prompt-check] %s prompt names tools it cannot call: %s. Bind them to %s on the Tools page, or edit the prompt.",
				key, strings.Join(drift, ", "), key)
		}
	}
}
