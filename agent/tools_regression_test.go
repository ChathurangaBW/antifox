package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"antifox/db"
)

func TestAddIntentInvalidAssetIDs(t *testing.T) {
	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("t-intent-val", "desc", "goal", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
	defer d.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)
	defer d.Exec(`DELETE FROM assets WHERE ip='10.254.1.10'`)

	validID, err := d.Assets().UpsertIP(db.UpsertIPReq{IP: "10.254.1.10", TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}

	ts := NewToolSet(d.Exploration(task.ExplorationID), "planner")
	ts.SetAssetStore(d.Assets(), d.Companies())
	ts.SetTaskID(task.ID)
	tool := ts.addIntent()

	badCases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "nonexistent ids",
			body: map[string]any{"summary": "probe invalid", "asset_ids": []int64{99999991, 99999992}},
			want: "invalid asset_ids",
		},
		{
			name: "mixed valid and nonexistent ids",
			body: map[string]any{"summary": "probe mixed", "asset_ids": []int64{validID, 99999993}},
			want: "99999993",
		},
		{
			name: "zero or non-positive id",
			body: map[string]any{"summary": "probe zero", "asset_ids": []int64{0}},
			want: "positive integer",
		},
	}

	for _, tc := range badCases {
		raw, _ := json.Marshal(tc.body)
		res, err := tool.Call(context.Background(), raw, nil)
		if err != nil {
			t.Fatalf("%s: unexpected Call error: %v", tc.name, err)
		}
		txt := res.Flatten()
		if !res.IsError || !strings.Contains(txt, tc.want) {
			t.Fatalf("%s: expected tool error containing %q, got isError=%v text=%q", tc.name, tc.want, res.IsError, txt)
		}
	}

	var intentCnt, anchorCnt int
	_ = d.QueryRow(`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id=$1 AND kind='intent'`, task.ExplorationID).Scan(&intentCnt)
	_ = d.QueryRow(`SELECT COUNT(*) FROM exploration_anchors ea JOIN exploration_nodes en ON en.id=ea.node_id WHERE en.exploration_id=$1`, task.ExplorationID).Scan(&anchorCnt)
	if intentCnt != 0 || anchorCnt != 0 {
		t.Fatalf("expected 0 partial writes after invalid asset_ids, got intents=%d anchors=%d", intentCnt, anchorCnt)
	}

	// Valid asset_id succeeds cleanly.
	raw, _ := json.Marshal(map[string]any{"summary": "probe valid target", "asset_ids": []int64{validID}})
	res, err := tool.Call(context.Background(), raw, nil)
	if err != nil || res.IsError {
		t.Fatalf("valid add_intent failed: err=%v isError=%v out=%s", err, res.IsError, res.Flatten())
	}
	_ = d.QueryRow(`SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id=$1 AND kind='intent'`, task.ExplorationID).Scan(&intentCnt)
	if intentCnt != 1 {
		t.Fatalf("expected 1 intent after valid call, got %d", intentCnt)
	}
}

func TestInsertAssetsEnforcesTaskContext(t *testing.T) {
	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("t-task-ctx", "desc", "goal", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
	defer d.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)
	defer d.Exec(`DELETE FROM assets WHERE ip IN ('10.254.2.10','10.254.2.11')`)

	// Bound to task.ID: caller-supplied task_id=99999 must NOT override task.ID.
	ts := NewToolSet(d.Exploration(task.ExplorationID), "worker")
	ts.SetAssetStore(d.Assets(), d.Companies())
	ts.SetTaskID(task.ID)

	out := callInsertAssets(t, ts, map[string]any{
		"task_id": 99999,
		"assets": []any{
			map[string]any{"type": "ip", "ip": "10.254.2.10"},
		},
	})
	if errs, _ := out["errors"].([]any); len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	warn, _ := out["warning"].(string)
	if !strings.Contains(warn, "99999") {
		t.Errorf("expected warning about ignored caller task_id, got %q", warn)
	}

	var taskIDs []byte
	if err := d.QueryRow(`SELECT array_to_json(task_ids)::text FROM assets WHERE type='ip' AND ip='10.254.2.10'`).Scan(&taskIDs); err != nil {
		t.Fatal(err)
	}
	var gotIDs []int64
	_ = json.Unmarshal(taskIDs, &gotIDs)
	if len(gotIDs) != 1 || gotIDs[0] != task.ID {
		t.Fatalf("task_ids = %v, want [%d] (caller task_id must not override task context)", gotIDs, task.ID)
	}

	// Unbound ToolSet (taskID=0): caller task_id still works for standalone ingestion.
	unbound := NewToolSet(nil, "")
	unbound.SetAssetStore(d.Assets(), d.Companies())
	out2 := callInsertAssets(t, unbound, map[string]any{
		"task_id": 88888,
		"assets": []any{
			map[string]any{"type": "ip", "ip": "10.254.2.11"},
		},
	})
	if errs, _ := out2["errors"].([]any); len(errs) > 0 {
		t.Fatalf("unbound unexpected errors: %v", errs)
	}
	var gotUnbound []int64
	_ = d.QueryRow(`SELECT array_to_json(task_ids)::text FROM assets WHERE type='ip' AND ip='10.254.2.11'`).Scan(&taskIDs)
	_ = json.Unmarshal(taskIDs, &gotUnbound)
	if len(gotUnbound) != 1 || gotUnbound[0] != 88888 {
		t.Fatalf("unbound task_ids = %v, want [88888]", gotUnbound)
	}
}

