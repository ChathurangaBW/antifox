package db

import (
	"strings"
	"testing"
	"time"
)

func assetTaskIDs(t *testing.T, d *DB, id int64) string {
	t.Helper()
	var s string
	if err := d.QueryRow(`SELECT array_to_string(task_ids, ',') FROM assets WHERE id=$1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestAssetTaskRefTrigger: the database refuses a new reference to a task that
// does not exist, on insert and on update, while a row that already holds an
// old dangling id can still be updated.
func TestAssetTaskRefTrigger(t *testing.T) {
	d, _, _ := testSetup(t)
	t.Cleanup(func() { d.Close() })
	task := testTask(t, d, 1)
	t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM assets WHERE domain LIKE 'taskref-%.test'`) })

	_, err := d.Exec(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-a.test','taskref-a.test','{987654321}')`)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("insert with a missing task: got %v", err)
	}

	var id int64
	if err := d.QueryRow(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-b.test','taskref-b.test',ARRAY[$1::bigint]) RETURNING id`, task).Scan(&id); err != nil {
		t.Fatalf("insert with an existing task: %v", err)
	}
	if _, err := d.Exec(`UPDATE assets SET task_ids = task_ids || 987654321::bigint WHERE id=$1`, id); err == nil {
		t.Fatal("update adding a missing task was accepted")
	}
	if _, err := d.Exec(`UPDATE assets SET task_ids = task_ids || NULL::bigint WHERE id=$1`, id); err == nil {
		t.Fatal("update adding a NULL task id was accepted")
	}

	// A row written before the trigger existed, holding a dangling id.
	if _, err := d.Exec(`ALTER TABLE assets DISABLE TRIGGER trg_assets_task_ids`); err != nil {
		t.Fatal(err)
	}
	var legacy int64
	err = d.QueryRow(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-c.test','taskref-c.test','{987654321}') RETURNING id`).Scan(&legacy)
	if _, e := d.Exec(`ALTER TABLE assets ENABLE TRIGGER trg_assets_task_ids`); e != nil {
		t.Fatal(e)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE assets SET task_ids = task_ids || $2::bigint, last_seen = now() WHERE id=$1`, legacy, task); err != nil {
		t.Fatalf("legacy row could not be updated with a valid task: %v", err)
	}
	if got := assetTaskIDs(t, d, legacy); got != "987654321,"+itoa64(task) {
		t.Fatalf("legacy row task_ids = %s", got)
	}
}

// TestDeleteTaskRemovesAssetReferences: deleting a task removes its id from
// every asset that referenced it and leaves other references alone.
func TestDeleteTaskRemovesAssetReferences(t *testing.T) {
	d, _, _ := testSetup(t)
	t.Cleanup(func() { d.Close() })
	keep := testTask(t, d, 1)
	gone, err := d.CreateTask("taskref-delete", "d", "g", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := d.QueryRow(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-d.test','taskref-d.test',ARRAY[$1::bigint,$2::bigint]) RETURNING id`, keep, gone.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteAsset(d, id) })
	if err := d.DeleteTask(gone.ID); err != nil {
		t.Fatal(err)
	}
	if got := assetTaskIDs(t, d, id); got != itoa64(keep) {
		t.Fatalf("after delete task_ids = %s, want %d", got, keep)
	}
}

// TestTaskDeleteBySQLRemovesAssetReferences: the cleanup is in the database, so
// it also holds for a task deleted outside DeleteTask: direct SQL, or an older
// application version (v2.1.2 deletes the rows without touching task_ids) run
// against this schema after a rollback.
func TestTaskDeleteBySQLRemovesAssetReferences(t *testing.T) {
	d, _, _ := testSetup(t)
	t.Cleanup(func() { d.Close() })
	keep := testTask(t, d, 1)
	gone, err := d.CreateTask("taskref-sql-delete", "d", "g", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := d.QueryRow(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-f.test','taskref-f.test',ARRAY[$1::bigint,$2::bigint,$1::bigint]) RETURNING id`, gone.ID, keep).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteAsset(d, id) })
	// The v2.1.2 DeleteTask statements, verbatim.
	if _, err := d.Exec(`DELETE FROM tasks WHERE id=$1`, gone.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`DELETE FROM explorations WHERE id=$1`, gone.ExplorationID); err != nil {
		t.Fatal(err)
	}
	if got := assetTaskIDs(t, d, id); got != itoa64(keep) {
		t.Fatalf("after SQL delete task_ids = %s, want %d (every copy of the deleted id removed)", got, keep)
	}
}

// TestDeleteTaskRacesAssetWrite: an asset write that references a task while
// that task is being deleted cannot leave a dangling id. The writer holds the
// task row; the delete waits, then removes the new reference.
func TestDeleteTaskRacesAssetWrite(t *testing.T) {
	d, _, _ := testSetup(t)
	t.Cleanup(func() { d.Close() })
	task, err := d.CreateTask("taskref-race", "d", "g", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := d.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := tx.QueryRow(`INSERT INTO assets(type, domain, root_domain, task_ids) VALUES ('root_domain','taskref-e.test','taskref-e.test',ARRAY[$1::bigint]) RETURNING id`, task.ID).Scan(&id); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteAsset(d, id) })

	done := make(chan error, 1)
	go func() { done <- d.DeleteTask(task.ID) }()
	select {
	case err := <-done:
		_ = tx.Rollback()
		t.Fatalf("delete did not wait for the in-flight asset write (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("delete did not finish after the asset write committed")
	}
	if got := assetTaskIDs(t, d, id); got != "" {
		t.Fatalf("dangling reference left after the race: %s", got)
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
