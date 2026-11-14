// Package guard implements the safety boundary layer (docs §11): side-effect
// gating of destructive/exfil shell commands, an audit log, and Observer/G5
// failure attribution. Every tool call passes through the PreToolUse hook before
// executing. (The RoE authorization-scope mechanism was removed; a replacement
// may be added later.)
package guard

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"antifox/engine/hook"
	"antifox/intercept"
)

// AuditEntry records one gated tool call.
type AuditEntry struct {
	TS      int64  `json:"ts"`
	Tool    string `json:"tool"`
	Action  string `json:"action"` // allow|block
	Reason  string `json:"reason,omitempty"`
	Command string `json:"command,omitempty"`
}

// Guard enforces the side-effect policy via agent-core hooks.
type Guard struct {
	mu          sync.Mutex
	audit       []AuditEntry
	attrib      map[string]int // failure attribution counts (Observer / G5)
	reg         *hook.Registry
	unattended  *hook.Registry         // same checks, but "ask" rules block (see UnattendedHooks)
	interceptor *intercept.Interceptor // optional; nil disables user-configured rules
}

// unattendedAskMessage tells an agent that cannot wait for a human why a call
// was blocked and what to do instead.
const unattendedAskMessage = "An intercept rule requires operator approval for this call, and this agent runs without an operator to approve it. " +
	"Do not retry it here: assign the work to a worker as an intent, where the approval request reaches the operator."

// New creates a Guard without user-configured intercept rules (used for pentest
// tasks where the Interceptor is not yet available).
func New() *Guard { return newGuard(nil) }

// NewWithInterceptor creates a Guard with user-configured intercept rules.
func NewWithInterceptor(ic *intercept.Interceptor) *Guard { return newGuard(ic) }

func newGuard(ic *intercept.Interceptor) *Guard {
	g := &Guard{attrib: map[string]int{}, interceptor: ic}
	g.reg = hook.NewRegistry().
		On(hook.PreToolUse, g.preToolUse).
		On(hook.PostToolUse, g.postToolUse)
	g.unattended = hook.NewRegistry().
		On(hook.PreToolUse, func(ctx context.Context, ev hook.Event) hook.Result { return g.check(ctx, ev, false) }).
		On(hook.PostToolUse, g.postToolUse)
	return g
}

// Hooks returns the hook registry to attach to an agent session.
func (g *Guard) Hooks() *hook.Registry { return g.reg }

// UnattendedHooks returns the registry for agents that run with no operator
// watching, such as the planner. Built-in and user deny rules apply exactly as
// in Hooks; a rule that asks for approval blocks the call instead of waiting,
// because no one would answer and the agent would stall.
func (g *Guard) UnattendedHooks() *hook.Registry { return g.unattended }

var (
	reDestructive = regexp.MustCompile(`(?i)\b(rm\s+-rf\s+/|mkfs|dd\s+if=|:\(\)\s*\{|shutdown|reboot|>\s*/dev/sd)`)
	reExfil       = regexp.MustCompile(`(?i)(curl|wget|nc|ncat)\b[^|]*\b(\|\s*(curl|wget|nc))`)
)

func (g *Guard) preToolUse(ctx context.Context, ev hook.Event) hook.Result {
	return g.check(ctx, ev, true)
}

// check runs the built-in shell rules, then the user intercept rules. allowAsk
// selects what an "ask" rule does: wait for the operator, or block at once.
func (g *Guard) check(ctx context.Context, ev hook.Event, allowAsk bool) hook.Result {
	// Gate the shell-command surface: Bash + the interactive-shell tools (shell_open's
	// command, shell_send's text). Same destructive/exfil rules — an interactive
	// session must not bypass the safety boundary. Other tools pass through.
	var cmd string
	switch ev.ToolName {
	case "Bash", "shell_open":
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(ev.Input, &in)
		cmd = in.Command
	case "shell_send":
		var in struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(ev.Input, &in)
		cmd = in.Text
	default:
		g.record(ev.ToolName, "allow", "", "")
		return g.applyIntercept(ctx, ev, allowAsk)
	}
	if strings.TrimSpace(cmd) == "" { // e.g. shell_send with only keys/hex → nothing to check
		g.record(ev.ToolName, "allow", "", "")
		return g.applyIntercept(ctx, ev, allowAsk)
	}

	// destructive / exfil gating (§11 side-effect gating)
	if reDestructive.MatchString(cmd) {
		return g.block(ev.ToolName, "Destructive orders are denied at secure borders (and manual approval is required)", cmd)
	}
	if reExfil.MatchString(cmd) {
		return g.block(ev.ToolName, "Suspected data leak pipe denied.", cmd)
	}

	g.record(ev.ToolName, "allow", "", cmd)
	return g.applyIntercept(ctx, ev, allowAsk)
}

