package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/harness"
	"antifox/engine/llm"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// Planner is the event-driven LLM planner (docs §4.3): each time the asset or
// exploration graph changes (debounced), it reads the exploration route, queries
// assets, judges whether the task goal is met, and emits 0..N exploration intents
// into the frontier. It is the sole intent generator.
type Planner struct {
	prov        llm.Provider
	model       string
	tx          *transcript.Store                      // raw LLM conversation persistence (nil = off)
	window      int                                    // context window in tokens (for compaction)
	maxTurns    int                                    // max agent turns per run (0 = unlimited)
	killWork    func(intentID int64) error             // engine callback to terminate a running work (nil = off)
	steerWork   func(intentID int64, msg string) error // engine callback to steer a running work mid-run (nil = off)
	proxyAddr   string                                 // recording proxy for WebFetch (empty = direct)
	proxyCACert string                                 // recording proxy's CA cert path (HTTPS verify)
	webSearch   WebSearchOpts                          // web_search tool backend selection (off by default)
	workDir     string                                 // shared work dir (surfaced in prompt as artifact-output target)
	// The planner can record findings too (report_finding), so it gets the same
	// record-time traffic linkage and verification as a worker. Before, its
	// findings were never linked to the traffic that proves them.
	trafficResolver func(taskID int64, repro json.RawMessage) json.RawMessage
	verifier        FindingVerifier

	// todos keeps ONE plan-scratchpad per task (keyed by exploration id) so the
	// planner's multi-step plan survives across wake-ups — each Plan() is a fresh
	// session, but the shared store lets it record a serial exploit chain once and
	// dispatch it step-by-step over rounds instead of front-loading it in parallel.
	todoMu sync.Mutex
	todos  map[int64]*actool.TodoStore
}

func NewPlanner(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *Planner {
	return &Planner{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, todos: map[int64]*actool.TodoStore{}}
}

// SetProxy points the planner's WebFetch at the recording proxy plus the CA cert
// it trusts to verify HTTPS through it (empty addr = direct).
func (p *Planner) SetProxy(addr, caCert string) { p.proxyAddr, p.proxyCACert = addr, caCert }

// SetTrafficResolver wires record-time linkage of finding steps to recorded traffic.
func (p *Planner) SetTrafficResolver(fn func(taskID int64, repro json.RawMessage) json.RawMessage) {
	p.trafficResolver = fn
}

// SetFindingVerifier wires machine-checked verification of recorded findings.
func (p *Planner) SetFindingVerifier(fn FindingVerifier) { p.verifier = fn }

// SetWebSearch selects the web_search backend for the planner (off by default).
func (p *Planner) SetWebSearch(o WebSearchOpts) { p.webSearch = o }

// todoFor returns the task's persistent planning todo store, creating it on first
// use. Shared across all of this task's planner wake-ups.
func (p *Planner) todoFor(expID int64) *actool.TodoStore {
	p.todoMu.Lock()
	defer p.todoMu.Unlock()
	s := p.todos[expID]
	if s == nil {
		s = actool.NewTodoStore()
		p.todos[expID] = s
	}
	return s
}

// SetKillWork wires the engine's per-work terminate callback so the planner's
// kill_work tool can stop a single running worker.
func (p *Planner) SetKillWork(fn func(intentID int64) error) { p.killWork = fn }

// SetSteerWork wires the engine's per-work steering callback so the planner's
// steer_work tool can inject a mid-run course-correction into a running worker.
func (p *Planner) SetSteerWork(fn func(intentID int64, msg string) error) { p.steerWork = fn }

// renderPlannerTodos formats the persistent planning todo for injection into the
// wake-up prompt (empty when there are no todos yet — first wake-up).
func renderPlannerTodos(items []actool.Todo) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Your plan to-do:\n")
	for _, it := range items {
		mark := map[actool.TodoStatus]string{actool.TodoPending: "☐", actool.TodoInProgress: "▶", actool.TodoCompleted: "✔"}[it.Status]
		if mark == "" {
			mark = "☐"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", mark, it.Content))
	}
	b.WriteString("Serial-chain rule: dispatch a subsequent intent ONLY after its prerequisite step is completed and the required fact already exists; use TodoWrite to update this list and mark steps satisfied by facts as completed. Do not re-dispatch pending or in_progress steps already on this list.]")
	return b.String()
}

// renderDoneIntents surfaces the intent(s) whose completion triggered this round
// so the planner looks first at their fresh yields (in theTrends below) before
// deciding whether a new direction opened up. Empty for non-completion wakes.
func renderDoneIntents(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return "\n\n[Newly completed intents triggering this round]: " + strings.Join(parts, ", ")
}

// renderGraphOverview folds the pre-computed graph_overview snapshot into the
// wake-up prompt so the planner starts each round with the full situation in
// hand — saving the round-trip it would otherwise spend calling the tool. It is
// the exact same JSON graph_overview would return; deeper detail is still one
// tool call away (node_detail / list_facts / ...).
func renderGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back to the model calling graph_overview itself
	}
	return "\n\n[Current-round state (prefetched graph_overview; identical to calling the tool yourself. Use node_detail/list_facts only when deeper detail is needed)]:\n" + string(b)
}

