package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/harness"
	"antifox/engine/llm"
	"antifox/engine/memory"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// Worker is an LLM work agent (docs §4.4): it claims ONE intent, completes it
// with real tools (Bash: kali tooling through the recording proxy), writes the
// FACTS it found back into the graph, and stops. It does NOT generate new
// directions (that is the planner's job) and does NOT keep exploring toward the
// goal on its own. Multiple workers run concurrently as goroutines.
// WebSearchOpts is the web-search backend selection the server pushes into each
// agent (planner/worker/main). Enabled=false leaves the web_search tool off.
// Backend is "ddgs" (no key), "brave-free" (BraveKey required), or "tavily"
// (TavilyKey required). It maps directly onto agentcore.Options.
// Proxy is a dedicated egress proxy for the search request (http/https/socks5),
// independent of the traffic-recording MITM proxy — set it when the search endpoint
// is only reachable via a VPN/SOCKS proxy. Empty = direct.
type WebSearchOpts struct {
	Enabled   bool
	Backend   string
	BraveKey  string
	TavilyKey string
	Proxy     string
}

type Worker struct {
	prov        llm.Provider
	model       string
	workDir     string
	proxyAddr   string
	proxyCACert string            // recording proxy's CA cert path (for WebFetch HTTPS verify)
	webSearch   WebSearchOpts     // web_search tool backend selection (off by default)
	mem         *memory.Store     // cross-engagement tradecraft memory (G4)
	tx          *transcript.Store // raw LLM conversation persistence (nil = off)
	window      int               // context window in tokens (for compaction)
	maxTurns    int               // max agent turns per run (0 = unlimited)
	// runTimeout is the wall-clock budget for the main exploration of one intent
	// (0 = unlimited). When it fires, the run is cut and a settlement round is
	// forced so already-identified facts get written back instead of being lost.
	runTimeout time.Duration
	// extraTools are host-provided tools (e.g. traffic query, oast) appended to
	// the worker's graph write-back tools.
	extraTools []actool.CoreTool
	// trafficResolver, if set, resolves a finding's reproduction steps to real
	// recorded traffic exchange ids at record time (host callback). nil = no linkage.
	trafficResolver func(taskID int64, repro json.RawMessage) json.RawMessage
}

// SetRunTimeout configures the per-intent wall-clock budget for the main
// exploration (0 = unlimited). When it fires, the SDK settlement phase still runs
// so facts are never lost to a timeout. Safe to call before Execute.
func (w *Worker) SetRunTimeout(run time.Duration) {
	w.runTimeout = run
}

// settleHardGrace is how far past the soft wall-clock budget (runTimeout) the hard
// ctx deadline sits — a backstop for a hung turn; the soft budget handles the
// normal case at the turn boundary.
const settleHardGrace = 90 * time.Second

// settleWrapUpPrompt is injected by the SDK settlement phase when a worker hits its
// turn/time budget: stop probing, write back what was found, then end with a
// plain-text one-liner (which becomes this run's displayed result).
const settleWrapUpPrompt = "You're about to be terminated for budget depletion. Do not run any more commands/Detection. In order:(1) I'll take it from there, and I'll take it from there. Write Back——Use of new assets insert_assets, Exploration findings/For the facts. record_fact, Confirm bugs. report_finding;(2) **One last sentence in plain text.**Summarize what you have done and what key conclusions you have reached (this sentence will be presented as a result of this operation and must be output)."

// SetMemory enables cross-engagement tradecraft memory (RecallMemory/RecordMemory
// tools + auto-injection of relevant memories, docs §8 G4).
func (w *Worker) SetMemory(m *memory.Store) { w.mem = m }

func NewWorker(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int, extra ...actool.CoreTool) *Worker {
	return &Worker{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, extraTools: extra}
}

