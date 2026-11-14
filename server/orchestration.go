package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"antifox/agent"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/traffic"
)

// jsonResult marshals v to a JSON tool result.
func jsonResult(v any) (actool.Result, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	return actool.Text(string(b)), nil
}

// This document achieves P2"Multitask Organization Toolset(docs/Run Division §2 P2). These are... host Tools——Yes.
// Visits Manager(Arbitrary assignments Store), Engine(Pause), And build task processes,That's why I live here. server Layer.
// Read-type tool to "Existing" per-task The tool is redirected to the target mission. store Run!(Build a temporary ToolSet
// and Call Response tool),And then it's exactly the same logic.;Control Class(spawn/pause)Direct Manager/Engine.
// They're like flow tools. seed In. tools Table, by agent Tie(Only to organize agent Only when I see you.).

// hostTools is the runtime host-tool provider fed to ToolAugment: traffic tools
// (gated by capture) + cross-task orchestration tools + user-defined custom tools.
// The second return is the names of custom tools flagged `deferred` (schema
// withheld, routed via SearchExtraTools/ExecuteExtraTool). Per-agent binding still
// decides who actually sees any of them.
//
//nolint:unused // used as the hostTools provider in wireAgentAugment
func (s *Server) hostTools() ([]actool.CoreTool, map[string][]string) {
	tools := append(s.m.HostTools(), s.orchestrationTools()...)
	tools = append(tools, s.platformTools()...) // Platform Operating Tool(Construction skill/Tools/MCP,Here. Auto Use)
	tools = append(tools, s.toolEnsureKaliTool())
	custom, err := s.customTools()
	if err != nil {
		log.Printf("[custom-tool] Loading failed: %v", err)
		return tools, nil
	}
	tools = append(tools, custom...)
	// deferred custom tools → name -> its bound agent keys. ToolAugment turns a
	// name into a deferred entry only for agents it's actually bound to (so we don't
	// advertise a tool the per-agent binding will drop from the callable set).
	deferred := map[string][]string{}
	rows, _ := s.m.pg.ListCustomTools()
	for _, t := range rows {
		if t.Deferred && t.Enabled {
			deferred[t.Key] = t.Agents
		}
	}
	return tools, deferred
}

// orchestrationTools returns the cross-task tool set. Bound per-agent via the
// tools table (default: no binding — opt-in for orchestration agents).
func (s *Server) orchestrationTools() []actool.CoreTool {
	return []actool.CoreTool{
		s.toolListTasks(),
		s.toolListLLMProfiles(),
		s.toolSpawnTask(),
		s.toolPauseTask(),
		s.toolGetTaskGraph(),
		s.toolListTaskFindings(),
		s.toolAddHint(),
		s.toolGetWorkerTrace(),
		s.toolListWorkerTraces(),
		s.toolSearchWorkerTraces(),
	}
}

// --- schema helpers ---

func strParam(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// parseProfileID reads an LLM profile id from a tool arg that may arrive as a JSON
// number (5) or a numeric string ("5"); returns 0 when absent/unparseable.
func parseProfileID(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		v, _ := strconv.ParseInt(strings.TrimSpace(str), 10, 64)
		return v
	}
	return 0
}

func objSchema(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		req := make([]any, len(required))
		for i, r := range required {
			req[i] = r
		}
		m["required"] = req
	}
	return m
}

func roTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		ReadOnly:   func(json.RawMessage) bool { return true },
		Concurrent: func(json.RawMessage) bool { return true },
		Permissions: func(context.Context, json.RawMessage, permission.Context) permission.Decision {
			return permission.Allowed()
		},
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

func wrTool(name, desc string, schema map[string]any, run func(context.Context, json.RawMessage) (actool.Result, error)) actool.CoreTool {
	return actool.Build(actool.Spec{
		Name: name, Description: desc, Schema: schema,
		Permissions: func(context.Context, json.RawMessage, permission.Context) permission.Decision {
			return permission.Allowed()
		},
		Run: func(ctx context.Context, in json.RawMessage, _ *actool.ToolContext) (actool.Result, error) {
			return run(ctx, in)
		},
	})
}

