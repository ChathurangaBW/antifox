package agent

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/llm"
	"antifox/engine/permission"
	actool "antifox/engine/tool"
	"antifox/engine/transcript"
	"antifox/guard"
)

// ChatAgent is the generic, task-independent conversational runner behind the chat
// page. It generalizes MainAgent.Chat: any agent (built-in OR a custom one, by
// key) can be chatted with, multi-turn history resumed from the transcript. It is
// a PURE ASSISTANT — base tools are the SDK DefaultTools (Bash/Read/Write/Edit/
// LS/Glob/Grep) plus whatever skills/MCP the key is made visible; NO pentest
// graph/task context is injected (that stays exclusive to MainAgent).
type ChatAgent struct {
	prov        llm.Provider
	model       string
	workDir     string
	tx          *transcript.Store
	window      int
	proxyAddr   string
	proxyCACert string
	webSearch   WebSearchOpts
	guard       *guard.Guard // optional; nil disables intercept hooks for chat
}

func NewChatAgent(prov llm.Provider, model, workDir string, tx *transcript.Store, window int) *ChatAgent {
	return &ChatAgent{prov: prov, model: model, workDir: workDir, tx: tx, window: window}
}

// SetProxy points the chat agent's WebFetch/Bash at the recording proxy plus the
// CA cert it trusts (empty addr = direct). Kept for parity with the other agents.
func (c *ChatAgent) SetProxy(addr, caCert string) { c.proxyAddr, c.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the chat agent (off by default).
func (c *ChatAgent) SetWebSearch(o WebSearchOpts) { c.webSearch = o }

// SetGuard attaches a guard (with user-configured intercept rules) to this chat
// agent. Must be called before Chat; safe to call multiple times.
func (c *ChatAgent) SetGuard(g *guard.Guard) { c.guard = g }

// chatWorkDirSpec returns a working-directory notice appended to every chat
// agent's system prompt. Mirrors artifactSpec but without pentest-specific
// wording ("payload", "Capture Response") that would be odd in a general assistant.
func chatWorkDirSpec(workDir string) string {
	return "\n\n**File Output Statute**:If you need to write a file, write to the work directory " + workDir + "(This is the default. CWD,This is where the relative path is, and it can be used.——Don't write. /tmp Or some other absolute path."
}

// chatSystem renders the DB-managed prompt body for agentKey. Custom agents have
// no per-key in-code default, so DefaultAssistantPrompt is the render fallback.
func chatSystem(agentKey, workDir string) string {
	body := renderSystem(agentKey, DefaultAssistantPrompt, chatVars{})
	return withEnglish(body + untrustedContentTail(body) + chatWorkDirSpec(workDir))
}

// chatVars carries no runtime variables today — a custom prompt referencing any
// {{.X}} simply fails to render and falls back to DefaultAssistantPrompt.
type chatVars struct{}

// Chat runs ONE turn of a conversation with the agent identified by agentKey,
// resuming prior history keyed by sessionID. maxTurns is the per-turn agent step
// budget (0 = unlimited). maxDuration is the wall-clock run budget per turn
// (0 = unlimited); the timer resets each time Chat is called, so a new user
// message always starts a fresh countdown. webSearch gates network search for
// THIS agent (the global backend/key still come from the chat agent's config,
// but each agent decides on/off). emit receives each execution step (thinking /
// tool_use / tool_result / text / result), tagged with the agent key as the
// worker lane.
//
// bind, when non-nil, attaches the conversation to a task: the agent's domain
// tools are backed by that task's exploration graph, so report_finding, record_fact
// and list_findings work (with traffic linking and verification) instead of
// returning "requires a task context". The tools table still decides which of
// them this agent sees. nil keeps the plain conversation behavior.
func (c *ChatAgent) Chat(ctx context.Context, agentKey, sessionID, message string, maxTurns int, maxDuration time.Duration, webSearch bool, emit func(db.Activity), bind *TaskBinding) (string, error) {
	// gate the global web-search opts by this agent's own flag.
	ws := c.webSearch
	if !webSearch {
		ws.Enabled = false
	}

	// Per-session working directory: <workDir>/sessions/<sessionID>/
	// Isolates file writes across conversations, mirroring how workers use i<intentID>/.
	sessionWorkDir := filepath.Join(c.workDir, "sessions", sessionID)
	_ = os.MkdirAll(sessionWorkDir, 0o755)

	// Pure assistant: DefaultTools as the base; AugmentTools layers in the key's
	// visible skills/MCP and lets the DB tools table filter/override. DefaultTools
	// have no tools-table rows, so they always pass through.
	base := actool.DefaultTools()
	if bind != nil && bind.Store != nil {
		// Task-bound domain tools go in the base, so they take precedence over the
		// store-less copies AugmentTools would otherwise inject (per-task instances
		// win), and the tools table filters them by this agent's bindings as usual.
		base = append(bind.Tools(agentKey), base...)
	}
	tools, def, cleanup := AugmentTools(ctx, agentKey, base)
	defer cleanup()

	sys := chatSystem(agentKey, sessionWorkDir)
	if bind != nil && bind.Store != nil {
		sys += bind.promptBlock()
	}
	system, boundary := deferredSystem(sys, def)
	opts := agentcore.Options{
		Provider:        c.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // Walking records agent marks;loading agent CA Authentication MITM Resigned HTTPS Certificate
		WebFetchProxy:   c.proxyAddr,
		WebFetchCACert:  c.proxyCACert,
		// Network Search(Optional). ddgs No need. key;brave-free Yes BraveKey;tavily Yes TavilyKey.
		// WebSearchProxy is an independent egress proxy (http/https/socks5), unrelated to the traffic-recording MITM proxy; empty means a direct connection.
		EnableWebSearch:    ws.Enabled,
		WebSearchBackend:   ws.Backend,
		BraveSearchAPIKey:  ws.BraveKey,
		TavilySearchAPIKey: ws.TavilyKey,
		WebSearchProxy:     ws.Proxy,
		BashEnv:            proxyEnv(c.proxyAddr, c.proxyCACert), // Bash Sub-order default proxy+Trust CA
		WorkingDir:         sessionWorkDir,
		MaxTurns:           maxTurns,
		MaxDuration:        maxDuration,
		Compaction:         compactionConfig(c.window),
		Todos:              actool.NewTodoStore(),
		// large tool output spills to cmd-output/ under the session dir.
		// Truncation cap: the SDK default (tool.Capture is limited to 30000 characters).
		ToolOutputDir: filepath.Join(sessionWorkDir, "cmd-output"),
		// When the step budget is exhausted the SDK ends the run: a summary sentence is produced, the prompt and the remaining turns are counted, and the per-agent key stays editable in the backend.
		// (Custom agent One each.;Leave space./0 Use General Default:5 Wheel).
		Settlement: wrapupSettlement(agentKey, nil),
	}
	if c.guard != nil {
		opts.Hooks = c.guard.Hooks()
	}
	if c.tx != nil { // persist raw human↔AI conversation; one accumulating file per thread
		opts.Transcript = c.tx
		opts.SessionID = sessionID
	}
	s := agentcore.NewSession(opts)
	defer s.Close()
	// reload prior conversation so the agent has context across turns (each Chat is
	// a fresh session). First turn: no file yet → Resume loads nothing and proceeds.
	if c.tx != nil {
		_ = s.Resume(sessionID)
	}
	// re-unlock skill-gated MCPs from prior Skill() calls in the reloaded history so
	// revealed tools stay callable across the fresh session.
	seedUnlockFromHistory(s.Messages(), def.UnlockSkill)
	text, _, err := captureRunSession(ctx, s, message, func(r db.Activity) {
		if emit != nil {
			r.Worker = agentKey
			emit(r)
		}
	})
	return text, err
}
