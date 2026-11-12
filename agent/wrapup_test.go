package agent

import (
	"strings"
	"testing"

	"antifox/engine/harness"
)

// TestSettlementPromptEnglishRule covers the visible run result: it is the
// model's terminal text from the settlement turn verbatim, so the settlement
// prompt itself must demand English - otherwise the model mirrors Chinese
// engagement data and the result line comes back in Chinese even though the
// whole run was English.
func TestSettlementPromptEnglishRule(t *testing.T) {
	t.Cleanup(func() { WrapupOverride = nil })

	for _, key := range []string{"worker", "mainagent", "custom-agent"} {
		if got := wrapupSettlement(key, nil).Prompt; !strings.Contains(got, "Write the summary sentence in English") {
			t.Fatalf("%s settlement prompt missing the English rule", key)
		}
	}

	// A DB-edited wrap-up prompt (legacy Chinese or a user customization) must
	// still produce an English instruction.
	WrapupOverride = func(k string) (string, bool) { return "\u8bf7\u7528\u4e2d\u6587\u603b\u7ed3", true }
	if got := wrapupSettlement("worker", nil).Prompt; !strings.Contains(got, "Write the summary sentence in English") {
		t.Fatalf("DB wrap-up prompt dropped the English rule: %q", got)
	}

	// The planner closes a round without a summary message; the rule would only
	// contradict it.
	if got := wrapupSettlement("planner", nil).Prompt; strings.Contains(got, "Write the summary sentence in English") {
		t.Fatalf("planner settlement must not carry the summary rule: %q", got)
	}

	// Task-timeout settlement: both reasons end in a displayed result sentence.
	st := wrapupSettlementForTask("worker", []string{"Bash"}, true)
	for _, reason := range []harness.TerminalReason{harness.ReasonTimeout, harness.ReasonMaxTurns} {
		if !strings.Contains(st.PromptByReason[reason], "Write the summary sentence in English") {
			t.Fatalf("task-timeout settlement (%s) missing the English rule", reason)
		}
	}
}