// delegateToTask resolves the `task_id` in the input, builds a ToolSet bound to
// that task's store, strips task_id, and calls the chosen per-task tool — so the
// cross-task read reuses the exact in-task logic against another task.
func (s *Server) delegateToTask(ctx context.Context, in json.RawMessage, pick func(*agent.ToolSet) actool.CoreTool) (actool.Result, error) {
	var head struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(in, &head)
	if strings.TrimSpace(head.TaskID) == "" {
		return actool.Errorf("task_id You have to fill it."), nil
	}
	t, ok := s.m.Task(head.TaskID)
	if !ok {
		return actool.Errorf("task does not exist: " + head.TaskID), nil
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(in, &m)
	delete(m, "task_id")
	inner, _ := json.Marshal(m)
	tsx := agent.NewToolSet(t.Store, "orchestrator")
	if s.m.Assets() != nil {
		tsx.SetAssetStore(s.m.Assets(), s.m.Assets().Companies())
	}
	tsx.SetNotify(t.Notify) // hint writes wake this task's planner (no-op for read tools)
	return pick(tsx).Call(ctx, inner, nil)
}

// --- tools ---

func (s *Server) toolListTasks() actool.CoreTool {
	return roTool("list_tasks",
		"List all tasks(id/Description/Objective/Status/Run-time/Father Job/LLM Configure),Organization agent Use it to master the whole picture, see which missions are stuck too long, which ones are used. LLM. Run-time: running=Create→Now, final.=Create→Final activities(sec). llm_profile:Tasks planner/worker The configuration name,(Activate Configuration)=Follow global activation.",
		objSchema(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			lastAct, _ := s.m.PG().LastActivityAll()
			// id -> name to resolve each task's pinned LLM profile.
			profName := map[int64]string{}
			if profs, err := s.m.pg.ListProfiles(); err == nil {
				for _, p := range profs {
					profName[p.ID] = p.Name
				}
			}
			out := make([]map[string]any, 0)
			for _, t := range s.m.List() {
				status := s.deriveTaskStatus(t)
				end := lastAct[t.ExpID]
				if live := s.engine.LastActivity(t.ID); live > end {
					end = live
				}
				dur := int64(0)
				if status == "running" {
					dur = time.Now().Unix() - t.CreatedAt
				} else if end > t.CreatedAt {
					dur = end - t.CreatedAt
				}
				row := map[string]any{"id": t.ID, "description": t.Description, "goal": t.Goal, "status": status, "run_seconds": dur}
				if t.ParentRef != "" {
					row["parent_ref"] = t.ParentRef
				}
				if t.LLMProfileID == nil {
					row["llm_profile"] = "(Activate Configuration)"
				} else if n, ok := profName[*t.LLMProfileID]; ok {
					row["llm_profile"] = n
				} else {
					row["llm_profile"] = fmt.Sprintf("#%d(Deleted)", *t.LLMProfileID)
				}
				out = append(out, row)
			}
			return jsonResult(out)
		})
}

// toolListLLMProfiles lists the available LLM profiles (name/model/active) so an
// orchestration agent can pick one for spawn_task's llm_profile. Never leaks keys.
func (s *Server) toolListLLMProfiles() actool.CoreTool {
	return roTool("list_llm_profiles",
		"List the available LLM profiles: id, name, model, format, and whether the profile is currently active. Use the id to set the llm_profile_id parameter of spawn_task and give a subtask its own LLM (for example, the use of cheap models and the use of strong models for reconnaissance). does not contain API Key. ",
		objSchema(map[string]any{}),
		func(context.Context, json.RawMessage) (actool.Result, error) {
			profs, err := s.m.pg.ListProfiles()
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(profs))
			for _, p := range profs {
				out = append(out, map[string]any{
					"id": p.ID, "name": p.Name, "model": p.Model, "format": p.Format, "is_active": p.IsDefault,
				})
			}
			return jsonResult(map[string]any{"profiles": out})
		})
}

