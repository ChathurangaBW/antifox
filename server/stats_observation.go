package server

import (
	"sync/atomic"
	"time"
)

// taskEngineObservation is a read-only projection, not an engine-health probe.
// Executions counts planner and worker calls together; it is not a worker count.
type taskEngineObservation struct {
	Mode       string
	Status     string
	Running    bool
	Paused     bool
	Stalled    bool
	Last       int64
	Executions int64
}

func (e *Engine) observeTaskEngine(t *Task, runningIntents int, now time.Time) taskEngineObservation {
	o := taskEngineObservation{Mode: "idle", Status: t.Status, Last: e.LastActivity(t.ID)}
	// Do not use inflightCount here: that accessor creates a counter on a miss.
	if value, ok := e.inflight.Load(t.ID); ok {
		o.Executions = atomic.LoadInt64(value.(*int64))
	}
	if isTerminalStatus(t.Status) {
		return o
	}
	o.Paused = t.Paused || e.IsPaused(t.ID)
	o.Running = e.Ready() && e.Started(t.ID) && !o.Paused
	o.Stalled = o.Running && runningIntents == 0 && o.Executions == 0 && o.Last > 0 && now.Unix()-o.Last > 60
	switch {
	case o.Paused:
		o.Mode, o.Status = "paused", "paused"
	case o.Stalled:
		o.Mode, o.Status = "stalled", "running"
	case o.Running:
		o.Mode, o.Status = "exploring", "running"
	}
	return o
}
