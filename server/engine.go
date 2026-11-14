package server

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"antifox/agent"
	"antifox/db"
	"antifox/engine/harness"
	"antifox/engine/llm"
	"antifox/intercept"
)

// model_error(provider/API Malfunction:LLM The layer is running out of time, or the stream is starting to run out.
// It's over. work It's not that it's not really a failure, it's an external shaking. Default as permanent
// blocked It's a waste of purpose, so there's an extra run for the end, and each time, there you go.
// provider Time to recover; if suspended during re-testing/Termination/The cancellation will immediately yield to the corresponding branch.
const (
	modelErrorRetries      = 2               // model_error Number of additional retests after closing
	modelErrorRetryBackoff = 3 * time.Second // The Avoidance Before Every Again
)

// Engine drives the event-driven exploration loop with real LLM agents
// (docs §4.3/§4.4): on asset/exploration-graph change (debounced) it wakes the
// planner, which reads the route, queries assets, judges goals and emits intents;
// N concurrent work agents claim intents and execute them. There is no
// simulation mode — an LLM provider is required. The planner/worker can be
// (re)installed at runtime (LLM configured from the UI); the loops always run
// but idle until an LLM is set.
type Engine struct {
	m        *Manager
	debounce time.Duration

	bc *Broadcaster // live activity pub/sub (SSE)

	mu      sync.RWMutex
	planner *agent.Planner
	worker  *agent.Worker
	started sync.Map // taskID -> bool, so Run is idempotent per task
	lastAct sync.Map // taskID -> int64 unix, last planner/worker activity (heartbeat)
	paused  sync.Map // taskID -> bool, user-paused (planner + workers idle but loops alive)

	// per-task execution context: each planner.Plan / worker.Execute runs under it,
	// so pausing can CANCEL an in-flight run (not just skip the next one). Recreated
	// on resume since cancelling is one-shot.
	execMu     sync.Mutex
	execCancel map[string]context.CancelCauseFunc
	execCtx    map[string]context.Context

	// per-work (per running intent) cancel, so the planner's kill_work can stop a
	// single worker without pausing the whole task. Keyed by globally-unique intent id.
	workMu     sync.Mutex
	workCancel map[int64]context.CancelCauseFunc

	// steerBox queues planner course-corrections for a running work (keyed by intent
	// id). The worker's PreToolUse hook drains it before its next tool call and hands
	// the message to the model (blocking that call) so it re-plans — no kill needed.
	steerMu  sync.Mutex
	steerBox map[int64][]string

	plannerRound sync.Map // taskID -> int, planner round counter (for UI round separators)

	// Task level timeout(See docs/Task-level overtime and end-of-service design.md):
	settling     sync.Map // taskID -> bool, Task is in finish order(Stop pie./New intent.)
	deadline     sync.Map // taskID -> int64 unix, Absolute deadline(First run-time stamp;0/Defaults=Unlimited)
	stamped      sync.Map // taskID -> bool, first_run_at Is it stamped?(Only once in this process)
	inflight     sync.Map // taskID -> *int64, He's running. planner.Plan + worker.Execute Count(For drain)
	coordStarted sync.Map // taskID -> bool, deadline Has the co-ordinates been activated?(Run/reload Heavy.)

	// resolve, if set, returns a task's dedicated planner/worker when it pins a
	// specific LLM profile (nil,nil = use the global active pair). Set once at startup.
	resolve func(t *Task) (*agent.Planner, *agent.Worker)
}

// nextPlannerRound returns the next planner round number for a task (1-based).
func (e *Engine) nextPlannerRound(taskID string) int {
	v, _ := e.plannerRound.LoadOrStore(taskID, 0)
	n := v.(int) + 1
	e.plannerRound.Store(taskID, n)
	return n
}

// Pause stops a task: marks it paused AND cancels any in-flight planner/worker run
// for it (a long worker.Execute would otherwise keep going until it finishes).
func (e *Engine) Pause(taskID string) {
	e.paused.Store(taskID, true)
	e.cancelExec(taskID, agent.ErrTaskPaused)
}

// cancelExec cancels a task's current per-task exec context (any in-flight
// planner.Plan / worker.Execute), if present, recording cause so each run's
// result says why it stopped. Shared by Pause, task completion and the settle
// sequence's hard-drain backstop.
func (e *Engine) cancelExec(taskID string, cause error) {
	e.execMu.Lock()
	if cancel := e.execCancel[taskID]; cancel != nil {
		cancel(cause)
	}
	e.execMu.Unlock()
}

