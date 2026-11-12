package agent

import (
	"context"
	"encoding/json"
	"strings"

	"antifox/db"
	"antifox/engine/agentcore"
	acperm "antifox/engine/permission"
	actool "antifox/engine/tool"
)

// goalsDefaultTmpl is the built-in EDITABLE body (Paragraph [A]) of the goals-decomposer
// prompt, seeded into agent_prompts. No template vars are used today.
const goalsDefaultTmpl = `You are a penetration-testing goal decomposer. Your responsibility is to identify the **final outcomes to be achieved** from the user's input, not to plan attack steps.

**Goal = a final deliverable or independently verifiable outcome**

**The following are not goals and must not be listed as subgoals:**
- Information gathering, reconnaissance, or endpoint scanning
- Vulnerability analysis or validation activities
- Attack steps or exploitation techniques
- Steps used only to verify a result

**Decomposition rules:**
- If the user describes only one final outcome, output one goal.
- If the user requests multiple **independent** final deliverables, list them separately.
- Set vulnclass when a goal clearly corresponds to a vulnerability class, such as SQLi or IDOR. Leave it empty for business-logic or non-vulnerability outcomes.
- Never invent goals that the user did not request.

Call set_goals to submit the result.`

// GoalSpec is one decomposed objective.
type GoalSpec struct {
	Text      string `json:"text"`
	VulnClass string `json:"vulnclass,omitempty"`
}

// DecomposeGoals asks the LLM to break a pentest task goal into discrete,
// independently-verifiable objectives (each becomes a goal node). Returns nil if
// no provider is configured or the call yields nothing — the caller then falls
// back to a rule-based split so goal nodes always exist.
//
// desc is the task's free-text description (Background: range of targets/flag Number/Statements of engagement, etc.).
// It is fed alongside the goal so the decomposer no longer splits blind — the
// prompt still forbids inventing anything the two texts don't state.
//
// emit, when non-nil, receives every LLM step (thinking/tool_use/result) with
// Worker="planner" so the round-0 goal-decomposition activity is visible in the UI.
func DecomposeGoals(ctx context.Context, c Config, goalText, desc string, emit func(db.Activity)) []GoalSpec {
	prov, err := c.NewProvider()
	if err != nil {
		return nil
	}
	var out struct {
		Goals []GoalSpec `json:"goals"`
	}
	captured := false
	submitTool := actool.Build(actool.Spec{
		Name:        "set_goals",
		Description: "Submit the decomposed list of independently verifiable goals.",
		Schema: map[string]any{"type": "object", "properties": map[string]any{
			"goals": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "properties": map[string]any{
					"text":      map[string]any{"type": "string", "description": "One independently verifiable goal"},
					"vulnclass": map[string]any{"type": "string", "description": "The corresponding vulnerability class, when explicit (for example SQLi or IDOR); leave empty for business-logic goals"},
				}, "required": []string{"text"},
			}},
		}, "required": []string{"goals"}},
		Permissions: func(context.Context, json.RawMessage, acperm.Context) acperm.Decision { return acperm.Allowed() },
		Run: func(_ context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			_ = json.Unmarshal(in, &out)
			captured = true
			return actool.Text("ok"), nil
		},
	})
	// Description rides in the user message (same channel as the goal), NOT via the
	// {{.EngagementDescription}} template var — else a prompt that references the var
	// would inject the description twice. System prompt stays pure static instructions.
	sys := withEnglish(renderSystem("goals", goalsDefaultTmpl, GoalsVars{}))
	userMsg := "Task goal:\n" + goalText
	if d := strings.TrimSpace(desc); d != "" {
		userMsg += "\n\nTask description (background that may include target scope, flag count, or rules of engagement; use it only as context and do not invent anything it does not state):\n" + d
	}
	// Use captureRun so every LLM step is emitted as an activity record (visible in
	// the plan tab under the round-0 marker). Falls back gracefully when emit is nil.
	captureEmit := func(r db.Activity) {
		if emit != nil {
			r.Worker = "planner"
			emit(r)
		}
	}
	captureRun(ctx, agentcore.Options{
		Provider:               prov,
		SystemPrompt:           []string{sys},
		Tools:                  []actool.CoreTool{submitTool},
		PermissionMode:         acperm.ModeBypass,
		DisableBackgroundTasks: true,
		MaxTurns:               4,
	}, userMsg, captureEmit)
	if !captured {
		return nil
	}
	return out.Goals
}