func (s *Server) toolSpawnTask() actool.CoreTool {
	return wrTool("spawn_task",
		"Create a new submission and start the search engine. Go back. task_id. For one thing.(Like a question./A goal.)To be independent.parent_ref Optional: Fill in the parent task of the current organization id Make parent-child connections.",
		objSchema(map[string]any{
			"description":     strParam("Synchronising folder(Short title)"),
			"goal":            strParam("Mission objective(What do you want to achieve?)"),
			"parent_ref":      strParam("Optional: Parent Job id(As a parent and a son.)"),
			"llm_profile_id":  map[string]any{"type": "integer", "description": "Optional: Specify a book task planner/worker I used it. LLM Configure id(See list_llm_profiles);Leave space to inherit the parent's task and return to the global configuration"},
			"timeout_seconds": map[string]any{"type": "integer", "description": "Optional: Task-level overtime(sec). After point trigger graceful ending and entry timeout final; empty or 0 = Timeless"},
		}, "description", "goal"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Description, Goal, ParentRef string
				LLMProfileID                 json.RawMessage `json:"llm_profile_id"`
				TimeoutSeconds               int             `json:"timeout_seconds"`
			}
			_ = json.Unmarshal(in, &a)
			if strings.TrimSpace(a.Description) == "" {
				a.Description = "Unnamed Task"
			}
			if strings.TrimSpace(a.Goal) == "" {
				return actool.Errorf("goal You have to fill it."), nil
			}
			if a.TimeoutSeconds < 0 {
				a.TimeoutSeconds = 0
			}
			// LLM profile resolution: explicit id > inherit parent's pin > active(nil).
			var pin *int64
			if id := parseProfileID(a.LLMProfileID); id > 0 {
				if _, ok := s.loadProfileConfig(id); !ok {
					return actool.Errorf(fmt.Sprintf("LLM Configure #%d Not available or not set API Key", id)), nil
				}
				pin = &id
			} else if a.ParentRef != "" {
				if pt, ok := s.m.Task(a.ParentRef); ok {
					pin = pt.LLMProfileID
				}
			}
			t, err := s.m.CreateTask("", a.Description, a.Goal, "", pin, a.TimeoutSeconds)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			if a.ParentRef != "" {
				t.ParentRef = a.ParentRef
				if id, e := strconv.ParseInt(t.ID, 10, 64); e == nil {
					_ = s.m.PG().SetParentRef(id, a.ParentRef)
				}
			}
			s.seed(t, a.Description+" "+a.Goal) // seed Initial assets, feeding event driven loop
			s.createGoals(s.ctx, t, nil)        // Target breakdown(LLM;Bottom of the rule.)
			s.engine.Run(s.ctx, t)              // Start the mission's search engine.
			return actool.Text(fmt.Sprintf("task created: %s", t.ID)), nil
		})
}

func (s *Server) toolPauseTask() actool.CoreTool {
	return wrTool("pause_task", "Pause Assignment(Stop it. planner/worker Loop). ",
		objSchema(map[string]any{"task_id": strParam("Tasks to suspend id")}, "task_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				TaskID string `json:"task_id"`
			}
			_ = json.Unmarshal(in, &a)
			if _, ok := s.m.Task(a.TaskID); !ok {
				return actool.Errorf("task does not exist: " + a.TaskID), nil
			}
			s.engine.Pause(a.TaskID)
			return actool.Text("task paused: " + a.TaskID), nil
		})
}

func (s *Server) toolGetTaskGraph() actool.CoreTool {
	return roTool("get_task_graph", "Read an overview of the search for specified tasks(Same graph_overview:Assets count/frontier/Found/Overwrite etc.),Use task_id Specifies the task.",
		objSchema(map[string]any{"task_id": strParam("Tasks id")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).GraphOverviewTool)
		})
}

func (s *Server) toolListTaskFindings() actool.CoreTool {
	return roTool("list_task_findings", "Can not open message Hole(Ham flag/PoC;Every belt. id/task_id/intent_id/vulnclass/severity/Summary/Status),Use task_id Specifies the task.",
		objSchema(map[string]any{"task_id": strParam("Tasks id")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).ListFindingsTool)
		})
}

func (s *Server) toolAddHint() actool.CoreTool {
	return wrTool("add_task_hint", "Inject a strategic hint into an assigned task; the task planner reads it the next time it generates intents.\n"+
		"★ Prefer batching: put several hints into the hints array and submit them in one call. The ids array is returned in the same order and length as hints (failed entries have id=0); for a single hint, omit hints and pass the top-level text.",
		objSchema(map[string]any{
			"task_id":   strParam("Task ID"),
			"hints":     map[string]any{"type": "array", "description": "Preferred: an array of hints; each element uses the same fields as the top-level fields (text/asset_ids).", "items": map[string]any{"type": "object"}},
			"text":      strParam("[Single] The hint text."),
			"asset_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Anchored asset IDs (optional, 0/1/multiple; must belong to the task)."},
		}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).AddHintTool)
		})
}

