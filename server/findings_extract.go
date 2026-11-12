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
		fb.WriteString("\n\n[Facts recorded in the exploration graph (may include exposures or attack surface not yet promoted to findings)]\n")
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
		existingKeys = append(existingKeys, vulnDedupKey(p.VulnClass, p.Summary))
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
		key := vulnDedupKey(e.VulnClass, e.Summary)
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
	case "critical", "serious", "severe", "crit", "严重", "嚴重":
		return "critical"
	case "high", "high risk", "高危", "高":
		return "high"
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

func vulnDedupKey(vulnClass, summary string) string {
	return normalizeVuln(vulnClass) + "|" + normalizeVuln(summary)
}

func splitDedupKey(k string) (string, string) {
	if c, s, ok := strings.Cut(k, "|"); ok {
		return c, s
	}
	return "", k
}

// containsSimilar reports whether key strongly overlaps any existing key without
// collapsing distinct findings that merely share the same English VulnClass prefix.
func containsSimilar(existing []string, key string) bool {
	kClass, kSum := splitDedupKey(key)
	if kClass == "" && kSum == "" {
		return false
	}
	for _, e := range existing {
		if e == "" {
			continue
		}
		if e == key {
			return true
		}
		eClass, eSum := splitDedupKey(e)
		if kSum == "" || eSum == "" {
			continue
		}
		sameOrEmptyClass := kClass == "" || eClass == "" || kClass == eClass ||
			strings.Contains(kClass, eClass) || strings.Contains(eClass, kClass)
		if !sameOrEmptyClass {
			continue
		}
		if strings.Contains(eSum, kSum) || strings.Contains(kSum, eSum) {
			return true
		}
		if containsCJK(kSum) && containsCJK(eSum) {
			const n = 10
			kr, er := []rune(kSum), []rune(eSum)
			if len(kr) >= n && len(er) >= n && string(kr[:n]) == string(er[:n]) {
				return true
			}
		}
	}
	return false
}