// applyIntercept evaluates user-configured intercept rules against the tool call.
// It is called after all built-in safety checks pass.
func (g *Guard) applyIntercept(ctx context.Context, ev hook.Event, allowAsk bool) hook.Result {
	if g.interceptor == nil {
		return hook.Result{}
	}
	if !g.interceptor.IsToolEnabled(ev.ToolName) {
		return hook.Result{}
	}
	dec, matched := g.interceptor.Match(ev.ToolName, ev.Input)
	if !matched {
		return hook.Result{}
	}
	switch dec.Action {
	case "deny":
		return g.block(ev.ToolName, dec.Message, "")
	case "allow":
		return hook.Result{}
	case "ask":
		if !allowAsk {
			return g.block(ev.ToolName, unattendedAskMessage, "")
		}
		// If the worker context is already cancelled (task stopped / killed), block
		// immediately without creating a pending record — avoids orphaned DB entries
		// and makes execOne complete fast, reducing the race against drainSynthetic.
		if ctx.Err() != nil {
			return g.block(ev.ToolName, "The work has been cancelled. Interception rules are preventing enforcement.", "")
		}
		convID := intercept.ConvIDFromContext(ctx)
		if !g.interceptor.HandleAsk(ctx, convID, dec, ev.ToolName, ev.Input) {
			return g.block(ev.ToolName, "Timeout denied or approved by user", "")
		}
		return hook.Result{}
	}
	return hook.Result{}
}

var reBlocked = regexp.MustCompile(`(?i)(\bhttp[/0-9.]*\s+(?:403|429)\b|\bstatus[:\s=]+(?:403|429)\b|\b(?:forbidden|waf|rate.?limit|captcha|access\s+denied|request\s+blocked)\b)`)

// postToolUse is the Observer failure-attribution hook (G5): it classifies tool
// results into blocked / error / ok so the planner can change strategy instead
// of giving up at a WAF.
func (g *Guard) postToolUse(_ context.Context, ev hook.Event) hook.Result {
	switch ev.ToolName {
	case "Bash", "shell_open", "shell_send":
	default:
		return hook.Result{}
	}
	class := "ok"
	switch {
	case reBlocked.Match(ev.Result):
		class = "blocked"
	case ev.IsError:
		class = "error"
	}
	g.mu.Lock()
	g.attrib[class]++
	g.mu.Unlock()
	return hook.Result{}
}

// Attributions returns failure-attribution counts (Observer / G5).
func (g *Guard) Attributions() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]int, len(g.attrib))
	for k, v := range g.attrib {
		out[k] = v
	}
	return out
}

func (g *Guard) block(tool, reason, cmd string) hook.Result {
	g.record(tool, "block", reason, cmd)
	return hook.Result{Decision: "block", Message: reason}
}

func (g *Guard) record(tool, action, reason, cmd string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.audit = append(g.audit, AuditEntry{TS: time.Now().Unix(), Tool: tool, Action: action, Reason: reason, Command: cmd})
	if len(g.audit) > 2000 {
		g.audit = g.audit[len(g.audit)-2000:]
	}
}

// Audit returns a snapshot of recent gated calls (most recent last).
func (g *Guard) Audit() []AuditEntry {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]AuditEntry, len(g.audit))
	copy(out, g.audit)
	return out
}