func (s *Server) toolGetWorkerTrace() actool.CoreTool {
	return roTool("get_task_worker_trace",
		"Look at one of the specified tasks. work(Intention)Implementation process:get_task_worker_trace(task_id, intent_id) Read the summary of the steps; take another step_ids=[...] Take those steps.(Once.≤5). ",
		objSchema(map[string]any{
			"task_id":   strParam("Tasks id"),
			"intent_id": map[string]any{"type": "integer", "description": "Intention id(From the mission. work)"},
			"step_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Optional: Steps to retrieve full content id(≤5)"},
		}, "task_id", "intent_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).GetWorkerTraceTool)
		})
}

func (s *Server) toolListWorkerTraces() actool.CoreTool {
	return roTool("list_task_worker_traces", "Set out what's running on a given mission. work(Intention) + Step by step, to find out what work It's worth looking at.(Again. get_task_worker_trace). ",
		objSchema(map[string]any{"task_id": strParam("Tasks id")}, "task_id"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).ListWorkerTracesTool)
		})
}

func (s *Server) toolSearchWorkerTraces() actool.CoreTool {
	return roTool("search_task_worker_traces", "Search all by keyword in the given task work Implementation process(Return hit step summary + intent_id). ",
		objSchema(map[string]any{"task_id": strParam("Tasks id"), "q": strParam("Search keywords")}, "task_id", "q"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			return s.delegateToTask(ctx, in, (*agent.ToolSet).SearchWorkerTracesTool)
		})
}

// deriveTaskStatus mirrors listTasks' status derivation for the list_tasks tool.
func (s *Server) deriveTaskStatus(t *Task) string {
	switch {
	case isTerminalStatus(t.Status):
		return t.Status
	case t.Paused || s.engine.IsPaused(t.ID):
		return "paused"
	case s.engine.Ready() && s.engine.Started(t.ID):
		return "running"
	}
	return "created"
}

// orchestrationToolSeeds seeds the cross-task tools into the tools table so they
// are bindable per-agent (default: bound to nobody — opt-in for orchestration
// agents). First-insert only, like the traffic seeds.
func (s *Server) seedOrchestrationTools() {
	// task-op + platform tools default-bind to the built-in Auto agent (It's natural.
	// Operating Platform). SeedTool is insert-only (first write wins); the cooperative seeds are handled by seedAutoDefaultBindings.
	autoAgents, _ := json.Marshal([]string{"auto"})
	for _, t := range s.orchestrationTools() {
		schema, _ := json.Marshal(t.InputSchema())
		_ = s.m.PG().SeedTool(t.Name(), t.Description(), schema, autoAgents)
	}
	for _, t := range s.platformTools() {
		schema, _ := json.Marshal(t.InputSchema())
		_ = s.m.PG().SeedTool(t.Name(), t.Description(), schema, autoAgents)
	}
	kaliAgents, _ := json.Marshal([]string{"worker", "pentest", "auto"})
	if kt := s.toolEnsureKaliTool(); kt != nil {
		schema, _ := json.Marshal(kt.InputSchema())
		_ = s.m.PG().SeedTool(kt.Name(), kt.Description(), schema, kaliAgents)
		_ = s.m.PG().AddAgentToToolBinding("worker", []string{kt.Name()})
		_ = s.m.PG().AddAgentToToolBinding("pentest", []string{kt.Name()})
		_ = s.m.PG().AddAgentToToolBinding("auto", []string{kt.Name()})
	}
	s.seedKaliToolCatalog()
	s.refreshBuiltinToolSchemas()
	s.refreshReportFindingSchema()    // Add reproduction Parameters for the structured recurrence process → Push to old library,Otherwise... agent I can't see it.
	s.seedFindingVerificationSchema() // poe_level / evidence_quotes on report_finding, added once and edit-preserving
	s.seedAutoDefaultBindings()
	s.seedPlannerDefaultBindings()
	s.refreshToolListAssets()
	s.seedPentestDefaultBindings()
	s.seedAutoReportFindingBinding()
	s.seedMainagentReportFindingBinding()
	s.reseedAgentPrompt("mainagent", "prompt_exposure_findings_mainagent_v1", "Record exposure/Information leak class discovery + When combing, expose the confirmed.")
	s.reseedAgentPrompt("worker", "prompt_exposure_findings_worker_v1", "Exposure/Information leaks are also recorded as discovery.")
	s.reseedAgentPrompt("pentest", "prompt_exposure_findings_pentest_v1", "Exposure/Information leaks are also recorded as discovery.")
	// References AntiFox 0.3.4–0.3.6 Improved self-advocacy coverage Subsystem caliber:
	s.reseedAgentPrompt("planner", "prompt_zerointent_acceptance_v1", "0Attempted rewrite (two types of justification)+ Quantification check-ups (uncompliant prohibition of early receipt)")
	s.reseedAgentPrompt("worker", "prompt_negfact_threshold_v1", "Evidentiary threshold for negative conclusions: standard before exhaustion inferred,Don't be reckless. observed Welding direction.")
	// Local recording of the recurrence process: report_finding Fill Structure reproduction(Entry/Prefix/Order of real steps/Source.
	s.reseedAgentPrompt("worker", "prompt_reproduction_recipe_v1", "report_finding Fill structured recurrence process(Record locally,Request to be true word by word)")
	s.reseedAgentPrompt("pentest", "prompt_reproduction_recipe_pentest_v1", "report_finding Fill structured recurrence process")
	s.reseedAgentPrompt("mainagent", "prompt_reproduction_recipe_mainagent_v1", "report_finding Fill structured recurrence process")
	s.reseedAgentPrompt("planner", "prompt_planner_list_assets_v2", "Update list_assets usage guidance, forbid asset_id guessing, and remove retired asset_neighbors reference")
	s.reseedAgentPrompt("worker", "prompt_worker_list_assets_v2", "Remove retired asset_neighbors reference")
	s.reseedAgentPrompt("auto", "prompt_auto_update_skill_file_v1", "Name the real skill-editing tool, update_skill_file, instead of the nonexistent update_skill")
	s.bumpWorkerRunSecondsTo1200() // worker Single Wall Bell 600→1200s(only undefined defaults;references AntiFox 0.3.3)
	s.warnPromptToolDrift()        // last: checks prompts and bindings after every seed and reseed above
	// Notes:pentest Default tool binding does not need to be migrated——BuiltinToolSeeds At the start of the whole new thing.
	// list_assets/insert_assets/report_finding/list_findings/list_companies with
	// pentest Together. seed Okay.
}

