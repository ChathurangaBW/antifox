package agent

import (
	"context"
	"encoding/json"

	actool "antifox/engine/tool"
)

// taskOnlyTools names the domain tools that read or write a task's exploration
// graph. They need a ToolSet built with an ExplorationStore. The server-level
// registry injected into chat agents (auto, pentest, custom) has none, and these
// handlers would otherwise dereference a nil store. Tools run on their own
// goroutine, so that panic would take down the whole server process.
//
// TestTaskOnlyToolsListIsCurrent fails if a name here stops matching a real tool,
// and TestDomainToolsWithoutExplorationStore fails if a tool that needs the store
// is missing from this list.
var taskOnlyTools = map[string]struct{}{
	"add_hint":                 {},
	"add_intent":               {},
	"get_worker_output":        {},
	"get_worker_trace":         {},
	"graph_overview":           {},
	"list_facts":               {},
	"list_findings":            {},
	"list_goals":               {},
	"list_worker_traces":       {},
	"node_detail":              {},
	"prove_goal":               {},
	"record_fact":              {},
	"report_finding":           {},
	"search_all_worker_traces": {},
}

// IsTaskOnlyTool reports whether a tool needs a task's exploration graph.
func IsTaskOnlyTool(name string) bool {
	_, ok := taskOnlyTools[name]
	return ok
}

// taskContextError is returned to the model, so it says what to do instead.
// The alternatives follow ARTEX: hand the result to the task as a hint, or read
// through the cross-task tools that take an explicit task_id.
func taskContextError(name string) string {
	return name + " requires a task context: this agent is not running inside a task, so there is no exploration graph to read or write. " +
		"To get a finding or fact recorded, send it to the task with add_task_hint and include the evidence, so the task's own agents register it. " +
		"To read a task's graph, use a cross-task tool that takes a task_id, such as get_task_graph or list_task_findings. " +
		"If none of these tools are available to you, report the result in your reply instead."
}

// taskOnlyTool wraps a task-only tool. It embeds the original, so the name,
// description, schema, and permission behavior are unchanged; only Call checks
// for a store first.
type taskOnlyTool struct {
	actool.CoreTool
	hasStore bool
}

func (w *taskOnlyTool) Call(ctx context.Context, in json.RawMessage, tc *actool.ToolContext) (actool.Result, error) {
	if !w.hasStore {
		return actool.Errorf(taskContextError(w.Name())), nil
	}
	return w.CoreTool.Call(ctx, in, tc)
}

// guardTaskOnly wraps every task-only tool in list when this ToolSet has no
// exploration store. With a store, the list is returned unchanged.
func (t *ToolSet) guardTaskOnly(list []actool.CoreTool) []actool.CoreTool {
	if t.ts != nil {
		return list
	}
	out := make([]actool.CoreTool, len(list))
	for i, tl := range list {
		if _, ok := taskOnlyTools[tl.Name()]; ok {
			out[i] = &taskOnlyTool{CoreTool: tl, hasStore: false}
		} else {
			out[i] = tl
		}
	}
	return out
}
