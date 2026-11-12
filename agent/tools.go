package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"antifox/db"
	acperm "antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/knowledge"
)

// compactIntents distills intents to {id, summary, state, asset_ids, parents,
// yields} so the planner sees both the direction and its LINEAGE — parents (the
// upstream nodes it derived from: facts/intents/findings) and yields (the facts/
// findings it produced) — without pulling full payloads. parentsOf/yieldsOf are
// built from the exploration edges in graph_overview.
func compactIntents(ns []*db.Node, parentsOf, yieldsOf map[int64][]int64) []map[string]any {
	out := make([]map[string]any, 0, len(ns))
	for _, n := range ns {
		var p map[string]any
		_ = json.Unmarshal(n.Payload, &p)
		m := map[string]any{"id": n.ID, "summary": p["summary"], "state": n.State}
		// asset_ids is the structured "which assets this direction covers" signal for
		// dedup; fall back to legacy payload keys (target_ids plural, then target_id
		// single) so intents stored before the rename still surface their anchors.
		if tg, ok := p["asset_ids"]; ok && tg != nil {
			m["asset_ids"] = tg
		} else if tg, ok := p["target_ids"]; ok && tg != nil {
			m["asset_ids"] = tg
		} else if tg, ok := p["target_id"]; ok && tg != nil && tg != "" {
			m["asset_ids"] = []any{tg}
		}
		if ps := parentsOf[n.ID]; len(ps) > 0 {
			m["parents"] = ps // Upstream: from which nodes the intention was derived (multiple facts may produce an intention together)
		}
		if ys := yieldsOf[n.ID]; len(ys) > 0 {
			m["yields"] = ys // Downstream: What was the intention?/Found
		}
		out = append(out, m)
	}
	return out
}

// ToolSet exposes the PG-backed dual graph (asset + exploration) to an LLM agent.
// One ToolSet is created per planner/worker run; per-run signals live here.
type ToolSet struct {
	as     *db.AssetStore   // asset store (optional; nil = asset tools not available)
	cs     *db.CompanyStore // company store (optional)
	ts     *db.ExplorationStore
	worker string
	taskID int64 // PG tasks.id; 0 when unknown (tests / orchestrator cross-task reads)
	// ownerNode is the exploration node that writes attach to: assets this run
	// touches get anchored to it as lineage/provenance (NOT visibility — the asset
	// graph is global and shared). Worker = its claimed intent; planner = begin root.
	ownerNode int64
	GoalMet   bool
	Reason    string
	writes    WriteCounts
	// killWork, if set, terminates a running work by intent id (engine callback,
	// wired by the planner). nil = the kill_work tool reports unavailable.
	killWork func(intentID int64) error
	// steerWork, if set, queues a mid-run course-correction for the work running an
	// intent id (engine callback, wired by the planner): the worker injects it before
	// its next tool call and re-plans, without being killed. nil = tool unavailable.
	steerWork func(intentID int64, msg string) error
	// enrich, if set, receives async auto-completion triggers (DNS resolve for a
	// domain, HTTP probe for a site). nil = no engine enrichment.
	enrich EnrichTrigger
	// notify, if set, wakes the task's planner after a graph change that should be
	// re-planned promptly (currently: a new hint). nil = no wake (the hint is still
	// stored and read on the next round triggered by other events). debounced.
	notify func()
	// kb, if set, enables search_knowledge / list_knowledge_categories over the
	// two-layer knowledge base (global + this task). nil = tools report "not enabled".
	kb *knowledge.Service
	// resolveTraffic, if set, resolves a finding's reproduction steps to real recorded
	// traffic exchange ids AT RECORD TIME (so the finding's steps carry jump-to-traffic
	// links without view-time guessing). Input = the raw reproduction JSON; returns the
	// same shape with each step's exchange_id filled where it matched. nil = skip.
	resolveTraffic func(taskID int64, repro json.RawMessage) json.RawMessage
}

// SetNotify wires the planner-wake callback (see ToolSet.notify). Set by callers
// that hold the task handle (main-agent chat, cross-task orchestration).
func (t *ToolSet) SetNotify(fn func()) { t.notify = fn }

// EnrichTrigger is the enrichment engine seen from the tool layer (see package
// enrich). Kept as an interface here to avoid coupling agent → enrich.
type EnrichTrigger interface {
	ResolveDomain(id int64, host string)
	ProbeSite(id int64, url string)
}

// WriteCounts breaks down what a worker persisted this run, by node kind, so the
// engine can log an accurate "wrote back" summary instead of lumping assets and
// findings under "facts" (record_fact → Facts, insert_assets → Assets,
// report_finding → Findings; each element of a batch counts once).
type WriteCounts struct {
	Facts    int
	Assets   int
	Findings int
}

// Total is every node persisted this run, regardless of kind — the
// "explored but persisted nothing" signal (Total == 0).
func (w WriteCounts) Total() int { return w.Facts + w.Assets + w.Findings }

// String renders the per-kind breakdown for logs, e.g. "Facts1 Assets25 Hole0".
func (w WriteCounts) String() string {
	return fmt.Sprintf("Facts%d Assets%d Hole%d", w.Facts, w.Assets, w.Findings)
}

// Writes reports what this run wrote back, split by node kind (so the engine can
// tell "explored but persisted nothing" apart from a completed intent, and log an
// honest breakdown instead of calling assets/findings "facts").
func (t *ToolSet) Writes() WriteCounts { return t.writes }

func NewToolSet(ts *db.ExplorationStore, worker string) *ToolSet {
	t := &ToolSet{ts: ts, worker: worker}
	if ts != nil { // auto-wire the knowledge base off the store's DB handle
		t.kb = knowledge.NewService(ts.DB())
	}
	return t
}

// SetTaskID sets the PG task id on this ToolSet so that report_finding can
// dual-write to the standalone findings table (which survives task deletion).
func (t *ToolSet) SetTaskID(id int64) { t.taskID = id }

// SetTrafficResolver wires the record-time traffic linkage callback (host-provided;
// nil = no linkage — reproduction steps are stored verbatim without exchange ids).
func (t *ToolSet) SetTrafficResolver(fn func(taskID int64, repro json.RawMessage) json.RawMessage) {
	t.resolveTraffic = fn
}

