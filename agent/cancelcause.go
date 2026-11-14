package agent

import (
	"context"
	"errors"
)

// Why an agent run was cancelled. Every place that cancels a run passes one of
// these as the context cancellation cause (context.WithCancelCause), and the
// run's final result names it. Before, every interruption showed the same
// "task suspended / termination / restart" text, so a turn replaced by the
// operator's next chat message looked exactly like a crash or a restart.
var (
	ErrReplacedByMessage = errors.New("replaced by a newer message")
	ErrStoppedByOperator = errors.New("stopped by the operator")
	ErrTaskPaused        = errors.New("task paused")
	ErrWorkKilled        = errors.New("work stopped by the planner")
	ErrTaskCompleted     = errors.New("task completed, remaining work cancelled")
	ErrTaskDeadline      = errors.New("task deadline reached")
	ErrShutdown          = errors.New("server shutting down")
)

// Outcomes of a run, stored on its final "result" activity so the UI can tell a
// cancelled run from a failed one without parsing text.
const (
	OutcomeCompleted = "completed"
	OutcomeCancelled = "cancelled"
	OutcomeFailed    = "failed"
	OutcomeBudget    = "budget_reached"
)

// knownCauses are the causes cancelText recognises, in priority order.
var knownCauses = []error{
	ErrReplacedByMessage, ErrStoppedByOperator, ErrTaskPaused, ErrWorkKilled,
	ErrTaskCompleted, ErrTaskDeadline, ErrShutdown,
}

// cancelReason returns why ctx was cancelled: one of the named causes, "run time
// limit reached" for a deadline, or "cause unknown" when nothing more specific
// was recorded (a valid answer, never a guess).
func cancelReason(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return "cause unknown"
	}
	cause := context.Cause(ctx)
	for _, c := range knownCauses {
		if errors.Is(cause, c) {
			return c.Error()
		}
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return "run time limit reached"
	}
	return "cause unknown"
}

// cancelSummary is the result text for a cancelled run.
func cancelSummary(ctx context.Context) string {
	return "(Interrupted: " + cancelReason(ctx) + "; not completed)"
}