// Resume un-pauses a task and nudges a fresh planning round. The next exec under
// it gets a fresh (uncancelled) context.
func (e *Engine) Resume(t *Task) {
	e.paused.Delete(t.ID)
	t.Notify()
}

// Rerun continues a stopped/finished task (Continue running): the caller has already reset
// the task to a non-terminal status + cleared its run clock (Manager.ReactivateTask).
// Here we drop the stale deadline, ensure the planner/worker loops are alive, re-arm
// the timeout coordinator, and nudge a planning round. The exploration graph is kept.
func (e *Engine) Rerun(ctx context.Context, t *Task) {
	e.deadline.Delete(t.ID)     // clear stale absolute deadline (re-stamped on next run)
	e.coordStarted.Delete(t.ID) // allow the deadline coordinator to re-arm below
	e.paused.Delete(t.ID)
	e.Run(ctx, t)                      // start loops if not yet running this session; else nudge
	e.startDeadlineCoordinator(ctx, t) // re-arm timeout timer (dedup-safe via coordStarted)
	t.Notify()
}

// execContextFor returns a live per-task context derived from parent, recreating
// it if a prior pause cancelled it.
func (e *Engine) execContextFor(parent context.Context, taskID string) context.Context {
	e.execMu.Lock()
	defer e.execMu.Unlock()
	if e.IsPaused(taskID) {
		// never hand out a live context while paused (guards the claim→Execute race)
		c, cancel := context.WithCancelCause(parent)
		cancel(agent.ErrTaskPaused)
		return c
	}
	if c := e.execCtx[taskID]; c != nil && c.Err() == nil {
		return c
	}
	c, cancel := context.WithCancelCause(parent)
	e.execCtx[taskID] = c
	e.execCancel[taskID] = cancel
	return c
}

// IsPaused reports whether a task is user-paused.
func (e *Engine) IsPaused(taskID string) bool {
	v, ok := e.paused.Load(taskID)
	return ok && v.(bool)
}

// Started reports whether the engine loops are running for a task.
func (e *Engine) Started(taskID string) bool {
	_, ok := e.started.Load(taskID)
	return ok
}

// LastActivity returns the unix time of the last planner/worker activity for a
// task (0 if none yet).
func (e *Engine) LastActivity(taskID string) int64 {
	if v, ok := e.lastAct.Load(taskID); ok {
		return v.(int64)
	}
	return 0
}

func (e *Engine) touch(taskID string) { e.lastAct.Store(taskID, time.Now().Unix()) }

func NewEngine(m *Manager) *Engine {
	return &Engine{m: m, debounce: 800 * time.Millisecond, bc: NewBroadcaster(),
		execCancel: map[string]context.CancelCauseFunc{}, execCtx: map[string]context.Context{},
		workCancel: map[int64]context.CancelCauseFunc{}, steerBox: map[int64][]string{}}
}

// registerWork records the cancel for the work currently running intentID.
func (e *Engine) registerWork(intentID int64, cancel context.CancelCauseFunc) {
	e.workMu.Lock()
	e.workCancel[intentID] = cancel
	e.workMu.Unlock()
}

// unregisterWork removes and releases the work's cancel (called when Execute returns).
func (e *Engine) unregisterWork(intentID int64) {
	e.workMu.Lock()
	if c := e.workCancel[intentID]; c != nil {
		delete(e.workCancel, intentID)
		c(nil) // release context resources (no-op if already cancelled)
	}
	e.workMu.Unlock()
	e.steerMu.Lock()
	delete(e.steerBox, intentID) // drop any undelivered steering for a finished work
	e.steerMu.Unlock()
}

// SteerWork queues a mid-run course-correction for the work running intentID (the
// planner's steer_work tool). The worker delivers it before its next tool call and
// re-plans — no kill. Errors if no work is currently running that intent.
func (e *Engine) SteerWork(intentID int64, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return fmt.Errorf("Scattering messages cannot be empty.")
	}
	e.workMu.Lock()
	running := e.workCancel[intentID] != nil
	e.workMu.Unlock()
	if !running {
		return fmt.Errorf("Intention %d No running work((perhaps closed or not received)", intentID)
	}
	e.steerMu.Lock()
	e.steerBox[intentID] = append(e.steerBox[intentID], msg)
	e.steerMu.Unlock()
	return nil
}