// Cross-task reuse: exported accessors returning the per-task tool logic bound to
// THIS ToolSet's store. Host-side orchestration tools build a ToolSet for an
// arbitrary task, then Call these — so cross-task reads/hint reuse the exact
// same logic as the in-task tools. (readTool ignores ToolContext, so Call(...,nil)
// is safe; add_hint is a writeTool but also doesn't deref the context here.)
func (t *ToolSet) GraphOverviewTool() actool.CoreTool      { return t.graphOverview() }
func (t *ToolSet) ListFindingsTool() actool.CoreTool       { return t.listFindings() }
func (t *ToolSet) GetWorkerTraceTool() actool.CoreTool     { return t.getWorkerTrace() }
func (t *ToolSet) ListWorkerTracesTool() actool.CoreTool   { return t.listWorkerTraces() }
func (t *ToolSet) SearchWorkerTracesTool() actool.CoreTool { return t.searchAllWorkerTraces() }
func (t *ToolSet) AddHintTool() actool.CoreTool            { return t.addHint() }

// SetEnrich wires the async enrichment engine (DNS/HTTP auto-completion).
func (t *ToolSet) SetEnrich(e EnrichTrigger) { t.enrich = e }

// SetOwnerNode sets the exploration node that writes anchor to (worker: its
// intent node; planner/main: the begin root). Assets created/referenced while
// ownerNode is set are anchored to it as lineage (not visibility).
func (t *ToolSet) SetOwnerNode(id int64) { t.ownerNode = id }

// anchorOwner records a lineage edge from this run's owner node to an asset
// (no-op if unset). Provenance only — the asset graph is global and shared, so
// this no longer affects which assets a task can read.
func (t *ToolSet) anchorOwner(assetID int64) {
	if t.ownerNode > 0 && assetID > 0 {
		_ = t.ts.Anchor(t.ownerNode, assetID)
	}
}

// pid parses an id that may arrive as a JSON number or string ("" / 0 → 0).
func pid(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return v
	}
	return 0
}

// pidList parses a list of ids (number|string), dropping zeros/invalids.
func pidList(raw []json.RawMessage) []int64 {
	var out []int64
	for _, r := range raw {
		if v := pid(r); v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}
func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func intp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func idp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func readTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		ReadOnly:    func(json.RawMessage) bool { return true },
		Concurrent:  func(json.RawMessage) bool { return true },
		Permissions: func(context.Context, json.RawMessage, acperm.Context) acperm.Decision { return acperm.Allowed() },
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

func writeTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		Permissions: func(context.Context, json.RawMessage, acperm.Context) acperm.Decision { return acperm.Allowed() },
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

func jsonResult(v any) (actool.Result, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	return actool.Text(string(b)), nil
}

// --- read tools (planner + worker) ---

func (t *ToolSet) graphOverview() actool.CoreTool {
	return readTool("graph_overview",
		"(Exploration chain map) A distilled snapshot of the exploration state: asset counts, sites without endpoints, frontier, findings, and hints (strategic notes from the human or main agent that must be considered when generating intents). Call it before planning.",
		obj(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			return jsonResult(t.graphOverviewData())
		})
}

// graphOverviewData computes the distilled situational snapshot shared by the
// graph_overview tool and the planner's wake-up prompt (which pre-injects it so
// the model needn't spend a turn calling the tool — every plan round starts with
// an empty context and always needs this first).
func (t *ToolSet) graphOverviewData() map[string]any {
	{
		out := map[string]any{}
		// goals summary folded in so the planner needn't call list_goals each round.
		goals, _ := t.ts.ListByKind(db.KindGoal, 100)
		gsum := make([]map[string]any, 0, len(goals))
		for _, g := range goals {
			var p map[string]any
			_ = json.Unmarshal(g.Payload, &p)
			gsum = append(gsum, map[string]any{"id": g.ID, "state": g.State, "text": p["text"]})
		}
		out["goals"] = gsum
		// hints: Human./Lord agent Pass. add_hint (a) A strategic reminder of the wall chart;folded in so the
		// planner reads them every round when generating intents (Otherwise, you can't read.).
		hints, _ := t.ts.ListByKind(db.KindHint, 50)
		hsum := make([]map[string]any, 0, len(hints))
		for _, h := range hints {
			var p map[string]any
			_ = json.Unmarshal(h.Payload, &p)
			hsum = append(hsum, map[string]any{"id": h.ID, "state": h.State, "text": p["text"]})
		}
		out["hints"] = hsum
		// lineage from the exploration edges: an intent's parents (what it
		// derived_from — possibly several facts combined) and its yields (the
		// facts/findings it produced). factFrom maps a fact → the intent that
		// produced it. This is the relationship layer the flat lists lacked.
		edges, _ := t.ts.Edges(5000)
		parentsOf := map[int64][]int64{}
		yieldsOf := map[int64][]int64{}
		factFrom := map[int64]int64{}
		for _, e := range edges {
			switch e.Rel {
			case db.RelDerivedFrom, db.RelSpawns: // upstream: derived_from (fact/finding/intent→intent) or spawns (origin fact→goal, legacy begin→intent)
				parentsOf[e.To] = append(parentsOf[e.To], e.From)
			case db.RelYields: // intent --yields--> fact/finding
				yieldsOf[e.From] = append(yieldsOf[e.From], e.To)
				factFrom[e.To] = e.From
			}
		}
		fr, _ := t.ts.Frontier(100)
		out["open_intents"] = compactIntents(fr, parentsOf, yieldsOf)
		all, _ := t.ts.ListByKind(db.KindIntent, 300)
		var running, recentDone []*db.Node
		for _, n := range all {
			switch n.State {
			case "running":
				running = append(running, n)
			case "done", "blocked", "exhausted":
				if len(recentDone) < 15 {
					recentDone = append(recentDone, n)
				}
			}
		}
		out["running_intents"] = compactIntents(running, parentsOf, yieldsOf)
		out["recent_done_intents"] = compactIntents(recentDone, parentsOf, yieldsOf)
		out["frontier_open"] = len(fr)
		// findings (confirmed vulns) and facts (worker exploration results) are
		// now distinct node kinds. recent_facts surfaces fact summaries (esp.
		// negative results) so the planner sees them in one call; full content
		// via node_detail(id).
		vulnNodes, _ := t.ts.ListByKind(db.KindFinding, 1000)
		factNodes, _ := t.ts.ListByKind(db.KindFact, 1000) // newest first
		out["findings"] = len(vulnNodes)                   // Confirm the number of holes.
		out["facts"] = len(factNodes)                      // To explore the facts./Number of conclusions (including negative conclusions)
		recentFacts := make([]map[string]any, 0, 20)
		for _, n := range factNodes {
			if len(recentFacts) >= 20 {
				break
			}
			m := compactNode(n)
			if from := factFrom[n.ID]; from > 0 {
				m["from_intent"] = from // What was the intent of this fact?
			}
			// confidence Bring in the overview: let the planners see which conclusion is just inferred(Particularly negative conclusions
			// Don't be an iron case;evidence Longer. node_detail(id).
			var fp map[string]any
			if json.Unmarshal(n.Payload, &fp) == nil {
				if c, ok := fp["confidence"].(string); ok && c != "" {
					m["confidence"] = c
				}
			}
			recentFacts = append(recentFacts, m)
		}
		out["recent_facts"] = recentFacts // Recent facts {id, summary, from_intent, confidence?},Details/Evidence node_detail(id)
		// the original task (root) so the planner always has it, not just the
		// decomposed goals.
		if description, goal, err := t.ts.Root(); err == nil {
			out["task"] = map[string]any{"description": description, "goal": goal}
		}
		return out
	}
}

// compactNode distills any exploration node to id + summary + state, dropping the
// big detail/evidence (fetch that on demand via node_detail).
func compactNode(n *db.Node) map[string]any {
	var p map[string]any
	_ = json.Unmarshal(n.Payload, &p)
	return map[string]any{"id": n.ID, "state": n.State, "summary": p["summary"]}
}

// compactFinding is compactNode plus the vuln-specific vulnclass/severity.
func compactFinding(n *db.Node) map[string]any {
	var p map[string]any
	_ = json.Unmarshal(n.Payload, &p)
	m := map[string]any{"id": n.ID, "state": n.State, "summary": p["summary"]}
	if vc, ok := p["vulnclass"]; ok && vc != nil && vc != "" {
		m["vulnclass"] = vc
	}
	if sv, ok := p["severity"]; ok && sv != nil && sv != "" {
		m["severity"] = sv
	}
	return m
}

func (t *ToolSet) listFindings() actool.CoreTool {
	return readTool("list_findings", "List confirmed vulnerabilities (compact: id + task_id + intent_id + vulnclass + severity + summary + status). task_id is the task the vulnerability belongs to; intent_id is the intent that produced it. Only vulnerabilities are listed here, not ordinary exploration facts (use list_facts for those). Use node_detail(id) for details.",
		obj(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			f, _ := t.ts.ListByKind(db.KindFinding, 500)
			intentOf, _ := t.ts.FindingIntents() // finding id -> Make it. intent id
			taskID := t.ts.ID()                  // exploration id = the task id (the store holds one finding at a time)
			out := make([]map[string]any, 0, len(f))
			for _, n := range f {
				m := compactFinding(n)
				m["task_id"] = taskID
				if iid, ok := intentOf[n.ID]; ok {
					m["intent_id"] = iid
				}
				out = append(out, m)
			}
			return jsonResult(out)
		})
}

func (t *ToolSet) listFacts() actool.CoreTool {
	return readTool("list_facts", "List exploration facts and conclusions (compact: id + summary + status) — the conclusions workers reached for each intent, including negative conclusions such as 'port closed' or 'not injectable'. Use node_detail(id) for details. Use list_findings for vulnerabilities.",
		obj(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			f, _ := t.ts.ListByKind(db.KindFact, 500)
			out := make([]map[string]any, 0, len(f))
			for _, n := range f {
				out = append(out, compactNode(n))
			}
			return jsonResult(out)
		})
}

func (t *ToolSet) nodeDetail() actool.CoreTool {
	return readTool("node_detail", `Retrieve the full content of an exploration node by ID (fact, finding, intent, or goal), including its summary and details/evidence/PoC. Use only exploration-node IDs returned by list_facts, list_findings, or graph_overview recent_facts. For asset IDs, use list_assets or asset_neighbors instead.`,
		obj(map[string]any{"id": idp("Exploration node ID (not an asset ID)")}, "id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				ID json.RawMessage `json:"id"`
			}
			_ = json.Unmarshal(in, &a)
			id := pid(a.ID)
			if id <= 0 {
				return actool.Errorf("id Required"), nil
			}
			n, err := t.ts.GetNode(id)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			if n == nil {
				return actool.Errorf(fmt.Sprintf("No exploration node found with ID %d. If you are looking for assets, use list_assets / asset_neighbors instead (asset IDs and exploration-node IDs are different ID spaces, and an asset ID cannot be passed to node_detail).", id)), nil
			}
			return jsonResult(n) // full payload incl. detail / evidence
		})
}

