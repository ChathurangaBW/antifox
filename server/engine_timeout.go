package server

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"antifox/agent"
	"antifox/db"
)

// Task Level Timeout Concorder(See docs/Task-level overtime and end-of-service design.md §4/§8.5)。
// Absolute Wall clock:Every belt. timeout A time-bound mission. goroutine,To point-driven orderly closing sequence:
//   ① settling → ② worker Stop taking new intentions. / ③ planner Drop Normal notify
//   ④ Drain the running workers (with the grace period) → ⑤ the planner's final round decides → ⑥ finality (with the guards).

const (
	settleDrainGrace     = 90 * time.Second // Waiting to run. worker The upper limit of elegant closing;More than hard cancel
	deadlinePollInterval = 2 * time.Second  // deadline Unsealed/LLM Interrogation interval when not ready
	deadlineMaxSleep     = 30 * time.Second // Single maximum sleep(Finality of periodic review)
)

// ---------- settling Status ----------

func (e *Engine) isSettling(taskID string) bool {
	v, _ := e.settling.Load(taskID)
	b, _ := v.(bool)
	return b
}

// markSettling flips settling on; returns true only for the first caller.
func (e *Engine) markSettling(taskID string) bool {
	_, loaded := e.settling.LoadOrStore(taskID, true)
	return !loaded
}

// ---------- I'm counting.(worker.Execute + planner.Plan),For drain ----------

func (e *Engine) inflightCounter(taskID string) *int64 {
	v, _ := e.inflight.LoadOrStore(taskID, new(int64))
	return v.(*int64)
}
func (e *Engine) incInflight(taskID string) { atomic.AddInt64(e.inflightCounter(taskID), 1) }
func (e *Engine) decInflight(taskID string) { atomic.AddInt64(e.inflightCounter(taskID), -1) }
func (e *Engine) inflightCount(taskID string) int64 {
	return atomic.LoadInt64(e.inflightCounter(taskID))
}

// ---------- deadline ----------

// taskDeadline returns the task's absolute deadline (unix). Prefers the in-process
// map (stamped this session); falls back to the DB-loaded value (restart), seeding
// the map. 0 = no timeout / not yet stamped.
func (e *Engine) taskDeadline(t *Task) int64 {
	if v, ok := e.deadline.Load(t.ID); ok {
		return v.(int64)
	}
	if t.DeadlineAt > 0 {
		e.deadline.Store(t.ID, t.DeadlineAt)
		return t.DeadlineAt
	}
	return 0
}

// stampFirstRun records first_run_at + deadline_at on the FIRST real run (LLM ready)
// of a timeout task, once per process. No-op when the task has no timeout.
func (e *Engine) stampFirstRun(t *Task) {
	if t.TimeoutSeconds <= 0 {
		return
	}
	if _, loaded := e.stamped.LoadOrStore(t.ID, true); loaded {
		return
	}
	dl, err := e.m.StampTaskFirstRun(t.ID)
	if err != nil {
		log.Printf("[deadline] task %s Seal first_run Failed: %v", t.ID, err)
		e.stamped.Delete(t.ID) // Allows the next attempt
		return
	}
	if dl > 0 {
		e.deadline.Store(t.ID, dl)
		log.Printf("[deadline] task %s First run,As of %s", t.ID, time.Unix(dl, 0).Format("2006-01-02 15:04:05"))
	}
}

// clockCtx layers the task's TaskClock (absolute deadline) onto a run's context so
// worker/planner can clamp their wall-clock budget and pick per-run vs task-timeout
// wrap-up words. final marks the coordinator-driven terminal planner round.
func (e *Engine) clockCtx(base context.Context, t *Task, final bool) context.Context {
	dl := e.taskDeadline(t)
	if dl <= 0 && !final {
		return base // no timeout → unchanged behavior
	}
	return agent.WithTaskClock(base, agent.TaskClock{DeadlineUnix: dl, Final: final})
}

// ---------- Coordinater ----------

// startDeadlineCoordinator launches the per-task deadline timer once (idempotent).
// Called from Run() and from the restart reload path, so non-active timeout tasks
// still get settled after their deadline even without live planner/worker loops.
func (e *Engine) startDeadlineCoordinator(ctx context.Context, t *Task) {
	if t == nil || t.TimeoutSeconds <= 0 {
		return
	}
	if _, loaded := e.coordStarted.LoadOrStore(t.ID, true); loaded {
		return
	}
	go e.deadlineCoordinator(ctx, t)
}