// plannerDefaultTmpl is the built-in EDITABLE body (Paragraph [A]) of the planner prompt,
// seeded into agent_prompts. Goal is a {{.Goal}} template var; the Intermediate output regulations
// tail is code-owned (artifactSpec) and appended by plannerSystem after rendering.
const plannerDefaultTmpl = `You are the planner for an authorized penetration-testing system. You are awakened frequently whenever the graph changes. Your responsibilities are to read the current state, determine whether goals have been met, and add exploration intents **only when a genuinely new direction is not already covered**.

Task goal: {{.Goal}}

⚠️ **Producing zero intents is a legitimate and normal result, but only for a specific valid reason—not as a default belief that assigning less work is safer.** Produce zero intents only when:
- **Already covered:** every direction you can identify is already covered by an open, running, or recent_done intent. Repeating an existing intent or recreating it with different wording is a serious error; or
- **Waiting for a dependency:** the next step depends on output from currently running work that has not yet been produced. Wait for that work to finish and for the graph update to wake you again; assigning the dependent work now would leave its worker without the required prerequisite and cause it to idle.

Conversely, if there is a new direction that no intent covers and that does not depend on running work, or if the goal remains unmet while in-scope surfaces remain untested, **do not stop merely because zero intents are common**. Assign warranted work. Never treat zero intents as a lazy default.

Decision process for every wake-up:

1. **The complete state is attached below this prompt. It is the graph_overview response, so do not call graph_overview again.** It contains: task (the **original task title and goal**, represented by the root node); asset counts; goals and their status; open/running/recent_done intents; sites_without_endpoints; findings (the count of **confirmed vulnerabilities**); facts (the count of **exploration facts/conclusions**, which are distinct from vulnerabilities); and recent_facts (recent {id, summary, confidence?} facts, including **negative conclusions** such as "port closed" or "not injectable"). Do not create intents for dead ends already established by those facts. However, a negative conclusion with confidence=inferred is only a weakly supported inference, not definitive proof; if that direction is important to the goal, one review intent may be warranted. **Exploration nodes here—goals, intents, facts, and findings—belong only to this task; they never include another task's goals. The asset graph is globally shared:** multiple tasks use the same graph, so in-scope asset counts are global rather than unique to this task. Only when deeper detail is needed, call list_facts for all facts, list_findings for all findings, or node_detail(id) for one node's complete evidence/details; lists and recent_facts contain summaries only.
   - open_intents / running_intents / recent_done_intents show which directions are covered or have already been attempted. Each intent includes **parents** (upstream facts/intents from which it was derived) and **yields** (downstream facts/findings it produced). This is the exploration graph's **lineage**; use it to understand which facts came from which direction and whether facts can be combined into a new direction. Each recent_facts entry also includes **from_intent**, identifying the producing intent.
   - sites_without_endpoints / findings indicate directions that **may** warrant exploration.
   - Only when one part requires detail, call list_assets on demand (pull mode: search with q or dsl; combine type/company_id/task_id filters; paginate with limit/offset; or fetch directly with id/ids), or call list_findings. Because the asset graph is shared globally, never fetch everything by default. Assets unrelated to this task may be present; ignore them when they are not relevant to the current task.

2. **Evaluate goals—this is your core responsibility.** graph_overview already lists goals and their statuses. When an unmet goal has been proven by a finding or fact, call prove_goal(goal_id,evidence_id,reason) to mark it met. **If the goal you mark with prove_goal is the last unfinished goal, the system automatically completes the entire task; you do not need to call goal_met.** Completion should be driven by proving each goal individually.
   ⚠️ **Verify quantitative acceptance criteria; never approve completion early.** If a goal has a measurable condition—such as "obtain N flags / N vulnerabilities," "obtain a particular privilege or sensitive data," or "reach conclusions for every asset/interface in a set"—then before prove_goal or any other completion action, **you must compare that condition against the measured graph_overview state above**: finding counts, relevant facts, whether each corresponding asset has a conclusion, and so on. If the measured result falls short, **do not** mark the goal met; assign intents to close the gap. **Never approve it early because "the main part is complete," "it is broadly achieved," or "the core objective is secured," and never use goal_met to bypass this check.** Example: if the goal requires three flags and the graph proves only one, the goal is unmet; continue assigning additional testing and do not mark it complete.

2.5. **Optional initial, extremely lightweight probing to improve understanding.** You have execution capabilities such as Bash. Their **only legitimate use** is when the graph contains almost no facts (recent_facts is essentially empty and the task has just begun) and the state alone is insufficient to describe precise initial intents. In that case, perform **a very small amount of read-only probing**, such as one or two curl requests to inspect the home page or fingerprint the target, solely to produce more precise **initial intents**.
   ⚠️ **Respect your role boundary: you are the planner, not the executor. Everything you do here exists only to generate and clearly describe intents, never to perform the work inside the planning phase.** The **only legitimate product of probing is a more precise one-sentence intent description**. It must never become vulnerability discovery, validation, or exploitation, nor endpoint/directory/parameter enumeration. Any thought such as "I can quickly test or confirm this too" crosses the boundary: that belongs to a worker during the work phase. Write it as an intent and assign it.
   **Three mandatory boundaries:**
   - **As soon as the graph contains any worker-produced fact (facts>0 or recent_facts is nonempty), you are forbidden from probing independently.** Base every judgment on existing facts. This round may only assign new intents or end. If a clue deserves deeper investigation, assign a worker intent; never curl it yourself.
   - Even at the beginning, **stop after at most three probes (≤3)**. Their sole purpose is clarifying initial intents. **Never** drift into enumerating endpoints/directories, trying IDs one by one, decoding base64 chains, repeatedly probing the same interface, or testing/validating injection, authorization bypass, or any vulnerability. All of that is worker execution and must be expressed as intent content. The moment you are "investigating deeply" instead of "quickly choosing a direction," stop and turn the remaining work into an intent.
   - Probing only helps you think; **it does not itself produce an intent**. If existing facts/state are sufficient, do not probe at all.

3. **Decide whether to add exploration directions, with restraint.** An intent is an **open exploration direction**, not a fixed type or menu item. Combine the **known facts, assets, and task goal** to decide which valuable directions remain uncovered and could advance the goal. Compare each candidate direction against open_intents, running_intents, and recent_done_intents:
   - If an open or running intent already covers the direction, **do not create another**; it is being handled.
   - If recent_done_intents already attempted the direction, even without producing output, **do not retry it unchanged**. Treat it as a **blocked route**. Reassign it only when a **materially new mechanism** exists—a new fact, asset, parameter, or clearly different technique/construction—and state in the new intent summary exactly how this attempt differs. **Different wording or "try once more in case it works" is not a new mechanism and must not be retried.** By contrast, when a critical direction was dismissed only by a weak confidence=inferred negative conclusion, one review intent to confirm or disprove it is justified because the weak evidence is a new reason for review.
   - Create intents only for **entirely new directions that currently have no intent coverage**.
   - If existing intents cover every known direction, **create no intents and end this round immediately**.
   - **Maintain route diversity and do not converge too early.** While the goal is unmet, if all existing intents cluster around the same attack route or entry class and a fundamentally different uncovered direction exists—another entry surface, asset class, or exploitation chain—prefer one divergent intent instead of adding a synonymous intent to the same route. Ideally, two or three independent routes based on different mechanisms remain active at once, such as an upload chain and an authentication-bypass chain. Concentrate resources only after a route produces evidence that advances the goal. Judge diversity by substantive mechanisms, not wording. This does not conflict with zero intents being normal: add a route only if it is genuinely different and uncovered; if existing intents already cover it, still produce zero.

3.5. **For serial exploitation chains, assign one step at a time rather than parallelizing the whole chain.** Many exploits form a **strongly dependent serial chain**, such as ① → ② → ③, where each later step requires the previous step's **actual output**. In that case:
   - **Do not** dispatch ①, ②, and ③ together as parallel intents. Downstream workers would lack prerequisites that do not yet exist and would duplicate work or idle.
   - Use TodoWrite to record the entire chain as separate pending steps, then in this round assign **only the step whose prerequisites are already satisfied**, usually the first.
   - After that step produces a fact, the next wake-up will include your todo list; assign the next step and mark satisfied steps completed.
   - Do not split the same action into two steps, such as "confirm the trigger" and "trigger the trigger." Use multiple parallel intents only for truly **independent exploration dimensions**, such as enumerating unrelated endpoints simultaneously.

4. Call add_intent **once** to batch-submit the new directions selected in step 3. Pass an intents array containing at most the four highest-value directions; do not make one tool call per intent.
   - summary: Describe the direction in one natural-language sentence stating what to do and why. **Do not force it into a fixed category.** Make the direction explicit because deduplication primarily compares it against summaries of existing intents.
   - asset_ids: IDs of the assets this direction will **test or attack**, returned by list_assets. **Never guess or invent asset IDs or pass exploration-node IDs.** Look them up with list_assets first; if the direction targets specific known assets, include their IDs so the intent links cleanly and avoids duplicates. Leave empty when no matching asset record exists or for global reconnaissance.
   - parent_ids: Optional IDs of the **upstream nodes** from which this direction was derived; zero, one, or many are allowed. If several facts combine to produce a new intent, include all their IDs. Also include the ID when deriving it from an upstream intent or finding. Leave empty for a wholly new top-level direction.

Prefer creating nothing over creating duplicates or filler. Be concise, restrained, and efficient.`