// --- planner write tools ---

// intentItem is one exploration direction for the add_intent batch/single call.
type intentItem struct {
	Summary   string            `json:"summary"`
	AssetIDs  []json.RawMessage `json:"asset_ids"`
	ParentIDs []json.RawMessage `json:"parent_ids"`
	Priority  int               `json:"priority"`
}

// addOneIntent creates an intent node, links it into the upstream lineage, and returns its id.
// Constraint: an intent may only be anchored to confirmed knowledge — every parent_id must be an
// existing fact/finding node (it cannot be attached to another intent/goal/hint). A brand-new top-level direction leaves parent_ids empty and falls back to the origin fact.
// This enforces "every intent is linked to a fact node, and is discovery-driven rather than planned from nothing" at creation time.
func (t *ToolSet) addOneIntent(it intentItem) (int64, error) {
	if strings.TrimSpace(it.Summary) == "" {
		return 0, fmt.Errorf("summary Can't be empty.")
	}
	// Validation of anchorages (pre-establishment of nodes, avoidance of orphan intentions).
	parents := pidList(it.ParentIDs)
	for _, pidv := range parents {
		n, err := t.ts.GetNode(pidv)
		if err != nil || n == nil {
			return 0, fmt.Errorf("parent_id %d does not exist; parent_ids must be the IDs of existing fact or finding nodes; leave parent_ids empty for a brand-new top-level direction", pidv)
		}
		if n.Kind != db.KindFact && n.Kind != db.KindFinding {
			return 0, fmt.Errorf("parent_id %d is a %q node and cannot be used as an intent anchor: an intent may only be anchored to a confirmed fact/finding node, never to an intent, goal, or hint; leave parent_ids empty for a brand-new top-level direction", pidv, n.Kind)
		}
	}
	priority := it.Priority
	if priority == 0 {
		priority = 5
	}
	anchors := pidList(it.AssetIDs)
	payload := map[string]any{"summary": it.Summary}
	if len(anchors) > 0 {
		payload["asset_ids"] = anchors
	}
	id, err := t.ts.AddIntent(payload, priority, anchors, "planner")
	if err != nil {
		return 0, err
	}
	// upstream lineage: link each (validated) fact/finding parent → this intent, so
	// "multiple facts combine into one new intent" is expressible.
	for _, parent := range parents {
		_ = t.ts.Link(parent, db.RelDerivedFrom, id)
	}
	// a top-level intent (no explicit parent) connects to the origin fact, so every
	// intent still traces back to a fact node — at task start the only fact is the
	// origin, and the first intents derive from it.
	if len(parents) == 0 {
		if origin, _ := t.ts.OriginFactID(); origin > 0 {
			_ = t.ts.Link(origin, db.RelDerivedFrom, id)
		}
	}
	return id, nil
}

