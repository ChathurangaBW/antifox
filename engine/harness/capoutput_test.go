package harness

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

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

func totalText(res tool.Result) int {
	n := 0
	for _, b := range res.Content {
		if b.Type == llm.BlockText {
			n += len(b.Text)
		}
	}
	return n
}

// slack covers the notices Capture and omittedNotice append.
const capSlack = 4096 + 512

// TestCapOutputAggregateManySmallBlocks: blocks that are each under the limit
// are still bounded as a whole, and the omission is reported.
func TestCapOutputAggregateManySmallBlocks(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000}
	var blocks []llm.ContentBlock
	for i := 0; i < 500; i++ {
		blocks = append(blocks, llm.TextBlock(strings.Repeat("a", 100)))
	}
	res := capOutput(tc, tool.Result{Content: blocks})
	if got := totalText(res); got > 1000+capSlack {
		t.Fatalf("aggregate not bounded: %d bytes from 500 small blocks", got)
	}
	last := res.Content[len(res.Content)-1].Text
	if !strings.Contains(last, "further text block(s) omitted") {
		t.Fatalf("omission not reported: %q", last)
	}
}

// TestCapOutputAggregateManyLargeBlocks: several blocks over the limit share one
// budget instead of each getting the full limit.
func TestCapOutputAggregateManyLargeBlocks(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 2000}
	var blocks []llm.ContentBlock
	for i := 0; i < 20; i++ {
		blocks = append(blocks, llm.TextBlock(strings.Repeat("b", 50_000)))
	}
	res := capOutput(tc, tool.Result{Content: blocks})
	if got := totalText(res); got > 2000+2*capSlack {
		t.Fatalf("aggregate not bounded: %d bytes", got)
	}
}

// TestCapOutputOrderAndNonTextLimit: order is preserved and non-text blocks are
// limited separately.
func TestCapOutputOrderAndNonTextLimit(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000}
	blocks := []llm.ContentBlock{llm.TextBlock("first")}
	for i := 0; i < maxNonTextBlocks+3; i++ {
		blocks = append(blocks, llm.ContentBlock{Type: "image"})
	}
	blocks = append(blocks, llm.TextBlock("last"))
	res := capOutput(tc, tool.Result{Content: blocks})
	if res.Content[0].Text != "first" || res.Content[len(res.Content)-2].Text != "last" {
		t.Fatalf("order not preserved: %+v", res.Content)
	}
	images := 0
	for _, b := range res.Content {
		if b.Type == "image" {
			images++
		}
	}
	if images != maxNonTextBlocks || !strings.Contains(res.Content[len(res.Content)-1].Text, "non-text block(s) omitted") {
		t.Fatalf("non-text limit: %d images kept, last=%q", images, res.Content[len(res.Content)-1].Text)
	}
}

// TestCapOutputUTF8AcrossBlocks: every block stays valid UTF-8 when cut.
func TestCapOutputUTF8AcrossBlocks(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 3001}
	var blocks []llm.ContentBlock
	for i := 0; i < 5; i++ {
		blocks = append(blocks, llm.TextBlock(strings.Repeat("漏洞", 2000)))
	}
	res := capOutput(tc, tool.Result{Content: blocks})
	for i, b := range res.Content {
		if !utf8.ValidString(b.Text) {
			t.Fatalf("block %d is not valid UTF-8", i)
		}
	}
}

// TestCapOutputSpillUnavailable: when the spill directory cannot be created,
// output is still bounded (head and tail kept) and nothing panics.
func TestCapOutputSpillUnavailable(t *testing.T) {
	notADir := t.TempDir() + "/file"
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tc := &tool.ToolContext{MaxOutputChars: 1000, OutputDir: notADir + "/sub"}
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{llm.TextBlock(strings.Repeat("c", 50_000))}})
	if got := totalText(res); got > 1000+capSlack || !strings.Contains(res.Content[0].Text, "characters truncated") {
		t.Fatalf("unavailable spill: %d bytes, %q", got, res.Content[0].Text[:80])
	}
}

// TestCapOutputSpillsLargeBlock: with an output dir, oversized text is saved to
// a file and the result points at it.
func TestCapOutputSpillsLargeBlock(t *testing.T) {
	tc := &tool.ToolContext{MaxOutputChars: 1000, OutputDir: t.TempDir()}
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{llm.TextBlock(strings.Repeat("d", 50_000))}})
	if !strings.Contains(res.Content[0].Text, "<persisted-output>") {
		t.Fatalf("large output not spilled: %q", res.Content[0].Text[:80])
	}
}

// TestCapOutputKeepsLargeImages: the non-text size limit is not the text budget.
// A screenshot-sized block (1 MB serialized) passes unchanged with the default
// 30000-character text budget.
func TestCapOutputKeepsLargeImages(t *testing.T) {
	shot := llm.ContentBlock{Type: "image", Thinking: strings.Repeat("A", 1<<20)}
	res := capOutput(&tool.ToolContext{}, tool.Result{Content: []llm.ContentBlock{shot, shot}})
	if len(res.Content) != 2 || len(res.Content[0].Thinking) != 1<<20 || len(res.Content[1].Thinking) != 1<<20 {
		t.Fatalf("large non-text blocks were not kept: %d blocks", len(res.Content))
	}
}

// TestCapOutputNonTextSizeBudget: non-text blocks are bounded by size as well as
// count, so a few huge blocks cannot make a result unbounded.
func TestCapOutputNonTextSizeBudget(t *testing.T) {
	old := maxNonTextBytes
	maxNonTextBytes = 2000
	t.Cleanup(func() { maxNonTextBytes = old })
	tc := &tool.ToolContext{MaxOutputChars: 2000}
	huge := llm.ContentBlock{Type: "thinking", Thinking: strings.Repeat("x", 5000)}
	small := llm.ContentBlock{Type: "image"}
	res := capOutput(tc, tool.Result{Content: []llm.ContentBlock{small, huge, small}})
	total := 0
	for _, b := range res.Content {
		if b.Type != llm.BlockText {
			total += blockSize(b)
		}
	}
	if total > 2000 {
		t.Fatalf("non-text blocks total %d bytes, over the 2000 budget", total)
	}
	last := res.Content[len(res.Content)-1].Text
	if !strings.Contains(last, "1 further non-text block(s) omitted") || !strings.Contains(last, "not saved") {
		t.Fatalf("size-based omission not reported clearly: %q", last)
	}
}