// refreshBuiltinToolSchemas propagates code schema/description changes on the
// orchestration + platform tools into already-seeded rows ONCE per version flag —
// SeedTool is first-insert-only, so a new parameter (e.g. spawn_task's llm_profile) never
// reaches an old DB otherwise. Preserves each tool's agent binding + enabled flag.
// Bump the flag whenever these tools' schemas/descriptions change in code.
func (s *Server) refreshBuiltinToolSchemas() {
	const flag = "tool_schema_refresh_v5_english_and_scan_ingest"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	tools := append(s.orchestrationTools(), s.platformTools()...)
	if kt := s.toolEnsureKaliTool(); kt != nil {
		tools = append(tools, kt)
	}
	tools = append(tools, traffic.SeedToolMetas()...)
	for _, t := range tools {
		schema, _ := json.Marshal(t.InputSchema())
		if err := s.m.pg.RefreshToolDefaults(t.Name(), t.Description(), schema); err != nil {
			log.Printf("[tools] refresh %s schema failed: %v", t.Name(), err)
		}
	}
	for _, sd := range agent.BuiltinToolSeeds() {
		schema, _ := json.Marshal(sd.Schema)
		if err := s.m.pg.RefreshToolDefaults(sd.Key, sd.Desc, schema); err != nil {
			log.Printf("[tools] refresh %s desc failed: %v", sd.Key, err)
		}
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[tools] Refreshed built-in tool schemas and descriptions to code defaults (one-time)")
}

// seedAutoReportFindingBinding adds "auto" to report_finding's binding ONCE so
// conversation-context agents can call it without requiring an intent_id.
func (s *Server) seedAutoReportFindingBinding() {
	const flag = "auto_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if err := s.m.pg.AddAgentToToolBinding("auto", []string{"report_finding"}); err != nil {
		log.Printf("[auto] report_finding Default binding failed: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// seedPlannerDefaultBindings adds "report_finding" and "list_assets" to "planner" ONCE
// (guarded by settings flags), so existing DBs also let the planner look up assets
// and record findings without overriding a user unbind.
func (s *Server) seedPlannerDefaultBindings() {
	const flag = "planner_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v != "true" {
		if err := s.m.pg.AddAgentToToolBinding("planner", []string{"report_finding"}); err != nil {
			log.Printf("[planner] report_finding Default binding failed: %v", err)
		} else {
			_ = s.m.pg.SetSetting(flag, "true")
		}
	}
	const assetFlag = "planner_list_assets_v1"
	if v, _, _ := s.m.pg.GetSetting(assetFlag); v != "true" {
		if err := s.m.pg.AddAgentToToolBinding("planner", []string{"list_assets"}); err != nil {
			log.Printf("[planner] list_assets default binding failed: %v", err)
		} else {
			_ = s.m.pg.SetSetting(assetFlag, "true")
		}
	}
}

// seedPentestDefaultBindings guarantees "pentest" has its core asset/finding/KB
// tools bound on upgraded DBs created before pentest had full default bindings.
func (s *Server) seedPentestDefaultBindings() {
	const flag = "pentest_default_bindings_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	keys := []string{"list_assets", "insert_assets", "report_finding", "list_findings", "list_companies", "search_knowledge", "list_knowledge_categories"}
	if err := s.m.pg.AddAgentToToolBinding("pentest", keys); err != nil {
		log.Printf("[pentest] default binding failed: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// seedMainagentReportFindingBinding binds report_finding to the Lord Agent
// (mainagent) ONCE. Root-cause fix: the main agent synthesizes/confirms vulns in
// its orchestration conversation but, without this tool, could only READ findings
// (list_findings) — never record them — so its conclusions never became finding
// nodes (missing from Found Page / Explore / Report). Guarded by a flag; won't re-add
// after a deliberate user unbind.
func (s *Server) seedMainagentReportFindingBinding() {
	const flag = "mainagent_report_finding_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if err := s.m.pg.AddAgentToToolBinding("mainagent", []string{"report_finding"}); err != nil {
		log.Printf("[mainagent] report_finding Default binding failed: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[mainagent] Default bound report_finding(Lord Agent )")
}

// bumpWorkerRunSecondsTo1200 raises the worker single-run wall-clock from the old
// 600s default to 1200s ONCE (guarded), for existing DBs. Only touches the untouched
// 600 default so a user-customized value is preserved. Fresh DBs already seed 1200.
// Runs inside seedOrchestrationTools (before applyLLM builds the worker), so the live
// worker picks up the new SetRunTimeout without a restart-after-migration.
func (s *Server) bumpWorkerRunSecondsTo1200() {
	const flag = "worker_run_seconds_1200_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	if a, err := s.m.pg.GetAgentByKey("worker"); err == nil && a != nil && a.RunSecs == 600 {
		if err := s.m.pg.SetAgentRunSeconds("worker", 1200); err != nil {
			log.Printf("[worker] run_seconds 1200 Migration failed: %v", err)
			return
		}
		log.Printf("[worker] Single Wall Bell Default 600→1200s")
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// reseedAgentPrompt force-updates ONE agent's DB prompt to the current code default
// ONCE (guarded by a settings flag). DB prompt overrides the code default, so a code
// change alone won't reach existing DBs — this bumps it. Use a FRESH flag whenever the
// code default changes again. Runs before any user edit; won't clobber later user edits
// (the flag is set on first apply).
func (s *Server) reseedAgentPrompt(key, flag, note string) {
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	a, err := s.m.pg.GetAgentByKey(key)
	if err != nil || a == nil {
		return
	}
	tmpl, ok := agent.BuiltinPromptSeeds()[key]
	if !ok {
		return
	}
	cur, updatedBy, err := s.m.pg.CurrentPromptMeta(a.ID)
	if err != nil || cur == "" {
		return
	}
	// Fresh DBs just seeded this exact template as v1; appending an identical v2 is a
	// cosmetic duplicate in the version history. Skip the write when already current.
	if cur == tmpl {
		_ = s.m.pg.SetSetting(flag, "true")
		return
	}
	// Never overwrite a prompt that an operator customized (updated_by != "system").
	if updatedBy != "system" {
		_ = s.m.pg.SetSetting(flag, "true")
		log.Printf("[%s] Preserving user-customized prompt (%s); skipped system prompt update", key, updatedBy)
		return
	}
	if _, err := s.m.pg.SavePrompt(a.ID, tmpl, "System update: "+note, "system"); err != nil {
		log.Printf("[%s] prompt update failed: %v", key, err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[%s] Updated prompt (%s)", key, note)
}

// refreshToolListAssets updates ONLY list_assets on existing DBs (once per flag)
// so new query parameters (q, company_id, task_id) and planner docs reach upgraded
// databases without resetting unrelated built-in tools or user customizations.
func (s *Server) refreshToolListAssets() {
	const flag = "tool_schema_refresh_list_assets_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	var seed *agent.ToolSeed
	for _, sd := range agent.BuiltinToolSeeds() {
		if sd.Key == "list_assets" {
			cp := sd
			seed = &cp
			break
		}
	}
	if seed == nil {
		return
	}
	cur, err := s.m.pg.GetTool("list_assets")
	if err != nil {
		return
	}
	if cur == nil {
		_ = s.m.pg.SetSetting(flag, "true")
		return
	}
	isStock := cur.Description == "" ||
		cur.Description == seed.Desc ||
		strings.HasPrefix(cur.Description, "Search for asset bank") ||
		strings.HasPrefix(cur.Description, "Search the asset store") ||
		strings.HasPrefix(cur.Description, "搜索资产库")
	if cur.Description != seed.Desc && isStock {
		b, _ := json.Marshal(seed.Schema)
		var seedMap map[string]any
		_ = json.Unmarshal(b, &seedMap)
		var curMap map[string]any
		if json.Unmarshal(cur.Schema, &curMap) == nil {
			if sp, ok := seedMap["properties"].(map[string]any); ok {
				if cp, ok := curMap["properties"].(map[string]any); ok {
					for k, v := range cp {
						if cm, ok := v.(map[string]any); ok {
							if def, hasDef := cm["default"]; hasDef {
								if sm, ok := sp[k].(map[string]any); ok {
									sm["default"] = def
								}
							}
						}
					}
				}
			}
		}
		schema, _ := json.Marshal(seedMap)
		if err := s.m.pg.RefreshToolDefaults(seed.Key, seed.Desc, schema); err != nil {
			log.Printf("[tools] refresh %s failed: %v", seed.Key, err)
			return
		}
	}
	_ = s.m.pg.SetSetting(flag, "true")
}

// refreshReportFindingSchema pushes report_finding's CODE schema to the DB tools table
// ONCE (guarded). The runtime offers the DB-stored schema to the agent, and SeedTool is
// first-insert-only — so the newly-added structured `reproduction` param wouldn't reach
// existing DBs' agents without this. Won't clobber user param-default edits after the flag.
func (s *Server) refreshReportFindingSchema() {
	const flag = "tool_schema_refresh_report_finding_repro_v3" // v3: Requires standard writing per step HTTP Information(Not curl), No need to respond....Ignore
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	for _, sd := range agent.BuiltinToolSeeds() {
		if sd.Key != "report_finding" {
			continue
		}
		schema, _ := json.Marshal(sd.Schema)
		if err := s.m.pg.RefreshToolDefaults(sd.Key, sd.Desc, schema); err != nil {
			log.Printf("[tools] refresh report_finding schema failed: %v", err)
			return
		}
	}
	_ = s.m.pg.SetSetting(flag, "true")
	log.Printf("[tools] Refreshed report_finding schema(Add reproduction Structured recurrence process parameters)")
}

// seedAutoDefaultBindings adds "auto" to the task-op + platform tools' bindings
// ONCE (guarded by a settings flag), so existing DBs whose tool rows were seeded
// before Auto existed still give Auto its default toolset — without re-adding it
// after a user deliberately unbinds.
func (s *Server) seedAutoDefaultBindings() {
	const flag = "auto_default_bindings_v3" // v3: Replace old asset toolnames, add insert_assets/add_company_scope
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	keys := make([]string, 0, len(platformToolKeys)+12)
	for _, t := range s.orchestrationTools() {
		keys = append(keys, t.Name())
	}
	keys = append(keys, platformToolKeys...)
	// Asset tool:Auto The operating platform is always seen./Register assets, manage the scope of the company.
	keys = append(keys, "insert_assets", "add_company_scope", "list_assets")
	if err := s.m.pg.AddAgentToToolBinding("auto", keys); err != nil {
		log.Printf("[auto] Default binding failed: %v", err)
		return
	}
	_ = s.m.pg.SetSetting(flag, "true")
}