func (t *ToolSet) addIntent() actool.CoreTool {
	return writeTool("add_intent", "Create an exploration direction in the frontier and link it into the exploration chain. An intent is an open exploration direction, not a fixed type — describe in one free-form summary what to explore, verify, or exploit.\n"+
		"★ Prefer batching: put several new directions from one round into the intents array and submit them in one call (fewer round-trips than one call per direction). The ids array is returned in the same order and length as intents (failed entries have id=0; see errors for details). For a single direction, omit intents and pass the top-level summary.",
		obj(map[string]any{
			"intents":    map[string]any{"type": "array", "description": "Preferred: an array of new exploration directions, processed in order. Each element uses the same fields as the top-level fields below (summary/asset_ids/parent_ids/priority). The returned IDs match this array in length and order.", "items": map[string]any{"type": "object"}},
			"summary":    str("[Single] One sentence describing the exploration direction: what to do and why. Be clear and explicit; it does not depend on asset IDs."),
			"asset_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": `Target asset IDs returned by list_assets, not exploration-node IDs. Supply every relevant asset, interface, parameter, or host so the intent can be deduplicated and linked into the asset chain. Leave empty only for global reconnaissance with no specific target.`},
			"parent_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Upstream anchor IDs (optional, 0/1/multiple): the confirmed facts (fact) and findings (finding) this direction is synthesized from. **Only supply IDs of existing fact/finding nodes — never intent, goal, or hint IDs.** An intent must be anchored in confirmed knowledge, so it is discovery-driven rather than planned from nothing. Supply several IDs when multiple facts together produce a new intent; leave it empty for a brand-new top-level reconnaissance direction (it is anchored to the task origin fact automatically)."},
			"priority":   intp("Priority 0-10, default 5"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Intents    []intentItem `json:"intents"`
				intentItem              // Single mode: Top layer summary/asset_ids/parent_ids/priority
			}
			_ = json.Unmarshal(in, &a)
			batch := len(a.Intents) > 0
			items := a.Intents
			if !batch {
				items = []intentItem{a.intentItem}
			}

			ids := make([]int64, len(items))
			errs := map[string]string{}
			for i, it := range items {
				id, err := t.addOneIntent(it)
				if err != nil {
					errs[strconv.Itoa(i)] = err.Error()
					continue
				}
				ids[i] = id
			}

			if !batch { // Single article: Keep Back
				if e, bad := errs["0"]; bad {
					return actool.Errorf(e), nil
				}
				return actool.Text(fmt.Sprintf("intent created: %d", ids[0])), nil
			}
			out := map[string]any{"ids": ids}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return jsonResult(out)
		})
}

func (t *ToolSet) listGoals() actool.CoreTool {
	return readTool("list_goals", "List the target nodes of the task and their status (open/met),To judge whether it has been achieved.",
		obj(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			g, _ := t.ts.ListByKind(db.KindGoal, 100)
			return jsonResult(g)
		})
}

func (t *ToolSet) proveGoal() actool.CoreTool {
	return writeTool("prove_goal", "When you judge something./It proved that a certain target was reached by connecting the evidence node to the target node and marking the target. met. ",
		obj(map[string]any{
			"goal_id":     idp("Goal node ID"),
			"evidence_id": idp("ID of the finding/fact node that proves it"),
			"reason":      str("Why does this evidence satisfy the goal?"),
		}, "goal_id", "evidence_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				GoalID     json.RawMessage `json:"goal_id"`
				EvidenceID json.RawMessage `json:"evidence_id"`
				Reason     string          `json:"reason"`
			}
			_ = json.Unmarshal(in, &a)
			goal, ev := pid(a.GoalID), pid(a.EvidenceID)
			if goal == 0 || ev == 0 {
				return actool.Errorf("goal_id and evidence_id Required"), nil
			}
			_ = t.ts.Link(ev, db.RelProves, goal)
			_ = t.ts.SetNodeState(goal, "met")
			// Each mark a target met,Check if this mission has all the targets. met;If so, automatically.
			// Mission accomplished (set) GoalMet),There's no need to rely on a model to make it visible. goal_met.
			if goals, err := t.ts.ListByKind(db.KindGoal, 1000); err == nil && len(goals) > 0 {
				allMet := true
				for _, g := range goals {
					if g.State != "met" {
						allMet = false
						break
					}
				}
				if allMet {
					t.GoalMet = true
					t.Reason = fmt.Sprintf("All %d We've got everything. met(Finally by goal %d Trigger)", len(goals), goal)
					return actool.Text(fmt.Sprintf("goal %d marked met;All objectives of this mission have been achieved and the mission is automatically determined to be complete.", goal)), nil
				}
			}
			return actool.Text(fmt.Sprintf("goal %d marked met", goal)), nil
		})
}

func (t *ToolSet) goalMet() actool.CoreTool {
	return writeTool("goal_met", "[End the entire task immediately] — call this only once you have confirmed that ALL of the task's goals are genuinely met and the overall objective is accepted (note: the task as a whole is complete; achieving only one goal, one flag, or one vulnerability does not count — for that, use prove_goal to mark that goal). ⚠️ This is NOT the way to end a planning round: when there is simply no new intent to assign, or you are waiting for worker output, just end the round directly without calling this tool (producing zero intents is perfectly normal). The normal path is to prove goals one by one with prove_goal; goal_met is only a shortcut that bypasses individual proofs and closes out the whole task at once.",
		obj(map[string]any{"reason": str(`Reason for completion. It must cite evidence that the overall objective was achieved; "no new direction this round" is not a valid completion reason.`)}, "reason"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct{ Reason string }
			_ = json.Unmarshal(in, &a)
			t.GoalMet = true
			t.Reason = a.Reason
			return actool.Text("acknowledged: goal marked met"), nil
		})
}

// --- worker write tools ---

