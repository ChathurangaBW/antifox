package server

// Gaps: From Task agent Recording Agent Synthesis + workers)Inline AI Ripping described but
// Never. report_finding We need to clear the hole. finding Node. Because of the map./Found Page/Report
// All read the same. finding Node, three copies at a time.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"antifox/db"
	"antifox/report"
)

// POST /api/tasks/{id}/extract-findings — back-fill findings from the transcript.
func (s *Server) extractFindings(w http.ResponseWriter, r *http.Request) {
	t := s.m.ResolveTask(r.PathValue("id"))
	if t == nil {
		writeErr(w, 404, "no active task")
		return
	}
	expID := t.Store.ID()
	transcript := s.m.pg.ExplorationSynthesis(expID, 48000)
	if strings.TrimSpace(transcript) == "" {
		writeErr(w, 400, "An unanalysed session record for this task")
		return
	}

	// Attach the [facts] recorded in the exploration diagram worker Interface found/Exposure)——We'll miss the conversation.
	// Use record_fact Write it back, but it didn't work out. finding Exposure/Information leak, Jean."Combination"You can raise them.
	if facts, ferr := t.Store.ListByKind(db.KindFact, 200); ferr == nil && len(facts) > 0 {
		var fb strings.Builder
		fb.WriteString("\n\n[Discovery chart documented facts (possibly including exposure not upgraded to discovery)/♪ The face of the attack ♪\n")
		for _, n := range facts {
			var p struct {
				Summary string `json:"summary"`
				Detail  string `json:"detail"`
			}
			_ = json.Unmarshal(n.Payload, &p)
			sm := strings.TrimSpace(p.Summary)
			if sm == "" {
				continue
			}
			fb.WriteString("- ")
			fb.WriteString(sm)
			if d := strings.TrimSpace(p.Detail); d != "" {
				if rr := []rune(d); len(rr) > 200 {
					d = string(rr[:200])
				}
				fb.WriteString(":")
				fb.WriteString(d)
			}
			fb.WriteString("\n")
		}
		transcript += fb.String()
	}

	// existing findings → dedup labels for the AI + a server-side guard
	existingNodes, _ := t.Store.ListByKind(db.KindFinding, 500)
	labels := make([]string, 0, len(existingNodes))
	existingKeys := make([]string, 0, len(existingNodes))
	for _, n := range existingNodes {
		var p struct {
			VulnClass string `json:"vulnclass"`
			Summary   string `json:"summary"`
		}
		_ = json.Unmarshal(n.Payload, &p)
		labels = append(labels, strings.TrimSpace(p.VulnClass+" — "+p.Summary))
		existingKeys = append(existingKeys, normalizeVuln(p.VulnClass+p.Summary))
	}

	extracted, err := report.ExtractFindings(s.ctx, s.m.pg, transcript, labels)
	if err != nil {
		writeErr(w, 502, "AI Ripping failed:"+err.Error())
		return
	}

	taskID, _ := strconv.ParseInt(t.ID, 10, 64)
	begin := s.m.pg.ExplorationBeginNode(expID)
	created := 0
	for _, e := range extracted {
		if strings.TrimSpace(e.Summary) == "" && strings.TrimSpace(e.VulnClass) == "" {
			continue
		}
		key := normalizeVuln(e.VulnClass + e.Summary)
		if containsSimilar(existingKeys, key) {
			continue // already recorded
		}
		payload := map[string]any{
			"vulnclass": e.VulnClass, "severity": normalizeSeverity(e.Severity), "summary": e.Summary,
			"evidence": map[string]any{"by": "Supplementary", "poc": e.Evidence},
		}
		id, aerr := t.Store.AddNode(db.KindFinding, payload, 9, "confirmed", "Supplementary", nil)
		if aerr != nil {
			continue
		}
		if begin > 0 {
			_ = t.Store.Link(begin, db.RelYields, id) // connect into the graph
		}
		_, _ = t.Store.AddStandaloneFinding(taskID, id, e.VulnClass, normalizeSeverity(e.Severity), e.Summary, e.Evidence, "Supplementary", nil)
		existingKeys = append(existingKeys, key)
		created++
	}
	writeJSON(w, 200, map[string]any{"extracted": len(extracted), "created": created})
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "serious", "severe", "high", "high risk", "高危", "高", "严重", "嚴重":
		return "high" // AntiFox severity set is high|medium|low
	case "medium", "medium risk", "moderate", "med", "中危", "中", "it's dangerous.":
		return "medium"
	case "low", "low risk", "low risk.", "低危", "低", "info", "information", "informational", "信息", "資訊":
		return "low"
	}
	return "medium"
}

func normalizeVuln(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 { // keep CJK
			b.WriteRune(r)
		}
	}
	return b.String()
}

// containsSimilar reports whether key strongly overlaps any existing key (either
// contains the other on a ≥8-char prefix) — a light dedup on top of the AI's.
func containsSimilar(existing []string, key string) bool {
	if key == "" {
		return false
	}
	for _, e := range existing {
		if e == "" {
			continue
		}
		if strings.Contains(e, key) || strings.Contains(key, e) {
			return true
		}
		n := 12
		if len([]rune(key)) >= n && len([]rune(e)) >= n {
			if string([]rune(key)[:n]) == string([]rune(e)[:n]) {
				return true
			}
		}
	}
	return false
}
