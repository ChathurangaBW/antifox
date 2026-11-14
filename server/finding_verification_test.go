package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"antifox/agent"
	"antifox/db"
	actool "antifox/engine/tool"
)

type fakeBodies map[string]struct {
	body      string
	truncated bool
	err       error
}

func (f fakeBodies) ResponseBody(id string, _ int) ([]byte, bool, error) {
	b, ok := f[id]
	if !ok {
		return nil, false, errors.New("no such exchange")
	}
	return []byte(b.body), b.truncated, b.err
}

const proofBody = `{"users":[{"id":1,"email":"admin@corp.example","role":"admin"}],
  "total": 1}`

func TestAssessVerification(t *testing.T) {
	ok := fakeBodies{"e1": {body: proofBody}}
	cases := []struct {
		name   string
		tr     fakeBodies
		ids    []string
		poe    int
		quotes []string
		want   string
		note   string
	}{
		{"verified", ok, []string{"e1"}, 3, []string{`"email":"admin@corp.example"`}, db.VerificationVerified, ""},
		// The body has "],\n  \"total\"": the quote's different run of whitespace still matches.
		{"whitespace normalized", ok, []string{"e1"}, 4, []string{"\"role\":\"admin\"}],   \n \"total\": 1}"}, db.VerificationVerified, ""},
		{"no linked traffic", ok, nil, 4, []string{`"email":"admin@corp.example"`}, db.VerificationUnverified, "no recorded traffic"},
		{"poe too low", ok, []string{"e1"}, 2, []string{`"email":"admin@corp.example"`}, db.VerificationUnverified, "below L3"},
		{"no quotes", ok, []string{"e1"}, 3, nil, db.VerificationUnverified, "no evidence quotes"},
		{"quote too short", ok, []string{"e1"}, 3, []string{"admin"}, db.VerificationUnverified, "shorter than"},
		{"quote absent", ok, []string{"e1"}, 3, []string{`"role":"superuser"`}, db.VerificationUnverified, "quote not found"},
		{"one of two quotes absent", ok, []string{"e1"}, 3, []string{`"email":"admin@corp.example"`, `"password":"hunter22"`}, db.VerificationUnverified, "quote not found"},
		{"unreadable body fails closed", fakeBodies{"e1": {err: errors.New("decode br body")}}, []string{"e1"}, 3, []string{`"email":"admin@corp.example"`}, db.VerificationNotAssessed, "unreadable"},
		{"binary body", fakeBodies{"e1": {body: "\x00\x01\x02 binary"}}, []string{"e1"}, 3, []string{`"email":"admin@corp.example"`}, db.VerificationNotAssessed, "not text"},
		{"truncated body", fakeBodies{"e1": {body: `{"users":[`, truncated: true}}, []string{"e1"}, 3, []string{`"email":"admin@corp.example"`}, db.VerificationNotAssessed, "read limit"},
		{"quote in second body", fakeBodies{"e1": {body: "nothing here"}, "e2": {body: proofBody}}, []string{"e1", "e2"}, 3, []string{`"email":"admin@corp.example"`}, db.VerificationVerified, ""},
	}
	for _, c := range cases {
		got, note := assessVerification(c.tr, c.ids, c.poe, c.quotes)
		if got != c.want || (c.note != "" && !strings.Contains(note, c.note)) {
			t.Errorf("%s: got %s (%q), want %s containing %q", c.name, got, note, c.want, c.note)
		}
	}
}

// TestFindingVerificationEndToEnd drives report_finding through a task-bound
// ToolSet wired to the real verifier, then checks storage, the tool reply, the
// list API overlay, and the edit-preserving schema upgrade.
func TestFindingVerificationEndToEnd(t *testing.T) {
	m, err := NewManager(t.TempDir(), "") // proxy "" = no traffic store
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	s := &Server{m: m}
	task, err := m.pg.CreateTask("t-verify", "desc", "goal", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.pg.Exec(`DELETE FROM findings WHERE task_id=$1`, task.ID)
		_, _ = m.pg.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)
		_, _ = m.pg.Exec(`DELETE FROM explorations WHERE id=$1`, task.ExplorationID)
	})

	tsx := agent.NewToolSet(m.pg.Exploration(task.ExplorationID), "work#1")
	tsx.SetTaskID(task.ID)
	tsx.SetFindingVerifier(s.recordFindingVerification)
	var rf actool.CoreTool
	for _, tl := range tsx.WorkerTools() {
		if tl.Name() == "report_finding" {
			rf = tl
		}
	}
	if rf == nil {
		t.Fatal("report_finding missing from worker tools")
	}
	in, _ := json.Marshal(map[string]any{
		"vulnclass": "idor", "severity": "high", "summary": "user record readable",
		"evidence": "GET /api/users/1", "poe_level": 3,
		"evidence_quotes": []string{`"email":"admin@corp.example"`},
	})
	res, err := rf.Call(context.Background(), in, &actool.ToolContext{})
	if err != nil || res.IsError {
		t.Fatalf("report_finding failed: %v %s", err, res.Flatten())
	}
	out := res.Flatten()
	if !strings.HasPrefix(out, "finding recorded: ") || !strings.Contains(out, "verification: not_assessed") {
		t.Fatalf("tool reply should keep its prefix and report the verdict, got %q", out)
	}
	var nodeID int64
	if _, err := fmt.Sscanf(out, "finding recorded: %d", &nodeID); err != nil || nodeID <= 0 {
		t.Fatalf("could not read node id from %q", out)
	}

	v, err := m.pg.GetFindingVerification(nodeID)
	if err != nil || v == nil {
		t.Fatalf("no stored verification: %v", err)
	}
	if v.POELevel != 3 || len(v.Quotes) != 1 || v.Status != db.VerificationNotAssessed || !strings.Contains(v.Note, "traffic recording is off") {
		t.Fatalf("stored verification = %+v", v)
	}
	dtos := s.mergeEnrichmentSummary([]FindingDTO{{NodeID: fmt.Sprint(nodeID)}})
	if dtos[0].VerificationStatus != db.VerificationNotAssessed || dtos[0].POELevel != 3 {
		t.Fatalf("list overlay = %+v", dtos[0])
	}

	// Schema upgrade on an existing database: the stored row predates the new
	// properties and carries an operator edit. The edit must survive and the new
	// properties must be added exactly once.
	if orig, _ := m.pg.GetTool("report_finding"); orig != nil {
		t.Cleanup(func() { _ = m.pg.UpsertToolForce(orig.Key, orig.Description, orig.Schema, mustJSON(orig.Agents)) })
	} else {
		t.Cleanup(func() { _, _ = m.pg.Exec(`DELETE FROM tools WHERE key='report_finding'`) })
		for _, sd := range agent.BuiltinToolSeeds() {
			if sd.Key == "report_finding" {
				sch, _ := json.Marshal(sd.Schema)
				if err := m.pg.SeedTool(sd.Key, sd.Desc, sch, mustJSON(sd.Agents)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	_, _ = m.pg.Exec(`UPDATE tools SET description='OPERATOR EDIT', schema=schema #- '{properties,poe_level}' #- '{properties,evidence_quotes}' WHERE key='report_finding'`)
	s.seedFindingVerificationSchema()
	s.seedFindingVerificationSchema()
	after, _ := m.pg.GetTool("report_finding")
	var sch struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	_ = json.Unmarshal(after.Schema, &sch)
	if after.Description != "OPERATOR EDIT" || sch.Properties["poe_level"] == nil || sch.Properties["evidence_quotes"] == nil {
		t.Fatalf("upgrade result: desc=%q props=%v", after.Description, len(sch.Properties))
	}
}