// drainSteer pops the oldest queued steering message for intentID (FIFO), if any.
func (e *Engine) drainSteer(intentID int64) (string, bool) {
	e.steerMu.Lock()
	defer e.steerMu.Unlock()
	q := e.steerBox[intentID]
	if len(q) == 0 {
		return "", false
	}
	msg := q[0]
	if len(q) == 1 {
		delete(e.steerBox, intentID)
	} else {
		e.steerBox[intentID] = q[1:]
	}
	return msg, true
}

// steerHooks wraps the guard's hook runner so the planner can steer a running work:
// before each tool call it drains a queued course-correction (if any) and blocks the
// call, handing the message back to the model — which re-plans its next step instead
// of running the tool. No queued message → the guard behaves exactly as before.
type steerHooks struct {
	inner harness.HookRunner
	drain func() (string, bool)
}

func (h steerHooks) PreToolUse(ctx context.Context, name string, input []byte) (bool, string, []byte) {
	if msg, ok := h.drain(); ok {
		return true, "[Planner's real time." + msg +
			"\n(This is the immediate instruction of the planners to this intent; the call for this tool has not been implemented and please adjust the next step accordingly. In the event of a conflict with your current intentions, this shall prevail.", nil
	}
	if h.inner != nil {
		return h.inner.PreToolUse(ctx, name, input)
	}
	return false, "", nil
}

func (h steerHooks) PostToolUse(ctx context.Context, name string, input, result []byte, isErr bool) {
	if h.inner != nil {
		h.inner.PostToolUse(ctx, name, input, result, isErr)
	}
}

func (h steerHooks) Stop(ctx context.Context, messages []llm.Message) (bool, []string, string) {
	if h.inner != nil {
		return h.inner.Stop(ctx, messages)
	}
	return false, nil, ""
}

// KillWork cancels the in-flight work running intentID (planner's kill_work tool).
// The work's agent-core session honors ctx cancellation and aborts promptly.
func (e *Engine) KillWork(intentID int64) error {
	e.workMu.Lock()
	c := e.workCancel[intentID]
	e.workMu.Unlock()
	if c == nil {
		return fmt.Errorf("Intention %d No running work((perhaps closed or not received)", intentID)
	}
	c(agent.ErrWorkKilled)
	return nil
}

// Broadcaster exposes the engine's live activity pub/sub (used by the SSE handler).
func (e *Engine) Broadcaster() *Broadcaster { return e.bc }

// emitActivity persists one captured step AND fans it out to live subscribers,
// from a single point so storage and the SSE stream never diverge.
func (e *Engine) emitActivity(t *Task, r db.Activity) {
	id, err := e.appendActivity(t, r)
	if err != nil {
		// NO LONGER SILENT: dropping a record breaks command↔result pairing in the
		// trace — a tool_use whose tool_result was lost shows as "Under implementation" forever.
		log.Printf("[activity] task %s Failed to try again, discard the record (kind=%s tool=%s tuid=%s): %v",
			t.ID, r.Kind, r.Tool, r.ToolUseID, err)
		e.touch(t.ID)
		return
	}
	r.ID = id
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	e.bc.Publish(t.ID, r)
	e.touch(t.ID)
}

// appendActivity persists one activity row, retrying briefly on write failure.
// Concurrent planner + worker inserts into the same exploration's activity log
// occasionally fail; a couple of quick retries recover most. Crucially, every
// failure is now LOGGED (it used to be swallowed by an `if err == nil`), so the
// underlying DB error is finally visible for diagnosis.
func (e *Engine) appendActivity(t *Task, r db.Activity) (int64, error) {
	var id int64
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if id, err = t.Store.AppendActivity(r); err == nil {
			if attempt > 1 {
				log.Printf("[activity] task %s Writing first %d Success of retry (worker=%s kind=%s tool=%s)",
					t.ID, attempt, r.Worker, r.Kind, r.Tool)
			}
			return id, nil
		}
		log.Printf("[activity] task %s Writing Failed (I don't think so. %d/3 Minor, worker=%s kind=%s tool=%s): %v",
			t.ID, attempt, r.Worker, r.Kind, r.Tool, err)
		time.Sleep(time.Duration(attempt) * 25 * time.Millisecond)
	}
	return 0, err
}

