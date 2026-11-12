package agent

import (
	"strings"

	"antifox/engine/harness"
)

// Endnote(wrap-up / settlement prompt):When? agent Because [steps run out](MaxTurns)]or
// [Timeout(run_seconds/MaxDuration)] When terminated, the SDK settlement stage injects this hint.
// Jean. agent First, put down the identified but not returned contents, then output a sentence.,Avoid rotting tails.
//
// Each agent Endnotes can be covered as required in the background(Save agents.wrapup_prompt),Leave it here.
// contains the default. Editable only [the body of the message];Disables which tools and ends their own budget rounds as a fixed code strategy.

// WrapupOverride, if set, returns the stored wrap-up prompt for an agent key and
// whether a non-empty one exists. Wired by the server to the agents table (like
// PromptOverride for system prompts). nil / empty → the built-in default is used.
var WrapupOverride func(agentKey string) (string, bool)

// WrapupMaxTurnsOverride, if set, returns the admin-configured turn budget for the
// wrap-up phase of an agent and whether a positive one exists. Wired to the agents
// table. nil / ≤0 → the built-in per-agent default (wrapupTurnDefaults) is used.
var WrapupMaxTurnsOverride func(agentKey string) (int, bool)

// Internal Default Endnote,Press agent key Index.worker Re-enact history hard code. settleWrapUpPrompt
// (Define in worker.go),planner/mainagent One edition each.;Unsullied.(Custom agent)Go to the general pocket.
var wrapupDefaults = map[string]string{
	"worker":    settleWrapUpPrompt,
	"planner":   plannerWrapUpDefault,
	"mainagent": mainAgentWrapUpDefault,
}

// wrapupTurnDefaults: the per-agent default turn budget for the settlement stage (the backend value overrides it when greater than 0).
// worker Could be a few rounds.(record_fact/insert_assets)Here. 4;The rest. 5. It didn't hit him. genericWrapupTurns.
var wrapupTurnDefaults = map[string]int{
	"worker":    4,
	"planner":   5,
	"mainagent": 5,
}

const genericWrapupTurns = 5

const plannerWrapUpDefault = "This planning round is almost out of steps. This ends only the current round: the system will wake you again when the state changes, so do not try to close the entire task. Persist conclusions already reached without inventing filler intents; zero intents remains valid. (1) If you identified exploration directions that should be assigned now, submit them in one add_intent batch. (2) For goals proven by a finding or fact, call prove_goal to mark them met. (3) If you identified a serial exploitation chain, record its steps with TodoWrite for the next wake-up. Then end this round without a summary message."

const mainAgentWrapUpDefault = "This interaction is almost out of steps. Do not start any new exploration or operation. In one standalone plain-text sentence, summarize current progress, key conclusions, and the recommended next step for the user."

const genericWrapUpDefault = "This run is about to stop because its budget is exhausted. First persist every completed result that has not yet been recorded. Then provide one standalone plain-text sentence summarizing what you did and the key conclusions; that sentence will be displayed as this run's result."

// WrapupDefault returns the built-in default wrap-up prompt for an agent key —
// used by the admin UI as the "restore default" value and empty-field placeholder.
func WrapupDefault(agentKey string) string {
	if d, ok := wrapupDefaults[agentKey]; ok {
		return d
	}
	return genericWrapUpDefault
}

// WrapupTurnsDefault returns the built-in wrap-up turn budget for an agent key —
// used by the admin UI as the "0 = default N" hint.
func WrapupTurnsDefault(agentKey string) int {
	if n, ok := wrapupTurnDefaults[agentKey]; ok {
		return n
	}
	return genericWrapupTurns
}

