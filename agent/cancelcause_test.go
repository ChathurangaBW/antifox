package agent

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"antifox/db"
	"antifox/engine/agentcore"
	"antifox/engine/llm"
)

// blockingProvider streams nothing until its context is cancelled, then fails
// with the context error: a model call in flight when a run is cancelled.
// With fail set it fails at once instead: a genuine provider error.
type blockingProvider struct{ fail error }

func (p blockingProvider) Stream(ctx context.Context, _ llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) {
		if p.fail != nil {
			yield(llm.StreamEvent{}, p.fail)
			return
		}
		<-ctx.Done()
		yield(llm.StreamEvent{}, ctx.Err())
	}
}

func (p blockingProvider) Complete(ctx context.Context, _ llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	if p.fail != nil {
		return llm.Message{}, "", llm.Usage{}, p.fail
	}
	<-ctx.Done()
	return llm.Message{}, "", llm.Usage{}, ctx.Err()
}

// runAndCollect runs one agent session on ctx and returns its final result row.
func runAndCollect(t *testing.T, ctx context.Context, p llm.Provider) db.Activity {
	t.Helper()
	var mu sync.Mutex
	var result *db.Activity
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = captureRun(ctx, agentcore.Options{Provider: p, SystemPrompt: []string{"test"}}, "go", func(a db.Activity) {
			if a.Kind == "result" {
				mu.Lock()
				cp := a
				result = &cp
				mu.Unlock()
			}
		})
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("run did not finish after cancellation")
	}
	mu.Lock()
	defer mu.Unlock()
	if result == nil {
		t.Fatal("no result row emitted")
	}
	return *result
}

// TestCancellationOutcomeNamesCause: each way of stopping a run produces a
// cancelled (not failed) result that names the actual cause.
func TestCancellationOutcomeNamesCause(t *testing.T) {
	cases := []struct {
		name  string
		cause error
		want  string
	}{
		{"new message", ErrReplacedByMessage, "replaced by a newer message"},
		{"operator stop", ErrStoppedByOperator, "stopped by the operator"},
		{"pause", ErrTaskPaused, "task paused"},
		{"kill", ErrWorkKilled, "work stopped by the planner"},
		{"shutdown", ErrShutdown, "server shutting down"},
		{"unrecorded", nil, "cause unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The cause is set on a parent, as the server does, and must reach
			// the run through derived contexts.
			parent, cancel := context.WithCancelCause(context.Background())
			ctx, stop := context.WithCancel(parent)
			defer stop()
			go func() { time.Sleep(50 * time.Millisecond); cancel(c.cause) }()
			r := runAndCollect(t, ctx, blockingProvider{})
			if r.Outcome != OutcomeCancelled || r.IsError {
				t.Fatalf("outcome=%q is_error=%v, want cancelled and not an error", r.Outcome, r.IsError)
			}
			if !strings.Contains(r.Summary, c.want) {
				t.Fatalf("summary %q does not name the cause %q", r.Summary, c.want)
			}
		})
	}

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		r := runAndCollect(t, ctx, blockingProvider{})
		if r.Outcome != OutcomeCancelled || !strings.Contains(r.Summary, "run time limit reached") {
			t.Fatalf("deadline: outcome=%q summary=%q", r.Outcome, r.Summary)
		}
	})

	t.Run("genuine failure stays a failure", func(t *testing.T) {
		r := runAndCollect(t, context.Background(), blockingProvider{fail: errors.New("upstream 500")})
		if r.Outcome != OutcomeFailed || !r.IsError {
			t.Fatalf("failure: outcome=%q is_error=%v summary=%q", r.Outcome, r.IsError, r.Summary)
		}
	})
}
