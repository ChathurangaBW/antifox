package agent

import (
	"context"
	"encoding/json"

	actool "antifox/engine/tool"
)

// This document changes the "Inline Tools" from a pure code to a listable, accessible DB Synchronising folder
//   - BuiltinToolSeeds():I'll execute three. agent Expand the integration of built-in tools seed Recordkey +
//     Description + Parameters schema + Default bound agent),For the service end to plant, etc. tools Table.
//   - ToolResolve Hook: Press on running DB Lee. tools line to do "Press on installed tools" agent Filter +
//     Overwrite Description/schema + Injecting parameter defaults.key/handler It's still on the code level.DB This post is part of our special coverage Syria Protests 2011.
// The handler (the call behaviour) always comes from the code, so the DB cannot change it; only the description and the defaults the model sees are editable.

// ToolSeed is a serializable snapshot of a built-in tool: the key, the CoreTool.Name() (and handler) that
// the UI only reads it. Desc/Schema is a tool definition taken from the code. Agents receive it by default from the code.
type ToolSeed struct {
	Key    string         // = CoreTool.Name(),Primary key, unalterable
	Desc   string         // Top Description UI Overwrite)
	Schema map[string]any // Parameter JSON Schema (the structure is read-only; the description and default values are editable in the UI)
	Agents []string       // Default bound agent key(worker/planner/mainagent)
}

// builtinToolsByAgent uses an empty shell.ToolSet (nil stores) to build each execution agent.
// Area tool set. Tool Construct function only closes Spec, Construct period does not solve references store,So... nil Clear.——
// These tools are only for reading here. Name()/Description()/InputSchema(),Never. Call.
//
// I don't know.SDK General Tools actool.DefaultTools()(Read/Write/Edit/MultiEdit/LS/Glob/
// Grep/Bash):Each of them. agent They have fixed ownership, no "bound to whom" trade-offs, and most of them are Prompt()
// List (this table only covers Description()). A half-covered listing would be misleading, so seed -> DB -> ToolResolve
// The conduct was consistent with that of the former. Only antifox Their own domain tool entry forms are manageable.
func builtinToolsByAgent() map[string][]actool.CoreTool {
	ts := NewToolSet(nil, "")
	return map[string][]actool.CoreTool{
		"mainagent": ts.MainAgentTools(),
		"planner":   ts.PlannerTools(),
		"worker":    ts.WorkerTools(),
		// auto Default breach reporting + Asset management tool + Knowledge base search, other domain tools available UI Check as required.
		// The new library seed writing;old cougar seedAutoDefaultBindings Move.
		"auto": {ts.addFinding(), ts.insertAssets(), ts.addCompanyScope(), ts.listAssets(), ts.listCompanies(), ts.searchKnowledge(), ts.listKnowledgeCategories()},
		// pentest (independent penetration-testing agent) default bindings: check assets / insert assets / report findings / list findings / business logic / knowledge-base search.
		// The new library seed writing;old cougar seedPentestDefaultBindings Move.
		"pentest": {ts.listAssets(), ts.insertAssets(), ts.addFinding(), ts.listFindings(), ts.listCompanies(), ts.searchKnowledge(), ts.listKnowledgeCategories()},
	}
}

// BuiltinToolSeeds Three. agent Internal toolsets to merge into seed List: Tools with the same name (e.g.
// list_assets bundles them together for the agents. The order is stable (mainagent -> planner -> worker).
func BuiltinToolSeeds() []ToolSeed {
	byAgent := builtinToolsByAgent()
	order := []string{"mainagent", "planner", "worker", "auto", "pentest"}

	type acc struct {
		tool   actool.CoreTool
		agents []string
	}
	m := map[string]*acc{}
	var keys []string
	for _, ak := range order {
		for _, t := range byAgent[ak] {
			a, ok := m[t.Name()]
			if !ok {
				a = &acc{tool: t}
				m[t.Name()] = a
				keys = append(keys, t.Name())
			}
			a.agents = append(a.agents, ak)
		}
	}

	out := make([]ToolSeed, 0, len(keys))
	for _, k := range keys {
		a := m[k]
		out = append(out, ToolSeed{
			Key:    k,
			Desc:   a.tool.Description(),
			Schema: a.tool.InputSchema(),
			Agents: a.agents,
		})
	}
	return out
}

