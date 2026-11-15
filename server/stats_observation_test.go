package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antifox/agent"
	"antifox/db"
	"antifox/traffic"
)

// Exercise the actual HTTP handlers and DB decoding, replacing only SQL I/O.
// SQL dialect and PostgreSQL integration are not covered by this fixture. No
// manager/server constructor or engine loop is started, and SQL writes fail.
type telemetryConnector struct {
	fail           string
	runningIntents int
	writes         atomic.Int64
}

func (c *telemetryConnector) Connect(context.Context) (driver.Conn, error) {
	return &telemetryConn{c}, nil
}
func (c *telemetryConnector) Driver() driver.Driver { return telemetryDriver{} }

type telemetryDriver struct{}

func (telemetryDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the test connector")
}

type telemetryConn struct{ c *telemetryConnector }

func (c *telemetryConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}
func (c *telemetryConn) Close() error { return nil }
func (c *telemetryConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (c *telemetryConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.c.writes.Add(1)
	return nil, errors.New("telemetry must not write")
}

func (c *telemetryConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var key string
	var values [][]driver.Value
	switch {
	case strings.Contains(query, "FROM rbac_users WHERE id="):
		key, values = "auth_user", [][]driver.Value{{"fixture", "fixture", "Fixture", "unused", true, false, int64(0), time.Unix(100, 0), time.Unix(100, 0)}}
	case strings.Contains(query, "FROM rbac_roles r JOIN"):
		key, values = "auth_roles", [][]driver.Value{{"fixture", "Fixture", "Test role", "all", false, time.Unix(100, 0), time.Unix(100, 0)}}
	case strings.Contains(query, "SELECT DISTINCT rp.permission_key"):
		key, values = "auth_permissions", [][]driver.Value{{"dashboard:read"}, {"task:read"}, {"task:write"}}
	case strings.Contains(query, "SELECT value FROM settings"):
		key, values = "auth_settings", [][]driver.Value{{"true"}}
	case strings.Contains(query, "FROM assets GROUP BY type"):
		key, values = "assets", [][]driver.Value{{"host", int64(1)}}
	case strings.Contains(query, "GROUP BY kind"):
		key, values = "exploration", [][]driver.Value{{"goal", int64(2)}}
	case strings.Contains(query, "kind=$2"):
		key = fmt.Sprint(args[1].Value)
		if key != "intent" && key != "goal" {
			return nil, fmt.Errorf("unexpected node kind %q", key)
		}
		states := []string{"open"}
		if key == "goal" {
			states = []string{"met", "open"}
		} else {
			for i := 0; i < c.c.runningIntents; i++ {
				states = append(states, "running")
			}
		}
		for i, state := range states {
			values = append(values, []driver.Value{int64(i + 1), key, []byte(`{}`), int64(1), state, "", "", time.Unix(100, 0)})
		}
	case strings.Contains(query, "SUM(input_tokens)"):
		key, values = "tokens", [][]driver.Value{{int64(5), int64(10), int64(20), int64(0), int64(0)}}
	case strings.Contains(query, "EXTRACT(EPOCH"):
		key, values = "activity", [][]driver.Value{{int64(5), int64(100)}}
	case strings.Contains(query, "COUNT(*) FILTER"):
		key, values = "task_goals", [][]driver.Value{{int64(5), int64(2), int64(1)}}
	default:
		return nil, fmt.Errorf("unexpected SQL: %s", query)
	}
	if c.c.fail == key+"/query" {
		return nil, errors.New("private database failure detail")
	}
	if c.c.fail == key+"/scan" {
		// Replace a numeric column (asset/kind counts use column 1).
		column := 0
		if key == "assets" || key == "exploration" {
			column = 1
		}
		values[0][column] = "not-a-number"
	}
	rows := &telemetryRows{values: values, width: len(values[0])}
	if c.c.fail == key+"/missing" {
		rows.values = nil
	}
	if c.c.fail == key+"/rows" {
		rows.err = errors.New("private row iteration failure")
	}
	return rows, nil
}

type telemetryRows struct {
	values [][]driver.Value
	width  int
	err    error
}

func (r *telemetryRows) Columns() []string {
	columns := make([]string, r.width)
	for i := range columns {
		columns[i] = fmt.Sprintf("column_%d", i)
	}
	return columns
}
func (r *telemetryRows) Close() error { return nil }
func (r *telemetryRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		if r.err != nil {
			return r.err
		}
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}

func telemetryFixture(t *testing.T, connector *telemetryConnector) (*Server, *Task) {
	t.Helper()
	sqlDB := sql.OpenDB(connector)
	t.Cleanup(func() { _ = sqlDB.Close() })
	pg := &db.DB{DB: sqlDB}
	task := &Task{ID: "5", ExpID: 5, Status: "created", Store: pg.Exploration(5), FirstRunAt: 100, DeadlineAt: 700}
	m := &Manager{pg: pg, assets: pg.Assets(), tasks: map[string]*Task{"5": task}, active: "5"}
	// Non-nil inert objects satisfy Ready. They never receive an execution call.
	e := &Engine{planner: &agent.Planner{}, worker: &agent.Worker{}}
	e.started.Store("5", true)
	e.lastAct.Store("5", time.Now().Add(-2*time.Minute).Unix())
	t.Cleanup(func() {
		if got := connector.writes.Load(); got != 0 {
			t.Errorf("telemetry issued %d SQL writes", got)
		}
		if task.FirstRunAt != 100 || task.DeadlineAt != 700 || task.CompletedAt != 0 {
			t.Error("telemetry modified task clocks")
		}
	})
	return &Server{m: m, engine: e}, task
}

func TestStatsObservationHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, status, mode string
		executions         int64
		workers            int
		paused             bool
	}{
		{"inactive", "created", "stalled", 0, 0, false},
		{"planner_in_flight", "created", "exploring", 1, 0, false},
		{"running_worker", "created", "exploring", 0, 1, false},
		{"paused", "created", "paused", 1, 0, true},
		{"done_over_started_and_paused", "done", "idle", 1, 1, true},
		{"failed_over_started", "failed", "idle", 0, 0, false},
		{"timeout_over_started", "timeout", "idle", 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := telemetryFixture(t, &telemetryConnector{runningIntents: tc.workers})
			task.Status, task.Paused = tc.status, tc.paused
			if tc.executions > 0 {
				counter := tc.executions
				s.engine.inflight.Store("5", &counter)
			}
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				s.stats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?task=5", nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
				}
				var response struct {
					Mode string `json:"engine_mode"`
					Task struct {
						Status     string `json:"status"`
						Running    bool   `json:"running"`
						Stalled    bool   `json:"stalled"`
						Workers    int    `json:"in_flight"`
						Executions int64  `json:"executions_in_flight"`
						Goals      int    `json:"goals_total"`
						Met        int    `json:"goals_met"`
					} `json:"active_task"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Mode != tc.mode || response.Task.Stalled != (tc.mode == "stalled") {
					t.Errorf("observation = %+v; want mode %s", response, tc.mode)
				}
				if response.Task.Workers != tc.workers || response.Task.Executions != tc.executions || response.Task.Goals != 2 || response.Task.Met != 1 {
					t.Errorf("incorrect counters: %+v", response.Task)
				}
				if isTerminalStatus(tc.status) && (response.Task.Status != tc.status || response.Task.Running) {
					t.Errorf("terminal status overridden: %+v", response.Task)
				}
			}
			if _, exists := s.engine.inflight.Load("5"); exists != (tc.executions > 0) {
				t.Error("status polling created an execution counter")
			}
			if task.Status != tc.status || task.Paused != tc.paused {
				t.Error("status polling mutated the task")
			}
		})
	}
}

func TestTelemetryReadFailuresHTTP(t *testing.T) {
	for _, key := range []string{"assets", "exploration", "intent", "goal", "tokens", "activity", "task_goals"} {
		for _, failure := range []string{"query", "scan", "rows"} {
			t.Run(key+"/"+failure, func(t *testing.T) {
				s, _ := telemetryFixture(t, &telemetryConnector{fail: key + "/" + failure})
				rec := httptest.NewRecorder()
				if key == "tokens" || key == "activity" || key == "task_goals" {
					s.listTasks(rec, httptest.NewRequest(http.MethodGet, "/api/tasks", nil))
				} else {
					s.stats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?task=5", nil))
				}
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if len(body) != 1 || body["error"] == nil || strings.Contains(rec.Body.String(), "private") {
					t.Fatalf("expected only a generic error, not partial counters or private details: %s", rec.Body.String())
				}
			})
		}
	}
}

func TestStatsTrafficReadFailureHTTP(t *testing.T) {
	s, _ := telemetryFixture(t, &telemetryConnector{})
	tr, err := traffic.Open(t.TempDir(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// Never start this proxy. Closing only its disposable SQLite DB forces Count
	// to fail, without affecting any service or creating a forwarding endpoint.
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	s.m.traffic = tr
	rec := httptest.NewRecorder()
	s.stats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?task=5", nil))
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), `"traffic":0`) {
		t.Fatalf("failed traffic read presented as successful: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStatsObservationBoundary(t *testing.T) {
	s, task := telemetryFixture(t, &telemetryConnector{})
	now := time.Unix(1000, 0)
	for _, age := range []int64{-1, 0, 60, 61} {
		s.engine.lastAct.Store("5", now.Unix()-age)
		got := s.engine.observeTaskEngine(task, 0, now)
		if got.Stalled != (age > 60) {
			t.Errorf("age %d: stalled=%v", age, got.Stalled)
		}
	}
	s.engine.lastAct.Delete("5")
	if got := s.engine.observeTaskEngine(task, 0, now); got.Stalled {
		t.Error("missing timestamp inferred as stalled")
	}
	s.engine.started.Delete("5")
	if got := s.engine.observeTaskEngine(task, 0, now); got.Mode != "idle" || got.Running {
		t.Errorf("unstarted task: %+v", got)
	}
}
