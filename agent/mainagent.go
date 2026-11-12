package agent

import (
	"context"
	"fmt"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/llm"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
)

// MainAgent is the thin human-interface orchestrator (docs §4.2 / §7). The human
// chats with it; it observes (read tools), and steers by injecting hints
// (→planner) or direct high-priority intents (→frontier). It does NOT run the
// autonomous intent-generation loop (that is the planner's job).
type MainAgent struct {
	prov        llm.Provider
	model       string
	tx          *transcript.Store // raw LLM conversation persistence (nil = off)
	window      int               // context window in tokens (for compaction)
	maxTurns    int               // max agent turns per run (0 = unlimited)
	proxyAddr   string            // recording proxy for WebFetch (empty = direct)
	proxyCACert string            // recording proxy's CA cert path (HTTPS verify)
	webSearch   WebSearchOpts     // web_search tool backend selection (off by default)
	workDir     string            // shared work dir (surfaced in prompt as artifact-output target)
}

func NewMainAgent(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *MainAgent {
	return &MainAgent{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns}
}

// SetProxy points the main agent's WebFetch at the recording proxy plus the CA
// cert it trusts to verify HTTPS through it (empty addr = direct).
func (m *MainAgent) SetProxy(addr, caCert string) { m.proxyAddr, m.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the main agent (off by default).
func (m *MainAgent) SetWebSearch(o WebSearchOpts) { m.webSearch = o }

// mainAgentDefaultTmpl is the built-in EDITABLE body (Paragraph [A]) of the main agent
// prompt, seeded into agent_prompts. Goal is a {{.Goal}} template var; the Centre
// Production Export Statute tail is code-owned (artifactSpec), appended after rendering.
const mainAgentDefaultTmpl = `You are the "main agent" of an authorized penetration-testing system and serve as the human operator's interface. You do not perform exploration yourself or continuously generate intents autonomously; that is the planner's responsibility. Your duties are:

1. **Observe:** Use graph_overview / list_findings / list_facts / list_assets / get_worker_output to answer questions about current progress.
2. **Record findings (critical—do not omit this):** For every vulnerability confirmed by a worker or by your own review, **immediately call report_finding once per finding**. Supply vulnclass (name/type), severity (critical|high|medium|low; critical means direct, complete compromise of the system or data, such as RCE, unauthenticated arbitrary-file read that exposes keys/credentials, or an authorization bypass that permits account takeover), summary (including the affected target), evidence (the exact reproducible request, command, response, or other real PoC), and asset_ids whenever affected asset IDs are known. **Whenever possible, also supply structured reproduction data:** entry_point; prerequisites, including how to register a test account; steps containing each real request, expected result, and what it proves; and provenance. Copy requests verbatim from worker transcripts or actual execution so a human can reproduce the issue from scratch. **Record a confirmed issue immediately—do not ask permission and do not merely list it in your reply.** Only findings saved through report_finding appear in Findings, the exploration graph, and reports. First call list_findings to avoid duplicates. Never report an issue as confirmed based only on a version/CVE match or because it "looks exploitable."

   Confirmed exposure and information disclosure also count as findings. If a worker has actually accessed an unauthorized sensitive surface and captured response evidence—such as a management/debug endpoint, Swagger/API documentation, exposed configuration/backup/source, or a sensitive-data endpoint—record each one with report_finding. Assign severity by sensitivity: ordinary information disclosure=low; unauthorized sensitive/management surface=medium; direct takeover or credential disclosure=high. Do not leave these only as assets or facts.

   When the user asks you to summarize current findings, or when the task is ending or paused, review the workers' recorded facts and findings, deduplicate them with list_findings/list_facts, and save every qualifying vulnerability and confirmed exposure. Even if the primary objective was not fully exploited, deliver all confirmed exposures and risks instead of merely mentioning them in the reply.
3. **Steer the system:**
   - If the operator wants to change direction, emphasize a vulnerability class, or focus on an area, call add_hint so the planner reads the instruction on its next run.
   - If the operator wants a specific target tested immediately, call add_intent to inject a high-priority intent (priority 8–10).
4. Reply concisely in plain English, explaining what you did and which findings you recorded.

Current task goal: {{.Goal}}

Never fabricate findings. Record and answer only from real tool output and validated evidence.`

func mainAgentSystem(goal, workDir string) string {
	body := renderSystem("mainagent", mainAgentDefaultTmpl, MainVars{Goal: goal})
	return body + artifactSpec(workDir)
}

// Chat handles one human message and returns the assistant reply. emit, if
// non-nil, receives each execution step (thinking / tool_use / tool_result /
// text / result) so the main-agent session shows its work — exactly like the
// worker/planner sessions — not just the final answer.
func (m *MainAgent) Chat(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal, message string, emit func(db.Activity), notify func()) (string, error) {
	tsx := NewToolSet(ts, "human")
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.SetNotify(notify) // add_hint wakes this task's planner (debounced)
	// Field tools + Basic Default ToolsetsRead/Write/Edit/MultiEdit/LS/Glob/Grep/Bash)
	base := append(tsx.MainAgentTools(), actool.DefaultTools()...)
	tools, def, cleanup := AugmentTools(ctx, "mainagent", base)
	defer cleanup()
	system, boundary := deferredSystem(mainAgentSystem(goal, m.workDir), def)
	opts := agentcore.Options{
		Provider:        m.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // Walking records agent marks;loading agent CA Authentication MITM Resigned HTTPS Certificate
		WebFetchProxy:   m.proxyAddr,
		WebFetchCACert:  m.proxyCACert,
		// Network Search(Optional). ddgs No need. key;brave-free Yes BraveKey;tavily Yes TavilyKey.
		// WebSearchProxy is an independent egress proxy (http/https/socks5), unrelated to the traffic-recording MITM proxy; empty means a direct connection.
		EnableWebSearch:    m.webSearch.Enabled,
		WebSearchBackend:   m.webSearch.Backend,
		BraveSearchAPIKey:  m.webSearch.BraveKey,
		TavilySearchAPIKey: m.webSearch.TavilyKey,
		WebSearchProxy:     m.webSearch.Proxy,
		BashEnv:            proxyEnv(m.proxyAddr, m.proxyCACert), // Bash Sub-order default proxy+Trust CA
		MaxTurns:           m.maxTurns,                           // 0 = unlimited (configurable in agent management)
		Compaction:         compactionConfig(m.window),           // long chats stay within the window
		Todos:              actool.NewTodoStore(),                // Session Level Temporary To-doTodoWrite),It's for pure planning.
		// When the step budget is exhausted the SDK ends the run: a progress summary is produced for the user, and the prompt and remaining turns are counted (default 5 turns).
		Settlement: wrapupSettlement("mainagent", nil),
	}
	if m.tx != nil { // persist raw human↔AI conversation; one accumulating file per task
		opts.Transcript = m.tx
		opts.SessionID = fmt.Sprintf("exp%d-main", ts.ID())
	}
	s := agentcore.NewSession(opts)
	defer s.Close()
	// reload the prior conversation from the transcript so the agent has context
	// across turns (each Chat is a fresh session; without this it can't see earlier
	// messages). First turn: no file yet → Resume loads nothing and proceeds.
	if m.tx != nil {
		_ = s.Resume(opts.SessionID)
	}
	// C2: this session is fresh each turn; re-unlock skill-gated MCPs from prior
	// Skill() calls in the reloaded history so revealed tools stay callable.
	seedUnlockFromHistory(s.Messages(), def.UnlockSkill)
	text, _, err := captureRunSession(ctx, s, message, func(r db.Activity) {
		if emit != nil {
			r.Worker = "mainagent"
			emit(r)
		}
	})
	return text, err
}
