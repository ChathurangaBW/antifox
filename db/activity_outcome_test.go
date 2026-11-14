package db

import "testing"

// TestActivityOutcomeRoundTrip: the run outcome is stored and read back on both
// activity tables, and rows without one read as empty.
func TestActivityOutcomeRoundTrip(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	t.Cleanup(func() { d.Close() })

	task, err := d.CreateTask("t-outcome", "d", "g", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
		_, _ = d.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)
	})
	es := d.Exploration(task.ExplorationID)
	if _, err := es.AppendActivity(Activity{Kind: "result", Summary: "x", Outcome: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	if _, err := es.AppendActivity(Activity{Kind: "text", Summary: "y"}); err != nil {
		t.Fatal(err)
	}
	acts, _, err := es.ActivityList(nil, 0, 10)
	if err != nil || len(acts) != 2 || acts[0].Outcome != "cancelled" || acts[1].Outcome != "" {
		t.Fatalf("activity outcomes = %+v err=%v", acts, err)
	}

	conv, err := d.CreateConversation("auto", "outcome-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.DeleteConversation(conv.ID) })
	if _, err := d.AppendConvActivity(conv.ID, Activity{Kind: "result", Summary: "z", Outcome: "failed", IsError: true}); err != nil {
		t.Fatal(err)
	}
	cacts, _, err := d.ConvActivityList(conv.ID, 0, 10)
	if err != nil || len(cacts) != 1 || cacts[0].Outcome != "failed" {
		t.Fatalf("conversation outcome = %+v err=%v", cacts, err)
	}
}
