package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"antifox/engine/llm"
	"antifox/engine/permission"
	"antifox/engine/tool"
)

// execOne resolves, permission-checks, and runs a single tool call, returning a
// tool_result block and any extra messages the tool injects after its result
// (Result.Extra — e.g. the Skill tool's instructions message). It never panics;
// failures become error results. It is safe to call from the streaming
// executor's goroutines (read-only access to loop config; permission/hook
// callbacks are the host's responsibility to make safe).
func (l *loop) execOne(use llm.ContentBlock, emitProgress func(tool.ProgressInfo)) (llm.ContentBlock, []llm.Message) {
	t, ok := l.in.Tools.Get(use.Name)
	if !ok {
		return llm.ToolResultText(use.ID, fmt.Sprintf("Error: unknown tool %q", use.Name), true), nil
	}
	input := use.Input

	// Schema validation (FR-04.5).
	if err := tool.ValidateInput(t.InputSchema(), input); err != nil {
		return llm.ToolResultText(use.ID, "Error: invalid tool input: "+err.Error(), true), nil
	}

	mode := l.in.PermissionMode
	if l.in.PermissionModeFunc != nil {
		mode = l.in.PermissionModeFunc()
	}
	pc := permission.Context{
		Mode:       mode,
		WorkingDir: l.in.WorkingDir,
		Allowed:    l.in.Allowed,
		Disallowed: l.in.Disallowed,
	}
	dec := permission.Evaluate(l.ctx, permission.Request{
		ToolName:     use.Name,
		Input:        input,
		IsReadOnly:   t.IsReadOnly(input),
		ToolDecision: t.CheckPermissions(l.ctx, input, pc),
		Ctx:          pc,
		Ask:          l.in.CanUseTool,
	})
	if dec.Behavior != permission.Allow {
		msg := dec.Message
		if msg == "" {
			msg = "denied"
		}
		return llm.ToolResultText(use.ID, "Tool call was not permitted: "+msg, true), nil
	}
	if len(dec.UpdatedInput) > 0 {
		input = dec.UpdatedInput
	}

	// Pre-tool hook.
	if l.in.Hooks != nil {
		if block, msg, updated := l.in.Hooks.PreToolUse(l.ctx, use.Name, input); block {
			return llm.ToolResultText(use.ID, "Blocked by hook: "+msg, true), nil
		} else if len(updated) > 0 {
			input = updated
		}
	}

	tc := &tool.ToolContext{
		WorkingDir:     l.in.WorkingDir,
		AgentID:        l.in.AgentID,
		Emit:           emitProgress,
		OutputDir:      l.in.ToolOutputDir,
		MaxOutputChars: l.in.MaxToolOutputChars,
		Tasks:          l.in.Tasks,
		Env:            l.in.BashEnv,
	}
	res, err := safeCall(l.ctx, t, input, tc)
	if err != nil {
		res = tool.Errorf("Error: " + err.Error())
	}
	// Output budget at the single post-tool choke point: every tool (built-in,
	// MCP, custom, host) passes through here whether or not it capped its own
	// output. The budget is for the whole result, not per block, so a tool that
	// returns many small blocks is bounded too; see capOutput.
	res = capOutput(tc, res)

	if l.in.Hooks != nil {
		raw, _ := json.Marshal(res.Flatten())
		l.in.Hooks.PostToolUse(l.ctx, use.Name, input, raw, res.IsError)
	}

	content := res.Content
	if len(content) == 0 {
		content = []llm.ContentBlock{llm.TextBlock("(no output)")}
	}
	return llm.ContentBlock{Type: llm.BlockToolResult, ToolUseID: use.ID, Content: content, IsError: res.IsError}, res.Extra
}

// safeCall runs a tool and turns a panic into an error result. The streaming
// executor runs each tool on its own goroutine, where an unrecovered panic would
// terminate the whole process, taking every running task and the web interface
// with it. The stack is logged for debugging; the model sees a plain error.
func safeCall(ctx context.Context, t tool.CoreTool, input json.RawMessage, tc *tool.ToolContext) (res tool.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[tool] %s panicked: %v\n%s", t.Name(), r, debug.Stack())
			res, err = tool.Errorf(fmt.Sprintf("tool %s failed with an internal error: %v", t.Name(), r)), nil
		}
	}()
	return t.Call(ctx, input, tc)
}

// capOutput runs every text block of a tool result through the idempotent output
// cap (tool.CaptureOnce). Non-text blocks, such as images, pass through untouched.
func capOutput(tc *tool.ToolContext, res tool.Result) tool.Result {
	budget := tool.MaxOutput(tc)
	used := 0
	nonText := 0
	var dropped []string // text blocks past the budget, in order
	droppedNonText := 0
	out := make([]llm.ContentBlock, 0, len(res.Content))
	for _, b := range res.Content {
		if b.Type != llm.BlockText {
			if nonText < maxNonTextBlocks {
				out = append(out, b)
				nonText++
			} else {
				droppedNonText++
			}
			continue
		}
		remaining := budget - used
		if remaining < minTextBlockBudget {
			dropped = append(dropped, b.Text)
			continue
		}
		// Cap this block at what is left of the result's budget. Oversized text
		// spills to a file (when the session has an output dir) or keeps head and
		// tail, so nothing is silently cut.
		sub := tool.ToolContext{}
		if tc != nil {
			sub = *tc
		}
		sub.MaxOutputChars = remaining
		b.Text = tool.CaptureOnce(&sub, b.Text)
		used += len(b.Text)
		out = append(out, b)
	}
	if len(dropped) > 0 || droppedNonText > 0 {
		out = append(out, llm.TextBlock(omittedNotice(tc, budget, dropped, droppedNonText)))
	}
	res.Content = out
	return res
}

// Result budget limits. The text budget is the session's MaxOutputChars for the
// whole result. Non-text blocks (images) are counted separately: at most
// maxNonTextBlocks per result.
const (
	maxNonTextBlocks = 8
	// minTextBlockBudget is the smallest remainder still worth giving a block;
	// below it, further blocks are reported as omitted rather than cut to a stub.
	minTextBlockBudget = 256
)

// omittedNotice reports what the budget left out. The omitted text is saved
// through the same capture path (spilled to a file when an output dir is set,
// otherwise head and tail), so the agent can still read it and knows it exists.
func omittedNotice(tc *tool.ToolContext, budget int, dropped []string, droppedNonText int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Tool output exceeded the %d-character total output limit.", budget)
	if len(dropped) > 0 {
		fmt.Fprintf(&b, " %d further text block(s) omitted.", len(dropped))
	}
	if droppedNonText > 0 {
		fmt.Fprintf(&b, " %d further non-text block(s) omitted (limit %d).", droppedNonText, maxNonTextBlocks)
	}
	b.WriteString("]")
	if len(dropped) > 0 {
		sub := tool.ToolContext{}
		if tc != nil {
			sub = *tc
		}
		sub.MaxOutputChars = minTextBlockBudget
		b.WriteString("\n")
		b.WriteString(tool.Capture(&sub, strings.Join(dropped, "\n")))
	}
	return b.String()
}