// SetProxy configures the recording proxy address that workers route target
// traffic through, plus the CA cert path WebFetch trusts to verify HTTPS through
// that MITM proxy. Empty addr disables the hint.
func (w *Worker) SetProxy(addr, caCert string) { w.proxyAddr, w.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for this worker (off by default).
func (w *Worker) SetWebSearch(o WebSearchOpts) { w.webSearch = o }

// SetTrafficResolver wires the record-time traffic linkage callback (host-provided).
func (w *Worker) SetTrafficResolver(fn func(taskID int64, repro json.RawMessage) json.RawMessage) {
	w.trafficResolver = fn
}

// proxyEnv builds the Bash-subprocess env that routes child-command HTTP through
// the recording proxy and makes the common toolchain trust its MITM CA — so tools
// need no manual -x/--proxy/-k. Each ecosystem reads a different CA var (verified
// empirically): SSL_CERT_FILE→curl/urllib/Go/openssl, REQUESTS_CA_BUNDLE→python
// requests (it ignores SSL_CERT_FILE), CURL_CA_BUNDLE→curl, GIT_SSL_CAINFO→git,
// NODE_EXTRA_CA_CERTS→node; NODE_USE_ENV_PROXY makes Node 24+ honor the proxy vars.
// Empty proxyAddr → nil (direct, unchanged env).
func proxyEnv(proxyAddr, caCert string) []string {
	if proxyAddr == "" {
		return nil
	}
	env := []string{
		"HTTP_PROXY=" + proxyAddr, "HTTPS_PROXY=" + proxyAddr,
		"http_proxy=" + proxyAddr, "https_proxy=" + proxyAddr,
		"NODE_USE_ENV_PROXY=1", // Node 24+: honor HTTP(S)_PROXY in built-in fetch/http
	}
	if caCert != "" {
		env = append(env,
			"SSL_CERT_FILE="+caCert,
			"CURL_CA_BUNDLE="+caCert,
			"REQUESTS_CA_BUNDLE="+caCert,
			"GIT_SSL_CAINFO="+caCert,
			"NODE_EXTRA_CA_CERTS="+caCert,
		)
	}
	return env
}

// workerDefaultTmpl is the built-in EDITABLE body (Paragraph [A]) of the worker system
// prompt, seeded into agent_prompts. The trafficTool block and the Intermediate output regulations
// are NOT here — they are code-owned and appended by workerSystem after rendering
// (Paragraph [B]/[C]), so editing the DB body can never drop them.
const workerDefaultTmpl = `You are an executor (work agent) in an authorized penetration-testing system. You receive **one intent**—a one-sentence exploration direction. Your sole responsibility is to **complete that one intent, write discovered facts back to the knowledge graph, and then stop and return**.

Mandatory rules:
1. **Perform only this one intent.** Its boundary is absolute. If the intent is fingerprinting, perform only fingerprint identification; do not also enumerate endpoints, brute-force directories, inspect JavaScript for APIs, or test vulnerabilities. Those are separate intents for the planner to assign to other workers.
2. **You do not choose new exploration directions.** When you discover a promising lead, do not pursue it beyond this intent. Write it back to the graph; the planner will read it and generate a new intent. Generating exploration directions is the planner's responsibility, not yours.
3. **Exhaust reasonable methods before returning; do not give up at the first obstacle.** An intent is complete only after you have genuinely explored its direction. One blocked payload, one 404 endpoint, or one injection attempt without reflection does not prove the route is impossible. Try reasonable alternatives within this intent—different encoding, method, parameter, or path—before concluding. **The boundary does not change:** exhaust only techniques within this intent. Never drift into a different intent, such as enumerating unrelated endpoints or testing another vulnerability. Once you have thoroughly explored the direction or confirmed it is blocked, write back immediately and return. Do not continue merely because the task's overall objective is unmet, and do not waste steps on an exhausted direction.
4. **Write back as you discover results, and write each result to the correct graph.** Save each result immediately rather than accumulating everything until the end; otherwise a turn limit may discard it all. A result exists only after it is written to the graph. Keep the two graphs distinct:
   - **New assets/resources** → use insert_assets for the **asset graph**: register the asset itself, such as an endpoint, parameter, technology fingerprint, service, credential, or subdomain. Put structured attributes on the asset: status code/title/body length/content_type for a site or endpoint belong in props.http; a technology stack is a type=tech node. Batch multiple assets through insert_assets.assets.
   - **Exploration conclusions/facts** → use record_fact for the **exploration graph**, passing intent_id. This includes positive conclusions such as fingerprints/enumeration and negative conclusions such as "port closed," "this parameter is not injectable," or "no login entry found." **Combine multiple observations from one exploration into one fact:** put a one-sentence conclusion in summary and related detail—technology, status codes, response characteristics, and so on—in detail. Do not create one fact per property; one intent usually produces one fact, or a facts array only when several genuinely distinct conclusions exist. **Write only the delta:** before recording, inspect recent_facts in the global exploration state above. Record only conclusions newly obtained in this run, never an existing fact reworded. If your observation merely confirms an existing fact and adds nothing, do not create another.
   - **Confirmed vulnerabilities** → use report_finding for the **exploration graph**, include the PoC, and pass intent_id.
   - **Exposure/information disclosure is also a finding, not merely an asset.** If you actually access a sensitive surface without authorization and receive evidence showing exposure—such as a management/debug interface, Swagger/API documentation, exposed configuration/backup/source, or a sensitive-data endpoint—that is a **confirmed** finding: reachability plus a real response proves it was triggered. Record it with report_finding. Assign severity by sensitivity: ordinary information disclosure=low; unauthorized sensitive/management interface=medium; direct takeover or exposed credentials/keys=high. Merely discovering that an interface exists is an asset for insert_assets; proving that it is accessible without authorization and exposes content that should be protected is a finding for report_finding. Do not leave the latter only as an asset or fact. If the primary exploitation objective fails, these confirmed exposures are still findings you must deliver.

Available tools:
- insert_assets: Register new assets in the asset graph. **Supply only raw information; code computes keys and parent/child relationships.** For a new interface, supply the complete url and method; code automatically creates domain → site → endpoint nodes, extracts URL query parameters, and accepts body/header parameters in params. For a fingerprint, use type=tech, name=<technology>, on_url=<site URL>, and props={version,category}. Batch multiple assets in assets. Store attributes in the asset's props; do not store exploration conclusions here.
- record_fact: Write an exploration **fact/conclusion** to the exploration graph and link it to this intent by passing intent_id. Use it for positive/negative conclusions, observations, and judgments. **Combine multiple observations from one exploration into one fact** with a one-sentence summary and details in detail; do not create one fact per attribute. Use facts only for genuinely distinct conclusions. **Record only what you actually observed:** provide evidence as one concise line containing the command/request and key output, with longer details in detail, and set confidence to observed for direct observation or inferred for an inference. **Evidence threshold for negative conclusions:** before concluding that something is not injectable, a port is closed, no login entry exists, or another conclusion that may cause the planner to abandon an entire route, exhaust reasonable methods within this intent—alternative encodings, parameters, paths, and methods. If reasonable methods remain or the evidence only suggests likelihood, set confidence=inferred. **Prefer inferred so the planner can review rather than using a careless observed negative to permanently close a route.** This is especially important early in a task, when one false observed negative can misdirect the whole assessment and be difficult to correct.
- report_finding: Record a confirmed vulnerability, including its PoC and intent_id for the assigned intent. **Use it only when this run actually triggered the issue and obtained reproducible evidence such as requests/responses or command output.** Never report a confirmed vulnerability based only on a version/fingerprint matching a CVE, because a parameter looks injectable, or from an external vulnerability database, changelog, or code diff. **Do not substitute CVE lookup or patch-version comparison for actual triggering.** If an issue is plausible but cannot be triggered, use record_fact with confidence=inferred, explaining the suspicion and why triggering failed, so the planner can assign follow-up work.
  **You must fill the structured reproduction object.** You have just performed the test and are the person best able to preserve each exact step: entry_point (the method and URL of the key interface that triggers the issue); prerequisites (required account/role/previous steps, and if a test account is needed, exactly how to register it, including the interface and key request details); steps (ordered steps where request is the exact request you sent, verbatim and never rewritten or invented; response is the exact real response including status, key headers, and the minimum verbatim body excerpt proving the issue; expected is a one-sentence expected result/conclusion; proves states what the step demonstrates); and provenance (the attack-chain source or dependencies). Preserve real requests and responses wherever possible. This reproduction data directly determines whether a human can reproduce the report finding, so complete it carefully.
- list_assets (asset lookup, not exploration nodes) / asset_neighbors / list_facts (exploration facts) / list_findings (vulnerabilities) / node_detail (exploration-node ID, not asset ID): Query context only as needed.
- search_all_worker_traces / list_worker_traces / get_worker_trace: Use these **only when necessary; most context is already in this session** to reuse information from another work run that appeared in its execution but was not saved as a fact. You cannot browse the exploration graph through these tools, but can search_all_worker_traces(q) across work histories (results include intent_id), list_worker_traces to see completed work, get_worker_trace(intent_id) for a step summary, and get_worker_trace(intent_id, step_ids=[...]) for full content of selected steps, at most five per call. **Use these only to reuse observations and avoid duplicated work; they do not broaden your task boundary. You must still perform only this intent.**

Operate only within the authorized scope. After completing this intent, summarize in one sentence what you did and which facts you wrote back. Be practical, restrained, and focused on this one intent.`

// workerTrafficBlock is Paragraph [B]: the traffic-tool note, code-injected only when
// traffic capture is on (proxyAddr set). Not stored, not editable.
func workerTrafficBlock(proxyAddr string) string {
	if proxyAddr == "" {
		return ""
	}
	return `

**Traffic tools:**
- traffic_search / traffic_get: The recording proxy stores all target traffic under data/traffic/<host>/<request path>/. Before requesting a resource again, search recorded traffic first and avoid repeating curl against the same URL. traffic_search requires host and by default returns only three lightweight index records (id/method/url/status/resp_len), without response bodies. Increase limit explicitly when needed; retrieve an exact recorded message with traffic_get(id).`
}

// artifactSpec is Paragraph [C]: the code-owned, non-editable tail appended to every
// pentest agent's prompt — intermediate artifacts must land in the shared work
// dir, never /tmp. Guaranteed present regardless of how the DB body is edited.
func artifactSpec(workDir string) string {
	return `

**Intermediate-artifact policy:** Write every script, payload, captured response body, temporary dataset, and other intermediate artifact to the shared working directory ` + workDir + `. This is the CWD shared by all agents; relative paths resolve here, and this absolute path may also be used. **Do not write to /tmp or any other absolute path.**`
}

// workerArtifactSubdir is the worker-only addendum to artifactSpec: put a run's
// artifacts under an i<intentID>/ subdir to avoid concurrent name collisions.
const workerArtifactSubdir = "To prevent filename collisions with other work runs, place this run’s artifacts under i<intentID>/ (for example, i123/exploit.py)."

func workerSystem(proxyAddr, workDir string) string {
	body := renderSystem("worker", workerDefaultTmpl, WorkerVars{ProxyAddr: proxyAddr})
	return body + workerTrafficBlock(proxyAddr) + artifactSpec(workDir) + workerArtifactSubdir
}

// renderIntentTask formats the claimed intent for the worker's SYSTEM prompt: the
// intent is the worker's whole job, so — like the planner's situational block — it
// belongs in system, where compaction can never drop it during a long run.
func renderIntentTask(intent *db.Node) string {
	return fmt.Sprintf(`

[Your assigned intent (the only task for this run: perform only this intent, produce facts, and stop when complete)]:
%s
Intent ID: %d (pass this to record_fact / report_finding)`, string(intent.Payload), intent.ID)
}

// renderWorkerGraphOverview folds the global situational snapshot into the worker's
// SYSTEM prompt for AWARENESS ONLY. The framing is deliberately strong: the overview
// must NOT widen the worker's job — it still does only its assigned intent. Its sole
// purpose is letting the worker read context (existing facts/assets/hints)
// so it avoids redundant work and doesn't re-derive what others already found.
func renderWorkerGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back silently: the worker just won't have the global context
	}
	return "\n\n[Global exploration (only for you, not your list of tasks):\n" +
		"An overview of the current exploration of the entire mandate follows. For you.**Unique purpose**It is to inform you of global dynamics, to repeat what has been concluded and to avoid repeating what others have done.\n" +
		"**It will never widen your boundaries of duty.**:You're still only doing the one intention you've received. There's something else here. open Intention / Uncovered Point / Other directions, too.**Don't do it yourself.**——Those are other things. worker The thing is, it's being moved by the planners. If you find any new information about it, write it in. fact Let the planners know not to chase themselves.\n" +
		string(b)
}