func (t *ToolSet) addFinding() actool.CoreTool {
	return writeTool("report_finding", "You MUST call this tool when you find a vulnerability or confirm an exposure risk! It records one confirmed vulnerability, or a proven exposure / information-disclosure risk (unauthorized reachable sensitive, management, or debug interfaces, exposed Swagger/configuration/backup files, sensitive data leaks, and so on — actually accessing them and obtaining response evidence counts as confirmed). In a task context, intent_id is required (the intent currently being executed); in a chat context, intent_id may be omitted.",
		obj(map[string]any{
			"vulnclass": str("Vulnerability class"),
			"severity":  str("critical|high|medium|low"),
			"summary":   str("Summary of the finding"),
			"intent_id": idp("ID of the intent that produced this finding (the current intent)"),
			"asset_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Affected asset IDs (optional, 0/1/multiple): parameters, endpoints, sites, and so on. Fill in every affected location when one vulnerability impacts several; leave it empty for pure observation."},
			"evidence":  str("Evidence / PoC text"),
			"reproduction": map[string]any{
				"type":        "object",
				"description": "Strongly recommended: the structured reproduction process — it enables manual reproduction from scratch and drives the delivered report. Write each step's request as a standard HTTP request message (not a curl/shell command) and its response as a status line plus key body content (do not omit key content with '...'). Copy everything verbatim from the requests and responses you actually executed in this run; never rewrite or fabricate them. You just performed every step, so you know them best.",
				"properties": map[string]any{
					"entry_point":   str("Key interface that triggers the vulnerability (method + URL), e.g. POST /user/validate/otp/forgetPasswordVerify"),
					"prerequisites": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Reproduction prerequisites: the required account, role, session, or preceding steps. If a test account is needed, state exactly how to register it (interface plus key request elements) so it can be reproduced from scratch."},
					"steps": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
						"request":  str(`The real request for this step in standard HTTP-message form: request line (METHOD /path?query HTTP/1.1), relevant headers (Host, Content-Type, Cookie, etc.), blank line, and body. Preserve the actual request verbatim. Do not provide a curl, Python, or shell command; convert it to the equivalent HTTP message. Only when the step genuinely has no HTTP request (for example, a local database operation or local token generation), describe it beginning with "Non-HTTP:".`),
						"response": str("The real response for this step, written as an HTTP response: status line (HTTP/1.1 200 OK) + key headers + key body excerpt, taken verbatim from the actual response. Do NOT use '...' to omit the elements that prove the problem; keep only the key fragments that locate this flow and demonstrate the vulnerability (echo, token, error message, and so on)."),
						"expected": str("Expected result for this step (status code + key fields), in one sentence"),
						"proves":   str("What does this step prove?"),
					}}, "description": "Ordered reproduction steps, from the entry point to confirmation. Write each step as a standard HTTP request message plus the key response (not curl/shell commands, and do not abbreviate the response with '...'), so the step can be linked automatically to the recorded real traffic package, enabling one-click navigation and screenshots."},
					"provenance": str("Which vulnerability this was explored through, or the intent that discovered it, plus the preceding results it depends on (the source of the attack chain)"),
				},
			},
		}, "vulnclass", "severity", "summary"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				VulnClass, Severity, Summary, Evidence string
				IntentID                               json.RawMessage   `json:"intent_id"`
				AssetIDs                               []json.RawMessage `json:"asset_ids"`
				Reproduction                           json.RawMessage   `json:"reproduction"`
			}
			_ = json.Unmarshal(in, &a)
			payload := map[string]any{"vulnclass": a.VulnClass, "severity": a.Severity, "summary": a.Summary,
				"evidence": map[string]any{"by": t.worker, "poc": a.Evidence}}
			repro := a.Reproduction
			if len(repro) > 2 { // "{}" is 2 bytes; only carry a real recipe
				// Resolve steps → real traffic exchange ids at record time (host callback),
				// so the stored finding's steps carry jump-to-traffic links up front.
				if t.resolveTraffic != nil && t.taskID > 0 {
					if r := t.resolveTraffic(t.taskID, repro); len(r) > 2 {
						repro = r
					}
				}
				payload["reproduction"] = repro
			}
			var anchors []int64
			for _, raw := range a.AssetIDs {
				if p := pid(raw); p > 0 {
					anchors = append(anchors, p)
				}
			}
			var id int64
			if t.ts != nil {
				var err error
				id, err = t.ts.AddNode(db.KindFinding, payload, 9, "confirmed", t.worker, anchors)
				if err != nil {
					return actool.Errorf(err.Error()), nil
				}
				if intent := pid(a.IntentID); intent > 0 {
					_ = t.ts.Link(intent, db.RelYields, id) // chain: intent -> finding
				}
				_, _ = t.ts.AddStandaloneFinding(t.taskID, id, a.VulnClass, a.Severity, a.Summary, a.Evidence, t.worker, anchors)
				if len(repro) > 2 { // Structural reproduction of local records(agent Fill it out when you're done.,Request to be true word by word)
					_ = t.ts.SetFindingReproduction(id, repro)
				}
			} else {
				// conversation context: no exploration store available, cannot record finding
				return actool.Errorf("report_finding Task context required (exploration store Not initialized)"), nil
			}
			t.writes.Findings++
			return actool.Text(fmt.Sprintf("finding recorded: %d", id)), nil
		})
}

// recordFact writes a general exploration RESULT/conclusion (not a vuln, not a
// new asset) into the EXPLORATION graph, chained to the intent that produced it.
// This is the home for observations and — importantly — negative results
// ("port closed", "param not injectable", "no login found"). Such conclusions
// must NOT be stuffed into the asset graph via upsert_asset.
// factItem is one fact for the record_fact batch/single call.
type factItem struct {
	Summary    string            `json:"summary"`
	Detail     string            `json:"detail"`
	Evidence   string            `json:"evidence"`   // One line of key evidence (orders)+Key Output Line) to support conclusions and facilitate ex post verification
	Confidence string            `json:"confidence"` // observed(I'll see you.| inferred(Insulated by phenomena)
	IntentID   json.RawMessage   `json:"intent_id"`
	AssetIDs   []json.RawMessage `json:"asset_ids"`
}