func plannerSystem(goal, workDir string) string {
	body := renderSystem("planner", plannerDefaultTmpl, PlannerVars{Goal: goal, Now: time.Now().Format("2006-01-02 15:04:05 MST")})
	return withEnglish(body + untrustedDataTail(body) + artifactSpec(workDir))
}

// Plan runs one planning round. emit, if non-nil, receives the planner's execution
// steps (so users can see how it reads the situation and judges goals — the
// planner is the intent generator and was previously a black box). Returns whether
// the planner judged the goal met.
// doneIntents carries the ids of the intent(s) whose completion triggered this
// round (may be several — the engine debounces a burst; empty for non-completion
// wakes). They are surfaced in the prompt so the planner focuses on the fresh
// yields of what just finished.
// hooks gates every tool call the planner makes (the task's guard). It has shell
// and file tools, so running it ungated left a hole in the safety boundary.
// nil runs ungated, for tests only.
func (p *Planner) Plan(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal string, doneIntents []int64, hooks harness.HookRunner, emit func(db.Activity)) (met bool, reason string, err error) {
	tsx := NewToolSet(ts, "planner")
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.killWork = p.killWork   // enable kill_work tool (nil = unavailable)
	tsx.steerWork = p.steerWork // enable steer_work tool (nil = unavailable)
	tsx.SetTrafficResolver(p.trafficResolver)
	tsx.SetFindingVerifier(p.verifier)
	if origin, _ := ts.OriginFactID(); origin > 0 {
		tsx.SetOwnerNode(origin) // planner-side anchors default to the task root (origin fact)
	}
	// Field tools + Basic Default ToolsetsRead/Write/Edit/MultiEdit/LS/Glob/Grep/Bash)
	base := append(tsx.PlannerTools(), actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "planner", base)
	defer cleanup()
	// Critical postures (intent just completed) + Full preset) in system prompt,Not... user Input:
	// system Duan will never be. compaction Condensed and guaranteed to be present in the long round; fixed inside, ready to eat.
	// system Cache. It fell after the order.deferred Before the toolbardeferredSystem The guarantee is at the end.
	situational := renderDoneIntents(doneIntents) + renderGraphOverview(tsx.graphOverviewData())
	// Task-level deadline / final mode (injected via ctx, see taskclock.go). The final round timed out.
	// planner End words as direct injection system,Let it be only a final target determination, not a new intent.
	tc := taskClockFrom(ctx)
	if tc.Final {
		situational += "\n\n[Task deadline reached (special instructions for this final round, overriding the general planning process above)]:\n" + resolveTaskTimeoutWrapup("planner")
	}
	system, boundary := deferredSystem(plannerSystem(goal, p.workDir)+situational, def)
	// the planner has no wall-clock budget of its own; the deadline comes from the context. With MaxDuration it uses the remaining time, so the planning loop keeps running
	// Finish the task at point.(Because it's time out.→Task timeout word,Step→per-run Word).
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, 0)
	settle := wrapupSettlement("planner", nil)
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("planner", nil, clamped)
	}
	opts := agentcore.Options{
		Provider:        p.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // Walking records agent marks;loading agent CA Authentication MITM Resigned HTTPS Certificate
		WebFetchProxy:   p.proxyAddr,
		WebFetchCACert:  p.proxyCACert,
		// Network Search(Optional). ddgs No need. key;brave-free Yes BraveKey;tavily Yes TavilyKey.
		// WebSearchProxy It's an independent export agent.(http/https/socks5),With the traffic recorded MITM Agent is irrelevant; empty is directly connected.
		EnableWebSearch:    p.webSearch.Enabled,
		WebSearchBackend:   p.webSearch.Backend,
		BraveSearchAPIKey:  p.webSearch.BraveKey,
		TavilySearchAPIKey: p.webSearch.TavilyKey,
		WebSearchProxy:     p.webSearch.Proxy,
		BashEnv:            proxyEnv(p.proxyAddr, p.proxyCACert), // Bash Sub-order default proxy+Trust CA
		MaxTurns:           p.maxTurns,                           // 0 = unlimited (configurable in agent management)
		MaxDuration:        maxDur,                               // 0 = unlimited; otherwise the remaining time until the deadline
		Compaction:         compactionConfig(p.window),
		// Oversized tool output spills to a file the planner can Read, so the
		// harness-wide output cap never drops data (same directory as workers).
		ToolOutputDir: filepath.Join(p.workDir, "cmd-output"),
		// Plan-to-dos for cross-aware sharing: keep the chain between multiple roundssession Is new,store No. No.
		Todos: p.todoFor(ts.ID()),
		// The budget for the strike.→ SDK End of the run.:Putting down what we've come up with.(It's a pie. add_intent,
		// I can prove it. prove_goal, Serial chain TodoWrite),Not stop planning.——planner And then they'll wake up again and again.
		// clamped(Tasked deadline Cracker.)Time to Use PromptByReason(See wrapupSettlementForTask).
		Settlement: settle,
	}
	if hooks != nil {
		opts.Hooks = hooks
	}
	if p.tx != nil { // persist raw LLM conversation; one accumulating file per task's planner
		opts.Transcript = p.tx
		opts.SessionID = fmt.Sprintf("exp%d-planner", ts.ID())
	}
	// (Intentions just completed) + It's spelled on it. system prompt(Anti-compression, in-wheel permanent). Here.
	// the user enters commands only, and the cross-wakeup todo list is the model's own plan, so the user does not have to intervene
	input := "The graph has changed. Plan the next step from the state in the system prompt and evaluate the goals. **If there is no genuinely new uncovered direction, end this planning round directly; producing zero intents is normal, especially while waiting for worker output. Ending this round does not mean calling goal_met. goal_met immediately completes the entire task and is only valid after every goal is truly achieved. Otherwise, call prove_goal for an individually proven goal, or take no completion action.**" +
		renderPlannerTodos(opts.Todos.List())
	// the deadline is stricter once it has been reached; the context carries the soft budget and the grace period, and the turn ends when the soft budget is exceeded
	runCtx := ctx
	if maxDur > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, maxDur+settleHardGrace)
		defer cancel()
	}
	_, _, err = captureRun(runCtx, opts, input,
		func(r db.Activity) {
			if emit != nil {
				r.Worker = "planner" // planner activity has no intent_id (it generates them)
				emit(r)
			}
		})
	return tsx.GoalMet, tsx.Reason, err
}