func TestReportFindingValidationAndMainAgent(t *testing.T) {
	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("t-main-rf", "desc", "goal", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM findings WHERE task_id=$1`, task.ID)
	defer d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
	defer d.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)
	defer d.Exec(`DELETE FROM assets WHERE ip='10.254.3.10'`)

	assetID, err := d.Assets().UpsertIP(db.UpsertIPReq{IP: "10.254.3.10", TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}

	tsx := NewToolSet(d.Exploration(task.ExplorationID), "human")
	tsx.SetAssetStore(d.Assets(), d.Companies())
	tsx.SetTaskID(task.ID)

	var rfTool = func() (res bool) {
		for _, tl := range tsx.MainAgentTools() {
			if tl.Name() == "report_finding" {
				// Bad asset_id fails cleanly without writing a finding.
				badIn, _ := json.Marshal(map[string]any{
					"vulnclass": "exposure",
					"severity":  "high",
					"summary":   "bad asset",
					"evidence":  "curl ...",
					"asset_ids": []int64{99999994},
				})
				rBad, err := tl.Call(context.Background(), badIn, nil)
				if err != nil || !rBad.IsError {
					t.Fatalf("expected invalid asset_id error, got err=%v isError=%v out=%s", err, rBad.IsError, rBad.Flatten())
				}
				// Valid report_finding works directly on task-bound MainAgent ToolSet.
				goodIn, _ := json.Marshal(map[string]any{
					"vulnclass": "exposure",
					"severity":  "medium",
					"summary":   "confirmed endpoint leak",
					"evidence":  "HTTP/1.1 200 OK",
					"asset_ids": []int64{assetID},
				})
				rGood, err := tl.Call(context.Background(), goodIn, nil)
				if err != nil || rGood.IsError {
					t.Fatalf("mainagent report_finding failed: err=%v isError=%v out=%s", err, rGood.IsError, rGood.Flatten())
				}
				return true
			}
		}
		return false
	}()
	if !rfTool {
		t.Fatal("MainAgentTools missing report_finding")
	}

	var cnt int
	_ = d.QueryRow(`SELECT COUNT(*) FROM findings WHERE task_id=$1`, task.ID).Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("findings count = %d, want 1", cnt)
	}
}

func TestGetWorkerOutputLifecycleStates(t *testing.T) {
	// Pure state classifier checks (incl. queued defensively).
	for _, s := range []string{"open", "queued", "running", "done"} {
		if intentTerminatedState(s) {
			t.Errorf("intentTerminatedState(%q) = true, want false", s)
		}
	}
	for _, s := range []string{"stopped", "blocked", "exhausted"} {
		if !intentTerminatedState(s) {
			t.Errorf("intentTerminatedState(%q) = false, want true", s)
		}
	}

	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("t-worker-out", "desc", "goal", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
	defer d.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)

	es := d.Exploration(task.ExplorationID)
	ts := NewToolSet(es, "planner")
	tool := ts.getWorkerOutput()

	call := func(id int64) map[string]any {
		in, _ := json.Marshal(map[string]any{"intent_id": id})
		res, err := tool.Call(context.Background(), in, nil)
		if err != nil || res.IsError {
			t.Fatalf("get_worker_output(%d) error: err=%v isError=%v out=%s", id, err, res.IsError, res.Flatten())
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(res.Flatten()), &out); err != nil {
			t.Fatalf("unmarshal: %v raw=%s", err, res.Flatten())
		}
		return out
	}

	// 1. open with no activity -> terminated=false
	idOpen, err := es.AddIntent(map[string]any{"summary": "open work"}, 5, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	outOpen := call(idOpen)
	if outOpen["state"] != "open" || outOpen["terminated"] != false {
		t.Fatalf("open output = %v, want state=open terminated=false", outOpen)
	}

	// 2. running with interim text only (no result) -> terminated=false
	if err := es.SetIntentState(idOpen, "running"); err != nil {
		t.Fatal(err)
	}
	if _, err := es.AppendActivity(db.Activity{NodeID: &idOpen, Worker: "worker-1", Kind: "text", Summary: "scanning port 80", Detail: "still scanning port 80"}); err != nil {
		t.Fatal(err)
	}
	outRun := call(idOpen)
	if outRun["state"] != "running" || outRun["terminated"] != false || outRun["final_text"] != "still scanning port 80" {
		t.Fatalf("running output = %v, want state=running terminated=false interim final_text", outRun)
	}

	// 3. done with result -> terminated=false
	if _, err := es.AppendActivity(db.Activity{NodeID: &idOpen, Worker: "worker-1", Kind: "result", Summary: "done scan", Detail: "completed cleanly"}); err != nil {
		t.Fatal(err)
	}
	if err := es.SetIntentState(idOpen, "done"); err != nil {
		t.Fatal(err)
	}
	outDone := call(idOpen)
	if outDone["state"] != "done" || outDone["terminated"] != false || outDone["final_text"] != "completed cleanly" {
		t.Fatalf("done output = %v, want state=done terminated=false completed result", outDone)
	}

	// 4. stopped / blocked / exhausted without result -> terminated=true
	for _, st := range []string{"stopped", "blocked", "exhausted"} {
		iid, err := es.AddIntent(map[string]any{"summary": st + " work"}, 5, nil, "planner")
		if err != nil {
			t.Fatal(err)
		}
		if err := es.SetIntentState(iid, st); err != nil {
			t.Fatal(err)
		}
		out := call(iid)
		if out["state"] != st || out["terminated"] != true {
			t.Fatalf("%s output = %v, want state=%s terminated=true", st, out, st)
		}
	}
}
