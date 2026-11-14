package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"antifox/db"
	actool "antifox/engine/tool"
)

// TaskBinding attaches a chat conversation to a task. A pentest (or any other
// chat) agent then works against that task's exploration graph: findings it
// reports, facts it records and assets it inserts land in the task, with the
// same record-time traffic linking and machine-checked verification a worker's
// findings get. Without a binding, conversations have no task context and those
// tools refuse with "requires a task context".
type TaskBinding struct {
	TaskID int64
	Name   string // task name, for the prompt (may be empty)
	Goal   string // task goal, for the prompt
	Store  *db.ExplorationStore
	Assets *db.AssetStore
	// Optional host callbacks, the same ones workers use.
	TrafficResolver func(taskID int64, repro json.RawMessage) json.RawMessage
	Verifier        FindingVerifier
	Notify          func() // wakes the task's planner after a hint
}

// toolSet builds the task-bound ToolSet for one chat turn. Writes are attributed
// to the agent key and anchored to the task root, like the planner's.
func (b *TaskBinding) toolSet(agentKey string) *ToolSet {
	tsx := NewToolSet(b.Store, agentKey)
	if b.Assets != nil {
		tsx.SetAssetStore(b.Assets, b.Assets.Companies())
	}
	tsx.SetTaskID(b.TaskID)
	tsx.SetTrafficResolver(b.TrafficResolver)
	tsx.SetFindingVerifier(b.Verifier)
	tsx.SetNotify(b.Notify)
	if origin, _ := b.Store.OriginFactID(); origin > 0 {
		tsx.SetOwnerNode(origin)
	}
	return tsx
}

// promptBlock tells the agent which task its work is saved to. Code-owned, so an
// edited prompt cannot drop it.
func (b *TaskBinding) promptBlock() string {
	label := fmt.Sprintf("#%d", b.TaskID)
	if n := strings.TrimSpace(b.Name); n != "" {
		label += " (" + n + ")"
	}
	var s strings.Builder
	s.WriteString("\n\nTask attachment: this conversation is attached to task " + label + ".")
	if g := strings.TrimSpace(b.Goal); g != "" {
		s.WriteString(" Task goal: " + g)
	}
	s.WriteString("\nFindings you record with report_finding, facts, and assets you insert are saved to this task, and list_findings reads its findings. " +
		"Reproduction steps are linked to the recorded traffic of this task, and findings are verified when you pass poe_level and evidence_quotes. " +
		"There is no intent in a conversation, so leave intent_id empty.")
	return s.String()
}

// Tools returns the task-bound domain tools for agentKey. The chat agent puts
// them in its base list, where they take precedence over store-less copies; the
// tools table then filters them by the agent's bindings.
func (b *TaskBinding) Tools(agentKey string) []actool.CoreTool {
	return b.toolSet(agentKey).AllDomainTools()
}