// recordOneFact Write one. fact Node Linked to Intentintent→yields→fact). defaultIntent Yes
// Default intent at bulk (not provided in this article) intent_id Time used.
func (t *ToolSet) recordOneFact(it factItem, defaultIntent int64) (int64, error) {
	if strings.TrimSpace(it.Summary) == "" {
		return 0, fmt.Errorf("summary Can't be empty.")
	}
	payload := map[string]any{"summary": it.Summary}
	if it.Detail != "" {
		payload["detail"] = it.Detail
	}
	if e := strings.TrimSpace(it.Evidence); e != "" {
		payload["evidence"] = e
	}
	if c := strings.TrimSpace(it.Confidence); c != "" {
		payload["confidence"] = c
	}
	// a fact is its OWN node kind (distinct from a vuln finding).
	id, err := t.ts.AddNode(db.KindFact, payload, 5, "confirmed", t.worker, pidList(it.AssetIDs))
	if err != nil {
		return 0, err
	}
	intent := pid(it.IntentID)
	if intent <= 0 {
		intent = defaultIntent
	}
	if intent > 0 {
		_ = t.ts.Link(intent, db.RelYields, id) // chain: intent -> fact
	}
	t.writes.Facts++
	return id, nil
}

func (t *ToolSet) recordFact() actool.CoreTool {
	return writeTool("record_fact", `Write an exploration fact or conclusion to the exploration graph and link it to the producing intent (intent_id). Use it for positive results such as fingerprints and enumeration, as well as negative conclusions such as "port closed", "parameter not injectable", or "no login portal found".`+"\n"+
		"⚠️ Consolidate the observations from one exploration into a single fact instead of splitting them into several: when they can be expressed as one fact, do so. summary = a one-sentence conclusion; detail = the supporting specifics (may contain several items). Example: a fingerprint intent produces one fact {summary:..Identified the tech stack and response characteristics of site X.., detail:..nginx 1.25 / Vue3 / 200 / title=.. / body_len=....}, not one fact each for the status code, fingerprint, and title. One intent usually produces one fact; over-splitting makes the graph grow without bound.\n"+
		"★ Use the facts array to write several distinct conclusions in one call (each may omit intent_id and inherit the top-level one). The ids array is returned in the same order and length as facts.\n"+
		"⚠️ Only write conclusions you actually saw in tool output; do not fill gaps from imagination. evidence and confidence keep inaccurate conclusions from polluting the graph:\n"+
		"  · evidence = the one line of key evidence supporting the conclusion (the command plus the one or two output lines that prove it). Keep it terse: details belong in detail, so do not paste large output here.\n"+
		"  · confidence = observed (seen directly in the output) or inferred (deduced from the symptom).\n"+
		"  · **Negative conclusions** (not injectable, port closed, no entry point found, etc.) especially need evidence and an honest confidence: they make the planner abandon the direction, and a wrong negative is costly. If you probed only once or the evidence is weak, mark it inferred rather than certain.",
		obj(map[string]any{
			"facts":      map[string]any{"type": "array", "description": "Use this when there are multiple distinct conclusions: an array of facts, where each element uses the same fields as the top-level fields below (summary/detail/evidence/confidence/intent_id/asset_ids). Omit intent_id to inherit the top-level intent_id. The returned IDs match this array in length and order.", "items": map[string]any{"type": "object"}},
			"summary":    str("One-sentence summary of the finding (put the details in detail)"),
			"intent_id":  idp("ID of the intent that produced this fact (the intent you received; defaults to the current one)"),
			"detail":     str("Details of this fact: include the various observations made during this exploration."),
			"evidence":   str("Key evidence: the command plus the one or two lines that support the conclusion. Keep it short and readable; put the rest in detail."),
			"confidence": str("observed (directly seen in the output) or inferred (deduced from the symptom). Be explicit, especially for negative conclusions."),
			"asset_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Related asset IDs (optional, 0/1/multiple): the assets this fact concerns."},
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Facts    []factItem `json:"facts"`
				factItem            // Single Bar Mode + Batch Default intent_id
			}
			_ = json.Unmarshal(in, &a)
			batch := len(a.Facts) > 0
			items := a.Facts
			if !batch {
				items = []factItem{a.factItem}
			}
			defaultIntent := pid(a.factItem.IntentID) // Top Level intent_id = Batch Default

			ids := make([]int64, len(items))
			errs := map[string]string{}
			for i, it := range items {
				id, err := t.recordOneFact(it, defaultIntent)
				if err != nil {
					errs[strconv.Itoa(i)] = err.Error()
					continue
				}
				ids[i] = id
			}

			if !batch { // Single article: Keep Back
				if e, bad := errs["0"]; bad {
					return actool.Errorf(e), nil
				}
				return actool.Text(fmt.Sprintf("fact recorded: %d", ids[0])), nil
			}
			out := map[string]any{"ids": ids}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return jsonResult(out)
		})
}

type hintItem struct {
	Text     string            `json:"text"`
	AssetIDs []json.RawMessage `json:"asset_ids"`
}

// addOneHint Hang one. hint Nodes(active/human)To Explore,Alignable assets,Back id.
func (t *ToolSet) addOneHint(it hintItem) (int64, error) {
	if strings.TrimSpace(it.Text) == "" {
		return 0, fmt.Errorf("text Can't be empty.")
	}
	var anchors []int64
	for _, raw := range it.AssetIDs {
		if tid := pid(raw); tid > 0 {
			anchors = append(anchors, tid)
		}
	}
	id, err := t.ts.AddNode(db.KindHint, map[string]any{"text": it.Text}, 0, "active", "human", anchors)
	if err == nil && t.notify != nil {
		t.notify() // wake the planner so the new hint is read promptly (debounced)
	}
	return id, err
}

func (t *ToolSet) addHint() actool.CoreTool {
	return writeTool("add_hint", "Attach a strategic hint from the human or main agent to the exploration graph; the planner reads it the next time it generates intents.\n"+
		"★ Prefer batching: put several hints into the hints array and submit them in one call (fewer round-trips than one call per hint). The ids array is returned in the same order and length as hints (failed entries have id=0; see errors for details). For a single hint, omit hints and pass the top-level text.",
		obj(map[string]any{
			"hints":     map[string]any{"type": "array", "description": "Preferred: an array of new hints, processed in order. Each element uses the same fields as the top-level fields below (text/asset_ids). The returned IDs match this array in length and order.", "items": map[string]any{"type": "object"}},
			"text":      str("[Single] One hint, e.g. 'Focus on digging into the authentication interface'"),
			"asset_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Anchored asset IDs (optional, 0/1/multiple)."},
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Hints    []hintItem `json:"hints"`
				hintItem            // Single mode: Top layer text/asset_ids
			}
			_ = json.Unmarshal(in, &a)
			batch := len(a.Hints) > 0
			items := a.Hints
			if !batch {
				items = []hintItem{a.hintItem}
			}

			ids := make([]int64, len(items))
			errs := map[string]string{}
			for i, it := range items {
				id, err := t.addOneHint(it)
				if err != nil {
					errs[strconv.Itoa(i)] = err.Error()
					continue
				}
				ids[i] = id
			}

			if !batch { // Single article: Keep Back
				if e, bad := errs["0"]; bad {
					return actool.Errorf(e), nil
				}
				return actool.Text(fmt.Sprintf("hint added: %d", ids[0])), nil
			}
			out := map[string]any{"ids": ids}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return jsonResult(out)
		})
}