// ToolResolve, if set, post-processes an agent's fully-assembled tool list against
// the DB tools table: it drops tools not bound to this agent (or globally disabled)
// and wraps the rest so the model sees the DB-overridden description/schema and
// Default Participation get injected. Tools with no matching DB row (MCP/skill/host tools like
// traffic) pass through untouched. nil = tools unchanged. Wired in server/assembly.go.
var ToolResolve func(ctx context.Context, agentKey string, tools []actool.CoreTool) []actool.CoreTool

// DecorateTool wraps t so Description()/InputSchema() report the DB overrides and
// Call() injects scalar parameter defaults (from schema's "default" props) whenever
// the model omitted them. Name/Prompt/permission/scheduler flags delegate to t, so
// the tool's identity and handler are unchanged. Empty desc/schema fall back to t's.
func DecorateTool(t actool.CoreTool, desc string, schema map[string]any) actool.CoreTool {
	if desc == "" {
		desc = t.Description()
	}
	if len(schema) == 0 {
		schema = t.InputSchema()
	}
	return &overriddenTool{CoreTool: t, desc: desc, schema: schema}
}

// overriddenTool is a CoreTool decorator: it embeds the original (so all behavioral
// methods — Prompt/IsReadOnly/IsConcurrencySafe/CheckPermissions/Name — delegate)
// and overrides only the model-facing description/schema plus default injection.
type overriddenTool struct {
	actool.CoreTool
	desc   string
	schema map[string]any
}

func (o *overriddenTool) Description() string         { return o.desc }
func (o *overriddenTool) InputSchema() map[string]any { return o.schema }

func (o *overriddenTool) Call(ctx context.Context, in json.RawMessage, tc *actool.ToolContext) (actool.Result, error) {
	return o.CoreTool.Call(ctx, injectDefaults(in, o.schema), tc)
}

// injectDefaults fills scalar parameter defaults declared in the (possibly edited)
// schema into the input JSON whenever the model omitted the field or left it empty/
// null. Structure (names/types/required) is untouched — onlyDefault value are merged in.
func injectDefaults(in json.RawMessage, schema map[string]any) json.RawMessage {
	defs := scalarDefaults(schema)
	if len(defs) == 0 {
		return in
	}
	m := map[string]json.RawMessage{}
	if len(in) > 0 {
		if err := json.Unmarshal(in, &m); err != nil {
			return in // non-object input: don't touch it
		}
	}
	changed := false
	for k, dv := range defs {
		if cur, ok := m[k]; !ok || isEmptyJSON(cur) {
			m[k] = dv
			changed = true
		}
	}
	if !changed {
		return in
	}
	b, err := json.Marshal(m)
	if err != nil {
		return in
	}
	return b
}

// scalarDefaults extracts properties[k]["default"] for scalar params (string/
// integer/number/boolean). Array/object defaults are skipped: merging them is
// ambiguous and not worth the surprise.
func scalarDefaults(schema map[string]any) map[string]json.RawMessage {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return nil
	}
	out := map[string]json.RawMessage{}
	for name, raw := range props {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		dv, ok := p["default"]
		if !ok || dv == nil {
			continue
		}
		switch p["type"] {
		case "string", "integer", "number", "boolean":
			if b, err := json.Marshal(dv); err == nil {
				out[name] = b
			}
		}
	}
	return out
}

func isEmptyJSON(raw json.RawMessage) bool {
	s := string(raw)
	return s == "null" || s == `""`
}