// deadlineCoordinator waits until the task's absolute deadline, then runs the settle
// sequence. Absolute wall-clock: it keeps counting through pauses.
func (e *Engine) deadlineCoordinator(ctx context.Context, t *Task) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			return // already finished (goals met / failed) — nothing to time out
		}
		dl := e.taskDeadline(t)
		if dl <= 0 {
			if sleepCtx(ctx, deadlinePollInterval) { // not yet stamped (task hasn't really run)
				return
			}
			continue
		}
		if remaining := time.Until(time.Unix(dl, 0)); remaining > 0 {
			nap := remaining
			if nap > deadlineMaxSleep {
				nap = deadlineMaxSleep
			}
			if sleepCtx(ctx, nap) {
				return
			}
			continue
		}
		e.settleTask(ctx, t)
		return
	}
}

// settleTask runs the ordered settle sequence once (§4 steps ①–⑥).
func (e *Engine) settleTask(ctx context.Context, t *Task) {
	if !e.markSettling(t.ID) {
		return
	}
	log.Printf("[deadline] task %s Overtime limit reached,Enter closing order", t.ID)

	// ④ Waiting to run. worker/planner drain(He's running. run It's a trap. MaxDuration Finish your own business.);
	// Over grace It's still empty. → Hard cancel Mission exec ctx(settling-aware Branches correctly classified)。
	hardStop := time.Now().Add(settleDrainGrace)
	for e.inflightCount(t.ID) > 0 {
		if time.Now().After(hardStop) {
			log.Printf("[deadline] task %s drain Timeout(%s),It's hard to cancel. run", t.ID, settleDrainGrace)
			e.cancelExec(t.ID)
			_ = sleepCtx(ctx, 3*time.Second) // Here. worker A little bit of time for the branch./Category
			break
		}
		if sleepCtx(ctx, 500*time.Millisecond) {
			return // All engines shut down.
		}
	}

	// ⑤ Final round planner(Task timeout word,Final target determination,No new intentions.)。
	met := e.runFinalPlannerRound(ctx, t)

	// ⑥ Finality(With guards.):met → done(completed);Otherwise... timeout。If the conventional path has been set first done,
	// Guard!(SetTaskStatusGuarded)You will refuse to overwhelm.,Reservations completed Semantics.
	status := "timeout"
	if met {
		status = "done"
	}
	won, err := e.m.SetTaskStatusGuarded(t.ID, status)
	switch {
	case err != nil:
		log.Printf("[deadline] task %s End of story failure: %v", t.ID, err)
	case won:
		log.Printf("[deadline] task %s Finish,Final=%s", t.ID, status)
	default:
		log.Printf("[deadline] task %s It's the end.,Keep Status", t.ID)
	}
}

// runFinalPlannerRound drives exactly ONE terminal planner round with the
// task-timeout planner words (final goal judgment; no new intents). Waits for the
// LLM to be ready (bounded by ctx) so a completable task isn't mis-judged timeout.
func (e *Engine) runFinalPlannerRound(ctx context.Context, t *Task) (met bool) {
	planner, _ := e.snapshotFor(t)
	for planner == nil {
		if sleepCtx(ctx, deadlinePollInterval) {
			return false
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			return false
		}
		planner, _ = e.snapshotFor(t)
	}
	emit := func(r db.Activity) { e.emitActivity(t, r) }
	e.emitActivity(t, db.Activity{Worker: "planner", Kind: "round",
		Summary: fmt.Sprintf("Overtime.·Final decision(I don't think so. %d Wheel)", e.nextPlannerRound(t.ID))})
	// Independence ctx(Hang up. execCancel,Avoid pause/Hard cancel Break this last round.),And... Final Injecting task timeout.
	fctx := e.clockCtx(ctx, t, true)
	e.incInflight(t.ID)
	tTaskID, _ := strconv.ParseInt(t.ID, 10, 64)
	met, reason, err := planner.Plan(fctx, tTaskID, e.m.assets, t.Store, t.Goal, t.drainDone(), emit)
	e.decInflight(t.ID)
	if err != nil {
		log.Printf("[deadline] task %s Final planning error: %v", t.ID, err)
	} else if met {
		log.Printf("[deadline] task %s Final target is met.: %s", t.ID, reason)
	}
	return met
}