// Execute runs one intent. hooks (the per-task Guard) gates every tool call; may
// be nil. emit, if non-nil, receives one ActivityRecord per execution step.
// Returns the terminal reason (so the engine can distinguish completed vs
// max_turns) and a per-kind breakdown of what was written back (so an intent that
// explored but persisted nothing isn't mistaken for done, and the engine can log
// facts/assets/findings separately instead of lumping them under "facts").
func (w *Worker) Execute(ctx context.Context, name string, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, intent *db.Node, hooks harness.HookRunner, emit func(db.Activity), enr EnrichTrigger) (harness.TerminalReason, WriteCounts, error) {
	tsx := NewToolSet(ts, name)
	tsx.SetTaskID(taskID)
	tsx.SetTrafficResolver(w.trafficResolver)
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetOwnerNode(intent.ID) // assets this worker discovers anchor to its intent → visible to the task
	tsx.SetEnrich(enr)          // async DNS/HTTP auto-completion for assets this worker writes
	// base = built-in worker tools ∪ host tools (traffic) ∪ default tools (incl. Bash);
	// then augment with the agent's visible skills/MCP. During the SDK settlement
	// phase, Bash is hidden via Settlement.DisabledTools (no local gating needed).
	base := append(tsx.WorkerTools(), w.extraTools...)
	base = append(base, actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "worker", base)
	defer cleanup()

	// The intention is... worker All missions, put them in. system prompt(Fight compaction, It's a whole round.
	// user Input——and planner Put the key postures in. system Unanimously. After the instructions,deferred Block before.
	// Global situation(graphOverviewData)Also, but only for the greater good; intended to be at the last and most visible.
	overview := renderWorkerGraphOverview(tsx.graphOverviewData())
	system, boundary := deferredSystem(workerSystem(w.proxyAddr, w.workDir)+overview+renderIntentTask(intent), def)
	// Task-level deadline (injected via ctx). The work run has its own wall-clock budget, and the run is closed when that budget is exhausted (see taskclock.go).
	tc := taskClockFrom(ctx)
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, w.runTimeout)
	settle := wrapupSettlement("worker", []string{"Bash"})
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("worker", []string{"Bash"}, clamped)
	}
	opts := agentcore.Options{
		Provider:        w.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		// WebFetch Go to the record agent. HTTP and curl Same mark;loading agent CA Make way. MITM
		// Resigned HTTPS Could not close temporary folder: %sproxy Empty and straight.
		EnableWebFetch: true,
		WebFetchProxy:  w.proxyAddr,
		WebFetchCACert: w.proxyCACert,
		// Network Search(Optional). ddgs No need. key;brave-free Yes BraveKey;tavily Yes TavilyKey.
		// WebSearchProxy is an independent egress proxy (http/https/socks5), unrelated to the traffic-recording MITM proxy; empty means a direct connection.
		EnableWebSearch:    w.webSearch.Enabled,
		WebSearchBackend:   w.webSearch.Backend,
		BraveSearchAPIKey:  w.webSearch.BraveKey,
		TavilySearchAPIKey: w.webSearch.TavilyKey,
		WebSearchProxy:     w.webSearch.Proxy,
		// Bash Sub-order. HTTP Default Walk Record Agent + Trust it. CA(Tools not needed -x/-k).
		BashEnv:    proxyEnv(w.proxyAddr, w.proxyCACert),
		WorkingDir: w.workDir,
		MaxTurns:   w.maxTurns, // 0 = unlimited (configurable in agent management)
		// Wall clock budget,Round border,Don't interrupt.;0 = Unlimited. Task level deadline The clock's coming. min(Self-budget,
		// remaining until the deadline), the worker run ends when the task ends (see taskclock.go).
		MaxDuration: maxDur,
		// Hit budget(Round OR Length)→ SDK Round and round.(Hide Bash),Write back the identified,Avoid rotting tails.
		// clamped(Tasked deadline Cracker.)Time PromptByReason:Because it's time out.=Mission to Point→Task timeout word,
		// Step=We'll run out of steps in the trap window.→Fall Back per-run Words. Not clamped Keep it pure. per-run.
		Settlement: settle,
		// large tool output spills to cmd-output/ with a head + pointer (SDK tool.Capture);
		// full output preserved on disk. Cut cap SDK Default(30000 Character).
		ToolOutputDir: filepath.Join(w.workDir, "cmd-output"),
		Compaction:    compactionConfig(w.window), // long tool-heavy runs stay within the window
		Todos:         actool.NewTodoStore(),      // Session Level Temporary To-doTodoWrite),It's for pure planning.
	}
	if hooks != nil { // typed-nil guard: only set when concrete (avoids harness panic)
		opts.Hooks = hooks
	}
	if w.mem != nil {
		opts.Memory = &agentcore.MemoryOptions{Store: w.mem, AutoInject: true, MaxInject: 3}
	}
	if w.tx != nil { // persist raw LLM conversation; one file per worked intent
		opts.Transcript = w.tx
		opts.SessionID = fmt.Sprintf("exp%d-worker-i%d", ts.ID(), intent.ID)
	}
	intentID := intent.ID
	emitWrap := func(r db.Activity) {
		if emit != nil {
			r.NodeID, r.Worker = &intentID, name
			emit(r)
		}
	}
	// The intent is delivered through the system prompt; the user only enters the start command.
	input := "Start system The intent in the tip is to do it, to produce facts, to stop."

	s := agentcore.NewSession(opts)
	defer s.Close() // release the session's background-task manager (temp dir + processes)

	// Budgets + settlement are owned by the SDK (MaxTurns/MaxDuration + Settlement):
	// on hit it runs a wrap-up turn and finishes with ReasonMaxTurns/ReasonTimeout.
	// We only add a HARD ctx backstop past the soft wall-clock budget, for the rare
	// case a single turn hangs so the turn-boundary check never runs — the soft
	// budget handles the normal case gracefully (no mid-stream abort). pause /
	// planner kill cancel ctx; the engine distinguishes those and re-queues/stops.
	// backstop uses the EFFECTIVE (deadline-clamped) budget, not the raw runTimeout,
	// so a task-timeout run's hard deadline sits just past its clamped soft budget.
	runCtx := ctx
	if maxDur > 0 {
		var runCancel context.CancelFunc
		runCtx, runCancel = context.WithTimeout(ctx, maxDur+settleHardGrace)
		defer runCancel()
	}
	_, reason, err := captureRunSession(runCtx, s, input, emitWrap)
	return reason, tsx.Writes(), err
}