// resolveWrapup returns the effective wrap-up prompt: the DB override (if set and
// non-empty) over the built-in default.
func resolveWrapup(agentKey string) string {
	if WrapupOverride != nil {
		if t, ok := WrapupOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return WrapupDefault(agentKey)
}

// resolveWrapupTurns returns the effective wrap-up turn budget: a positive DB
// override over the built-in per-agent default.
func resolveWrapupTurns(agentKey string) int {
	if WrapupMaxTurnsOverride != nil {
		if v, ok := WrapupMaxTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return WrapupTurnsDefault(agentKey)
}

// settlementLanguageRule is appended to the settlement (wrap-up) instruction for
// agents whose phase ends with a displayed summary sentence. That sentence is
// the model's terminal text verbatim (harness finish -> KindResult.Text -> the
// run result row), so its language is a pure generation choice. The settlement
// turn is one summarization prompt over a context that may be dominated by
// non-English engagement data; without an explicit rule the model mirrors that
// data even when every reasoning step and tool call in the run was English.
// Applied at construction so a DB-edited wrap-up prompt cannot drop it.
const settlementLanguageRule = "\n\nWrite the summary sentence in English, even when the intents, facts, or tool output being summarized are in another language."

// settlementPrompt applies the language rule to a resolved settlement prompt.
// The planner closes a round without a summary message, so the rule would only
// contradict it there.
func settlementPrompt(agentKey, prompt string) string {
	if agentKey == "planner" {
		return prompt
	}
	return prompt + settlementLanguageRule
}

// wrapupSettlement builds the settlement config for an agent's run. Prompt and the
// turn budget are admin-editable per agent; disabled tools are code-owned policy so
// a user can't edit away the "stop probing" guardrail. Resolved fresh each run
// (reads DB live), so edits apply on the next run without a restart.
func wrapupSettlement(agentKey string, disabledTools []string) *harness.Settlement {
	return &harness.Settlement{
		Prompt:        settlementPrompt(agentKey, resolveWrapup(agentKey)),
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
}

// ---------- Task-level overtime (see docs/Task-level overtime and end-of-service design.md)----------
//
// and per-run The last words are:per-run Yes."This time run I've run out of budgets.";Mission timed out.
// "The whole mission is at point. It's coming to an end.". Semantics are often the opposite (particularly) planner:per-run Say it."Keep planning.",
// The mission is overrated."Stop planning and make a final decision at point."). Only for worker/planner Configure.

// WrapupTaskTimeoutOverride / ...TurnsOverride:Task timeout end words and wheel numbers DB Overwrite
// (wire Present. agents.task_timeout_wrapup_prompt / _max_turns,Only worker/planner).
var (
	WrapupTaskTimeoutOverride      func(agentKey string) (string, bool)
	WrapupTaskTimeoutTurnsOverride func(agentKey string) (int, bool)
)

var taskTimeoutWrapupDefaults = map[string]string{
	"worker":  workerTaskTimeoutDefault,
	"planner": plannerTaskTimeoutDefault,
}

const workerTaskTimeoutDefault = "**The entire task has reached its time limit and is ending.** This is the deadline for the whole assessment, not merely this run's budget. This is your final opportunity to: (1) persist everything already identified but not yet recorded—new assets through insert_assets, exploration conclusions/facts through record_fact, and confirmed vulnerabilities through report_finding; (2) start no new commands or probes; and (3) provide one final standalone plain-text sentence summarizing the key conclusions for this intent."

const plannerTaskTimeoutDefault = "**The entire task has reached its time limit and is ending.** This is the final task deadline, not the end of one planning round. Make one final goal assessment from all current facts and findings. For each goal proven by evidence, call prove_goal to mark it met. **Do not generate any new intent**, because no new work will be executed. End immediately after the assessment without a summary message."

// TaskTimeoutWrapupDefault returns an agent's default task-timeout wrap-up text (used as the placeholder and for restoring the default).
func TaskTimeoutWrapupDefault(agentKey string) string {
	return taskTimeoutWrapupDefaults[agentKey] // Not configured(mainagent/chat)Return empty string
}

// resolveTaskTimeoutWrapup:DB Overwrite(Non-empty) > contains the default. Empty string indicates the agent No task timeout
// (Not worker/planner),The caller should turn back now. per-run Words.
func resolveTaskTimeoutWrapup(agentKey string) string {
	if WrapupTaskTimeoutOverride != nil {
		if t, ok := WrapupTaskTimeoutOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return TaskTimeoutWrapupDefault(agentKey)
}

func resolveTaskTimeoutTurns(agentKey string) int {
	if WrapupTaskTimeoutTurnsOverride != nil {
		if v, ok := WrapupTaskTimeoutTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return resolveWrapupTurns(agentKey) // Default Follow per-run Rounds
}

// wrapupSettlementForTask builds settlement for a worker/planner run that is aware
// of the task deadline. See §5 of the design doc:
//   - clamped=true  → This time. run Tasked deadline Snatch: 'Cause Timeout End=Mission to Point→Task timeout;
//     Because MaxTurns End=We've got a few minutes left to run out of steps in the trap window.→Fall Back per-run Words.
//   - clamped=false → Early mission: two reason Both. per-run Word (i.e. degraded to wrapupSettlement).
//
// Here the harness prompt is built by reason; the actual reason is used when closing, so there is no build mismatch.
func wrapupSettlementForTask(agentKey string, disabledTools []string, clamped bool) *harness.Settlement {
	perRun := settlementPrompt(agentKey, resolveWrapup(agentKey))
	st := &harness.Settlement{
		Prompt:        perRun, // Body (the reason is clamped to two occurrences per reason)
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
	if clamped {
		if tt := resolveTaskTimeoutWrapup(agentKey); tt != "" {
			st.PromptByReason = map[harness.TerminalReason]string{
				harness.ReasonTimeout:  settlementPrompt(agentKey, tt), // Mission to Point
				harness.ReasonMaxTurns: perRun,                         // We're running out of steps, we're running out of time.
			}
			st.MaxTurns = resolveTaskTimeoutTurns(agentKey)
		}
	}
	return st
}
