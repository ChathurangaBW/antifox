package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"

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
	// Global output cap at the single post-tool choke point: every tool (built-in,
	// MCP, custom, host) is bounded here whether or not it capped its own output,
	// so no tool can flood the context window. CaptureOnce is idempotent, so
	// output a tool already truncated or spilled is left untouched. Ported from Norma.
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
	for i, b := range res.Content {
		if b.Type == llm.BlockText {
			res.Content[i].Text = tool.CaptureOnce(tc, b.Text)
		}
	}
	return res
}