// UseLLM installs (or replaces) the LLM planner + work agent at runtime.
func (e *Engine) UseLLM(p *agent.Planner, w *agent.Worker) {
	e.mu.Lock()
	e.planner, e.worker = p, w
	e.mu.Unlock()
}

func (e *Engine) snapshot() (*agent.Planner, *agent.Worker) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.planner, e.worker
}

// SetAgentResolver installs a per-task planner/worker resolver (wired by the server).
// Called once at startup before any task loop runs, so no lock is needed on reads.
func (e *Engine) SetAgentResolver(fn func(t *Task) (*agent.Planner, *agent.Worker)) {
	e.resolve = fn
}

// snapshotFor returns the planner/worker a task should run on: the task's dedicated
// pair when it pins a specific LLM profile (via the resolver), else the global active
// pair. The resolver returning nil means "no override" (fall back to global).
func (e *Engine) snapshotFor(t *Task) (*agent.Planner, *agent.Worker) {
	if e.resolve != nil {
		if p, w := e.resolve(t); p != nil && w != nil {
			return p, w
		}
	}
	return e.snapshot()
}

// Ready reports whether an LLM provider is configured.
func (e *Engine) Ready() bool {
	p, w := e.snapshot()
	return p != nil && w != nil
}

// Mode reports "llm" when ready, else "idle".
func (e *Engine) Mode() string {
	if e.Ready() {
		return "llm"
	}
	return "idle"
}

// Run starts the planner loop + N worker loops for a task. The loops always run
// but no-op until an LLM is configured (so a task created while idle picks up
// automatically once LLM is set from the UI).
func (e *Engine) Run(ctx context.Context, t *Task) {
	if _, loaded := e.started.LoadOrStore(t.ID, true); loaded {
		t.Notify() // already running — just nudge a planning round
		return
	}
	e.touch(t.ID)
	go e.plannerLoop(ctx, t)
	workers := e.m.Workers()
	for i := 0; i < workers; i++ {
		go e.workerLoop(ctx, t, fmt.Sprintf("work#%d", i+1))
	}
	e.startDeadlineCoordinator(ctx, t) // Task level timer(Only timeout>0;Heavy.)
	t.Notify()                         // kick the first planning round (acted on once LLM is ready)
}

func (e *Engine) plannerLoop(ctx context.Context, t *Task) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.notify:
			// debounce: coalesce a burst of changes into one planning round
			timer := time.NewTimer(e.debounce)
		drain:
			for {
				select {
				case <-t.notify:
				case <-timer.C:
					break drain
				}
			}
			planner, _ := e.snapshotFor(t)
			if planner == nil {
				continue // idle until LLM configured
			}
			if e.IsPaused(t.ID) {
				continue // user-paused: don't plan
			}
			// terminal task (goals all met → done, or failed): the run is over. A
			// resume/nudge — e.g. auto-resume of the active task on restart — must NOT
			// re-plan (it would burn an LLM round and re-confirm a settled result).
			if isTerminalStatus(t.Status) {
				continue
			}
			// Task-level timeout: stop the normal wake-up, have the worker write back, and resume only once. No further notification.
			// Trigger the conventional planning wheel;The final round is coordinated.(settleTask)Direct driver,Don't go here.
			if e.isSettling(t.ID) {
				continue
			}
			e.stampFirstRun(t) // First real plan → Guy. first_run_at + Fine. deadline(Only with timeout Tasks)
			e.touch(t.ID)
			emit := func(r db.Activity) { e.emitActivity(t, r) }
			ectx := e.clockCtx(e.execContextFor(ctx, t.ID), t, false) // cancellable by Pause; Tasks deadline
			log.Printf("[planner] task %s Planning...", t.ID)
			// round marker: each Plan() is one planner round; emit a boundary so the
			// UI can separate rounds in the transcript (kind='round').
			e.emitActivity(t, db.Activity{Worker: "planner", Kind: "round",
				Summary: fmt.Sprintf("I don't think so. %d Round planning", e.nextPlannerRound(t.ID))})
			// which intents' completion triggered this round (may be several — debounce
			// coalesces a burst; empty for non-completion wakes like a new hint).
			doneIntents := t.drainDone()
			e.incInflight(t.ID)
			taskIDInt, _ := strconv.ParseInt(t.ID, 10, 64)
			met, reason, err := planner.Plan(ectx, taskIDInt, e.m.assets, t.Store, t.Goal, doneIntents, plannerHooks(t), emit)
			e.decInflight(t.ID)
			switch {
			case err != nil && ectx.Err() == nil:
				log.Printf("[planner] task %s Planning error: %v", t.ID, err)
			case met:
				log.Printf("[planner] task %s Target is set.: %s", t.ID, reason)
				// All goals achieved → set the task status to done (the frontend DTO is what surfaces this finality).
				if err := e.m.SetTaskStatus(t.ID, "done"); err != nil {
					log.Printf("[planner] task %s Failed to mark down library: %v", t.ID, err)
				}
				// Mission complete. → Cancel the running immediately. worker:It doesn't make sense what they're trying to do.
				// Next round. worker The cycle crashes into the final door and no new intent.;That's what's been canceled."Task completed"Branch
				// Attribution stopped(Not blocked).
				e.cancelExec(t.ID, agent.ErrTaskCompleted)
			default:
				log.Printf("[planner] task %s Planning completed", t.ID)
			}
			e.touch(t.ID)
		}
	}
}