// killWorkTool lets the planner terminate a single running work (by intent id).
func (t *ToolSet) killWorkTool() actool.CoreTool {
	return writeTool("kill_work", "Terminate a running intent (work). Use it to stop an off-track or pointless exploration; the terminated intent is marked stopped and is no longer automatically re-claimed. Check what it is doing with get_worker_output first.",
		obj(map[string]any{"intent_id": idp("ID of the intent to terminate (= the work handle)")}, "intent_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.killWork == nil {
				return actool.Errorf("kill_work Current Not Available"), nil
			}
			var a struct {
				IntentID json.RawMessage `json:"intent_id"`
			}
			_ = json.Unmarshal(in, &a)
			id := pid(a.IntentID)
			if id <= 0 {
				return actool.Errorf("intent_id Required"), nil
			}
			if err := t.killWork(id); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			return actool.Text(fmt.Sprintf("Termination signal sent to work %d", id)), nil
		})
}

// steerWorkTool lets the planner inject a mid-run course-correction into a running
// work WITHOUT killing it: the message reaches the worker before its next tool call,
// which re-plans its next step (already-gathered context is kept). For in-intent
// nudges ("Stop. X, Focus Y"); if the whole direction is wrong use kill_work + a new intent.
func (t *ToolSet) steerWorkTool() actool.CoreTool {
	return writeTool("steer_work", "Inject a correction instruction into a running intent (work) in real time without interrupting it or losing its progress: the worker receives your instruction before its next action and adjusts accordingly. Use it for in-intent corrections such as 'stop pursuing X, focus on Y'; if the whole direction is wrong, use kill_work and assign a new intent instead. Check what it is doing with get_worker_output first.",
		obj(map[string]any{
			"intent_id": idp("ID of the intent to steer (= the work handle)"),
			"message":   str("The correction instruction for the worker, making it stop and change course."),
		}, "intent_id", "message"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.steerWork == nil {
				return actool.Errorf("steer_work Current Not Available"), nil
			}
			var a struct {
				IntentID json.RawMessage `json:"intent_id"`
				Message  string          `json:"message"`
			}
			_ = json.Unmarshal(in, &a)
			id := pid(a.IntentID)
			if id <= 0 {
				return actool.Errorf("intent_id Required"), nil
			}
			if strings.TrimSpace(a.Message) == "" {
				return actool.Errorf("message Required"), nil
			}
			if err := t.steerWork(id, a.Message); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			return actool.Text(fmt.Sprintf("Correction instruction injected into work %d (takes effect on the next step)", id)), nil
		})
}

// getWorkerOutput returns a work's final (orat the time of suspension) conclusion text by intent id.
func (t *ToolSet) getWorkerOutput() actool.CoreTool {
	return readTool("get_worker_output", "Get the final output of an intent (work). A normally finished work returns its summary; a terminated (stopped) or abnormal work returns the last output produced before it was interrupted (terminated=true).",
		obj(map[string]any{"intent_id": idp("Intent ID (= the work handle)")}, "intent_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				IntentID json.RawMessage `json:"intent_id"`
			}
			_ = json.Unmarshal(in, &a)
			id := pid(a.IntentID)
			if id <= 0 {
				return actool.Errorf("intent_id Required"), nil
			}
			acts, _, err := t.ts.ActivityList(&id, 0, 1000)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			var chosen, fallback *db.Activity
			for i := range acts {
				switch acts[i].Kind {
				case "result":
					chosen = &acts[i]
					fallback = &acts[i]
				case "text":
					fallback = &acts[i]
				}
			}
			pick, terminated := chosen, false
			if pick == nil {
				pick, terminated = fallback, true
			}
			if pick == nil {
				return actool.Text("(The work No output yet)"), nil
			}
			detail, _ := t.ts.ActivityDetail(pick.ID)
			if detail == "" {
				detail = pick.Summary
			}
			return jsonResult(map[string]any{
				"intent_id": id, "worker_name": pick.Worker, "final_text": detail,
				"summary": pick.Summary, "is_error": pick.IsError, "terminated": terminated,
			})
		})
}

// traceSteps renders summary-only trace rows, re-truncating each summary to 100
// chars — the stored summary is capped at 200 for the UI transcript; the trace
// tools want it tighter since a whole work's step list is many rows.
func traceSteps(acts []db.Activity) []map[string]any {
	steps := make([]map[string]any, 0, len(acts))
	for i := range acts {
		steps = append(steps, map[string]any{
			"step_id": acts[i].ID, "kind": acts[i].Kind, "tool": acts[i].Tool,
			"is_error": acts[i].IsError, "summary": firstLine(acts[i].Summary, 100),
		})
	}
	return steps
}

