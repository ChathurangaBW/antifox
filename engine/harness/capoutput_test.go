package harness

import (
	"strings"
	"testing"

	"antifox/engine/llm"
	"antifox/engine/tool"
)

// TestCapOutputBoundsUncappedTools: a tool that never capped its own output
// (as MCP and custom tools did) is bounded by the global net.
func TestCapOutputBoundsUncappedTools(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000}
	big := strings.Repeat("x", 50_000)
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{llm.TextBlock(big)}})
	if got := len(res.Content[0].Text); got > 1000+512 {
		t.Fatalf("output not capped: %d bytes", got)
	}
	if !strings.Contains(res.Content[0].Text, "characters truncated") {
		t.Fatal("capped output must say it was truncated")
	}
}

// TestCapOutputIdempotent: output a tool already capped is not cut again.
func TestCapOutputIdempotent(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000}
	once := tool.Capture(tc, strings.Repeat("y", 50_000))
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{llm.TextBlock(once)}})
	if res.Content[0].Text != once {
		t.Fatal("already-capped output was modified")
	}
}

// TestCapOutputKeepsSmallAndNonText: small text and non-text blocks are unchanged.
func TestCapOutputKeepsSmallAndNonText(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000}
	img := llm.ContentBlock{Type: "image"}
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{llm.TextBlock("ok"), img}})
	if res.Content[0].Text != "ok" || res.Content[1].Type != "image" {
		t.Fatalf("unexpected change: %+v", res.Content)
	}
}

// TestCutHeadRuneSafe: truncation never splits a multi-byte character.
func TestCaptureKeepsValidUTF8(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1001}
	out := tool.Capture(tc, strings.Repeat("漏洞", 5000))
	if !strings.Contains(out, "characters truncated") {
		t.Fatal("expected truncation")
	}
	for _, r := range out {
		if r == '�' {
			t.Fatal("truncation split a multi-byte character")
		}
	}
}