func (e *Engine) workerLoop(ctx context.Context, t *Task, name string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, worker := e.snapshotFor(t)
		if worker == nil {
			if sleepCtx(ctx, 1500*time.Millisecond) {
				return
			}
			continue
		}
		if e.IsPaused(t.ID) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // user-paused: don't claim/execute intents
		}
		if e.isSettling(t.ID) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // Task timed out.:No more new intentions.(I'm closing myself up running.,Co-ordinaters and others drain)
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			if sleepCtx(ctx, 1000*time.Millisecond) {
				return
			}
			continue // Mission is final(done/failed/timeout):Stop receiving legacy intent,Don't run when you're done. frontier
		}
		intent := e.claimNext(t, name)
		if intent == nil {
			if sleepCtx(ctx, 800*time.Millisecond) {
				return
			}
			continue
		}
		log.Printf("[worker %s] task %s Intention to receive #%d", name, t.ID, intent.ID)
		e.stampFirstRun(t) // First real implementation → Guy. first_run_at + Fine. deadline(Only with timeout Tasks)
		e.touch(t.ID)
		emit := func(r db.Activity) { e.emitActivity(t, r) }
		ectx := e.clockCtx(e.execContextFor(ctx, t.ID), t, false) // cancellable by Pause; Tasks deadline
		// per-work child context so the planner's kill_work can stop just this work.
		workCtx, workCancel := context.WithCancelCause(ectx)
		e.registerWork(intent.ID, workCancel)
		// wrap the guard hooks so steer_work can inject a mid-run course-correction
		// for THIS intent (drained before the worker's next tool call).
		iid := intent.ID
		taskEmit := func(a db.Activity) {
			nid := iid
			a.NodeID, a.Worker = &nid, name
			emit(a)
		}
		workCtx = intercept.WithTaskContext(workCtx, t.ID, fmt.Sprintf("%s · #%d", name, iid), taskEmit)
		hooks := steerHooks{inner: t.Guard.Hooks(), drain: func() (string, bool) { return e.drainSteer(iid) }}
		wTaskID, _ := strconv.ParseInt(t.ID, 10, 64)
		e.incInflight(t.ID) // Counting on running.,For closing order drain Wait
		reason, wrote, err := worker.Execute(workCtx, name, wTaskID, e.m.assets, t.Store, intent, hooks, emit, e.m.enrich)
		// model_error End. → A few extra reruns (excuse and try again). It's just the intention. work, Tasks
		// Not suspended/Not terminated/No retrying [and not ending]; otherwise relinquishing to the corresponding branch(No closing period.
		// Try again.,Avoid shunning others. worker An elegant closing window).
		for attempt := 1; attempt <= modelErrorRetries &&
			reason == harness.ReasonModelError &&
			workCtx.Err() == nil && ectx.Err() == nil && !e.IsPaused(t.ID) && !e.isSettling(t.ID); attempt++ {
			log.Printf("[worker %s] task %s Intention #%d model_error It's over.%v Try again after (%d/%d)",
				name, t.ID, intent.ID, modelErrorRetryBackoff, attempt, modelErrorRetries)
			if sleepCtx(workCtx, modelErrorRetryBackoff) {
				break // Expiration (termination)/Pause)→ Leave it to the bottom branch.
			}
			reason, wrote, err = worker.Execute(workCtx, name, wTaskID, e.m.assets, t.Store, intent, hooks, emit, e.m.enrich)
		}
		e.decInflight(t.ID)
		// CAPTURE kill state BEFORE unregisterWork cancels workCtx. kill = this work's
		// ctx was cancelled (planner kill_work) while the TASK ctx kept running; a
		// pause cancels the task ctx (ectx) instead. Checking workCtx.Err() AFTER
		// unregister would always be true (unregister cancels it) → every completed
		// work would be wrongly marked stopped.
		killed := workCtx.Err() != nil && ectx.Err() == nil
		e.unregisterWork(intent.ID)
		// if a pause cancelled this run mid-flight, return the intent to the frontier
		// so it is re-claimed (and re-run from scratch) on resume — not marked done.
		if ectx.Err() != nil && e.IsPaused(t.ID) {
			_ = t.Store.SetIntentState(intent.ID, "open")
			continue
		}
		// Hard end on timeout: cancel (not pause, not kill) the cancelled worker run, and report the exhausted attribution as ended,
		// Don't miss the mark. blocked. At this point, worker Usually already. settlement This phase returns the results.
		if ectx.Err() != nil && e.isSettling(t.ID) {
			_ = t.Store.SetIntentState(intent.ID, "exhausted")
			log.Printf("[worker %s] task %s Intention #%d Because the job is over and done with.(exhausted),Write back %s", name, t.ID, intent.ID, wrote)
			e.touch(t.ID)
			continue
		}
		// The task is complete (done through the normal path), so cancel the cancelled worker run; the intent no longer has any meaning.
		// Indicators stopped(Nope. blocked),Do not contaminate the state of intent of the mission.
		if ectx.Err() != nil && isTerminalStatus(e.m.TaskStatus(t.ID)) {
			_ = t.Store.SetIntentState(intent.ID, "stopped")
			log.Printf("[worker %s] task %s Intention #%d Canceled due to completion of task(stopped)", name, t.ID, intent.ID)
			e.touch(t.ID)
			continue
		}
		// killed by the planner: mark stopped (don't write back results, don't auto-reclaim).
		if killed {
			_ = t.Store.SetIntentState(intent.ID, "stopped")
			log.Printf("[worker %s] task %s Intention #%d Terminated(stopped)", name, t.ID, intent.ID)
			e.touch(t.ID)
			t.Notify()
			continue
		}
		if err != nil {
			log.Printf("[worker %s] intent %d: %v", name, intent.ID, err)
		}
		// terminalDiversion: maximum number of bumps ≠ Done.max_turns→exhausted(That's how the planners know this direction.
		// Tried but not really finished, needed to change angles, not as covered permanently; error→blocked;Normal→done.
		state := "done"
		switch {
		case err != nil:
			state = "blocked"
		case reason == harness.ReasonMaxTurns:
			state = "exhausted"
			log.Printf("[worker %s] intent %d Maximum number of crashes(exhausted),This time, write back. %s", name, intent.ID, wrote)
		case reason == harness.ReasonTimeout:
			state = "exhausted"
			log.Printf("[worker %s] intent %d Run timeout(exhausted),Write back after closing. %s", name, intent.ID, wrote)
		}
		_ = t.Store.SetIntentState(intent.ID, state)
		log.Printf("[worker %s] task %s Intention #%d End: %s (Write back %s)", name, t.ID, intent.ID, state, wrote)
		e.touch(t.ID)
		t.NotifyDone(intent.ID) // results changed the graph -> wake the planner (with the just-finished intent id)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) (done bool) {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}

func (e *Engine) claimNext(t *Task, name string) *db.Node {
	fr, _ := t.Store.Frontier(20)
	for _, in := range fr {
		if ok, _ := t.Store.ClaimIntent(in.ID, name); ok {
			return in
		}
	}
	return nil
}

// plannerHooks returns the guard hooks for a task's planner. The planner runs
// with no operator watching, so it gets the unattended variant: deny rules apply
// in full and an approval rule blocks instead of stalling the planning round.
func plannerHooks(t *Task) harness.HookRunner {
	if t == nil || t.Guard == nil {
		return nil // a typed-nil registry would be a non-nil interface and crash the harness
	}
	return t.Guard.UnattendedHooks()
}