// getWorkerTrace exposes a work's execution PROCESS (not just its final output):
// list step summaries, keyword-search within one work, or pull full detail of a
// few specific steps. Thinking steps are excluded everywhere.
func (t *ToolSet) getWorkerTrace() actool.CoreTool {
	return readTool("get_worker_trace",
		"Inspect the execution trace of an intent (work) — distinct from get_worker_output, which returns only the final conclusion. Three usages:\n"+
			"① Pass only intent_id → returns the summary stream for every step of that work (summary ≤ 100 characters, including step_id; this is only the action outline, not the complete output);\n"+
			"② intent_id + q → returns only the summaries of the steps matching the keyword (searched in both summaries and complete outputs; still only summaries are returned, use ③ to read the content);\n"+
			"③ intent_id + step_ids (at most 5) → returns the complete content (detail) of those steps.\n"+
			"Typical flow: use ①/② first to locate the step_id of the suspicious step, then use ③ to fetch its complete output. Thinking steps are not included.",
		obj(map[string]any{
			"intent_id": idp("Intent ID (= the work handle)"),
			"q":         str("Keyword: return only summaries, or the full output of the matched steps (optional; combine with step_ids)"),
			"step_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "The step_ids whose full content you want (from the results of ①/②, at most 5 per call)."},
			"limit":     intp("Maximum number of summary rows to return (optional)"),
		}, "intent_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				IntentID json.RawMessage   `json:"intent_id"`
				Q        string            `json:"q"`
				StepIDs  []json.RawMessage `json:"step_ids"`
				Limit    int               `json:"limit"`
			}
			_ = json.Unmarshal(in, &a)
			id := pid(a.IntentID)
			if id <= 0 {
				return actool.Errorf("intent_id Required"), nil
			}
			// ③ detail drill-down by step ids (max 5), thinking excluded by the store.
			if len(a.StepIDs) > 0 {
				var ids []int64
				for _, raw := range a.StepIDs {
					if v := pid(raw); v > 0 {
						ids = append(ids, v)
					}
				}
				if len(ids) > 5 {
					return actool.Errorf("step_ids Most at a time. 5 One (first without one) step_ids , which is then selected in batch)"), nil
				}
				acts, err := t.ts.ActivityByIDs(ids)
				if err != nil {
					return actool.Errorf(err.Error()), nil
				}
				steps := make([]map[string]any, 0, len(acts))
				for i := range acts {
					steps = append(steps, map[string]any{
						"step_id": acts[i].ID, "kind": acts[i].Kind, "tool": acts[i].Tool,
						"is_error": acts[i].IsError, "detail": acts[i].Detail,
					})
				}
				return jsonResult(map[string]any{"intent_id": id, "steps": steps})
			}
			// ①/② summary stream, optionally keyword-filtered; 100-char summaries.
			var acts []db.Activity
			var err error
			if strings.TrimSpace(a.Q) != "" {
				acts, err = t.ts.ActivityTraceSearch(&id, a.Q, a.Limit)
			} else {
				acts, err = t.ts.ActivityTrace(id, a.Limit)
			}
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			return jsonResult(map[string]any{"intent_id": id, "steps": traceSteps(acts)})
		})
}

// searchAllWorkerTraces keyword-searches EVERY work's process in this task — for
// finding what a worker saw but never wrote back as a fact. Returns only matching
// summaries (≤100 chars), each tagged with its intent_id for follow-up drill-down.
func (t *ToolSet) searchAllWorkerTraces() actool.CoreTool {
	return readTool("search_all_worker_traces",
		"Search by keyword (q) across the execution traces of every work in this task — use it to recover something a worker saw but never recorded as a fact (a path, token, error message, and so on)."+
			"Only the summaries of matching steps are returned (summary ≤ 100 characters), each with its intent_id; use get_worker_trace(intent_id, step_ids=[...]) to fetch the complete content. Thinking steps are not included.",
		obj(map[string]any{
			"q":     str("Keyword, matched against every work step summary and full output"),
			"limit": intp("Maximum number of rows to return, default 100 (optional)"),
		}, "q"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Q     string `json:"q"`
				Limit int    `json:"limit"`
			}
			_ = json.Unmarshal(in, &a)
			if strings.TrimSpace(a.Q) == "" {
				return actool.Errorf("q Required"), nil
			}
			acts, err := t.ts.ActivityTraceSearch(nil, a.Q, a.Limit)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			hits := make([]map[string]any, 0, len(acts))
			for i := range acts {
				var intent int64
				if acts[i].NodeID != nil {
					intent = *acts[i].NodeID
				}
				hits = append(hits, map[string]any{
					"intent_id": intent, "step_id": acts[i].ID, "worker": acts[i].Worker,
					"kind": acts[i].Kind, "tool": acts[i].Tool, "is_error": acts[i].IsError,
					"summary": firstLine(acts[i].Summary, 100),
				})
			}
			return jsonResult(map[string]any{"query": a.Q, "hits": hits})
		})
}

// listWorkerTraces gives a worker (which has no graph_overview and can't see the
// intent graph) a lightweight index of the works in this task — intent_id +
// one-line summary + state — so it can DISCOVER which works to inspect via
// get_worker_trace. Without this a worker only knows intent_ids that come back
// from search_all_worker_traces hits. Excludes still-open intents (not yet run →
// no process to inspect).
func (t *ToolSet) listWorkerTraces() actool.CoreTool {
	return readTool("list_worker_traces",
		"List an index of the work (intents) that have already run in this task: intent_id + one-line direction (summary) + state."+
			"You (the worker) cannot see the exploration graph. Use this to discover which work runs are worth reviewing — then use get_worker_trace(intent_id) to read their steps and get_worker_trace(intent_id, step_ids=[...]) to read the details."+
			"Only work that has already run is listed (running/done/exhausted/blocked/stopped); open intents that have not started are excluded. Note: your task boundary remains the intent you were given — reviewing other work is only for reusing observations and avoiding duplicated effort.",
		obj(map[string]any{
			"q":     str("Keyword filter on the step summary (optional)"),
			"limit": intp("Maximum number of rows to return, default 50 (optional)"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Q     string `json:"q"`
				Limit int    `json:"limit"`
			}
			_ = json.Unmarshal(in, &a)
			limit := a.Limit
			if limit <= 0 {
				limit = 50
			}
			all, err := t.ts.ListByKind(db.KindIntent, 500)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			q := strings.ToLower(strings.TrimSpace(a.Q))
			out := make([]map[string]any, 0, limit)
			for _, n := range all {
				switch n.State {
				case "running", "done", "exhausted", "blocked", "stopped": // has run → has a process
				default:
					continue
				}
				var p map[string]any
				_ = json.Unmarshal(n.Payload, &p)
				summary, _ := p["summary"].(string)
				if q != "" && !strings.Contains(strings.ToLower(summary), q) {
					continue
				}
				out = append(out, map[string]any{"intent_id": n.ID, "summary": summary, "state": n.State})
				if len(out) >= limit {
					break
				}
			}
			return jsonResult(map[string]any{"works": out})
		})
}

// PlannerTools is the read + intent-generation + goal-judgement tool set.
func (t *ToolSet) PlannerTools() []actool.CoreTool {
	return []actool.CoreTool{
		t.graphOverview(), t.listFindings(), t.listFacts(), t.nodeDetail(),
		t.getWorkerOutput(), t.getWorkerTrace(), t.searchAllWorkerTraces(), t.listGoals(), t.addIntent(), t.proveGoal(), t.goalMet(),
		t.killWorkTool(), t.steerWorkTool(),
		// report_finding:Direct registration is possible when planning a posture study, if a gap has been identified (and worker The same tool).
		t.addFinding(),
		// list_companies:View Business List + scope + Number of assets company_id / Understanding the scope of attribution).
		t.listCompanies(),
		// Knowledge base: rules for scoring when planning/Objective/The general methodology is to calibrate the intent of exploration.
		t.searchKnowledge(), t.listKnowledgeCategories(),
	}
}
