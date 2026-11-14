package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"antifox/agent"
	"antifox/db"
)

// Machine-checked finding verification, adapted from ARTEX S1.
//
// A finding is verified only when the agent declared proof-of-exploitation level
// 3 or higher and every evidence quote appears verbatim (after whitespace
// normalization) in the response body of an exchange linked to the finding.
// Anything else is unverified or not_assessed, with a note saying why.
//
// Adaptation: ARTEX reads bodies from its evidence store and only counts traffic
// bound with a proof/verification role. This platform links traffic to a finding
// by resolving its reproduction steps to recorded exchanges at record time (plus
// manual links in the UI), so every linked exchange is a candidate proof body.

const (
	verificationMinPOE        = 3
	verificationMinQuoteRunes = 8       // shorter normalized quotes are ignored: "x7" proves nothing
	verificationReadCap       = 4 << 20 // bytes read per linked body; a quote beyond this cannot be confirmed
	verificationNoteQuoteLen  = 60
)

// linkedBody is one linked response body, normalized for the substring check.
type linkedBody struct {
	text      string // whitespace-normalized text (only when !nonText)
	truncated bool   // body was longer than verificationReadCap
	nonText   bool   // binary or non-UTF-8 content: never claimed as a match
}

// normalizeEvidenceText trims and collapses every run of whitespace to one
// space. Case is kept.
func normalizeEvidenceText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// assessFindingVerification computes the verdict for a finding from its linked
// traffic and the agent's declared POE level and quotes. It never returns
// "verified" unless every rule passes, and fails closed to not_assessed when
// evidence cannot be read.
func (s *Server) assessFindingVerification(nodeID int64, poe int, quotes []string) (status, note string) {
	tr := s.m.Traffic()
	if tr == nil {
		return db.VerificationNotAssessed, "traffic recording is off, so there is no linked evidence to check"
	}
	ids, err := s.m.pg.ListFindingTrafficIDs(nodeID)
	if err != nil {
		return db.VerificationNotAssessed, "could not read linked traffic: " + err.Error()
	}
	return assessVerification(tr, ids, poe, quotes)
}

// responseBodyReader is what the verdict needs from the traffic store.
type responseBodyReader interface {
	ResponseBody(id string, limit int) ([]byte, bool, error)
}

// assessVerification is the pure verdict over linked exchange ids. It never
// returns "verified" unless every rule passes.
func assessVerification(tr responseBodyReader, ids []string, poe int, quotes []string) (status, note string) {
	if len(ids) == 0 {
		return db.VerificationUnverified, "no recorded traffic is linked to this finding"
	}
	if poe < verificationMinPOE {
		return db.VerificationUnverified, fmt.Sprintf("declared POE level L%d is below L%d", poe, verificationMinPOE)
	}
	var checked []string
	for _, q := range quotes {
		if utf8.RuneCountInString(normalizeEvidenceText(q)) >= verificationMinQuoteRunes {
			checked = append(checked, q)
		}
	}
	if len(quotes) == 0 {
		return db.VerificationUnverified, "no evidence quotes provided"
	}
	if len(checked) == 0 {
		return db.VerificationUnverified, fmt.Sprintf("every evidence quote is shorter than %d characters", verificationMinQuoteRunes)
	}
	bodies := make([]linkedBody, 0, len(ids))
	for _, id := range ids {
		raw, truncated, err := tr.ResponseBody(id, verificationReadCap)
		if err != nil {
			// Fail closed: unreadable evidence is never an "unverified" verdict.
			return db.VerificationNotAssessed, "linked response body unreadable: " + err.Error()
		}
		lb := linkedBody{truncated: truncated}
		if truncated {
			// The cap may cut a multi-byte rune: drop at most UTFMax-1 trailing bytes.
			for i := 0; i < utf8.UTFMax-1 && !utf8.Valid(raw) && len(raw) > 0; i++ {
				raw = raw[:len(raw)-1]
			}
		}
		if bytes.IndexByte(raw, 0) >= 0 || !utf8.Valid(raw) {
			lb.nonText = true
		} else {
			lb.text = normalizeEvidenceText(string(raw))
		}
		bodies = append(bodies, lb)
	}
	for _, q := range checked {
		if quoteInBodies(normalizeEvidenceText(q), bodies) {
			continue
		}
		for _, b := range bodies {
			if b.nonText {
				return db.VerificationNotAssessed, "a linked response body is not text, so the quote could not be checked"
			}
		}
		for _, b := range bodies {
			if b.truncated {
				return db.VerificationNotAssessed, "quote not found within the read limit of the linked evidence"
			}
		}
		return db.VerificationUnverified, "quote not found in linked evidence: " + shortQuote(q)
	}
	return db.VerificationVerified, ""
}

func quoteInBodies(nq string, bodies []linkedBody) bool {
	for _, b := range bodies {
		if !b.nonText && strings.Contains(b.text, nq) {
			return true
		}
	}
	return false
}

// shortQuote returns the first verificationNoteQuoteLen runes of the trimmed
// quote, for use in a note.
func shortQuote(q string) string {
	r := []rune(strings.TrimSpace(q))
	if len(r) > verificationNoteQuoteLen {
		r = append(r[:verificationNoteQuoteLen], '…')
	}
	return string(r)
}

// recordFindingVerification stores the agent's declared POE level and quotes,
// assesses the finding, and stores the verdict. It is the host callback wired
// into report_finding, which runs after the reproduction steps are linked to
// traffic. The verdict goes back to the agent so it can strengthen the evidence.
func (s *Server) recordFindingVerification(nodeID int64, poe int, quotes []string) (status, note string) {
	if s.m == nil || s.m.pg == nil || nodeID <= 0 {
		return db.VerificationNotAssessed, "no database"
	}
	if err := s.m.pg.SetFindingVerificationInput(nodeID, poe, quotes); err != nil {
		return db.VerificationNotAssessed, err.Error()
	}
	status, note = s.assessFindingVerification(nodeID, poe, quotes)
	if err := s.m.pg.SetFindingVerificationResult(nodeID, status, note); err != nil {
		log.Printf("[verification] store verdict for finding node %d: %v", nodeID, err)
	}
	return status, note
}

// reverifyFinding re-runs the check from the stored POE level and quotes, after
// an operator links or unlinks traffic on the finding page.
func (s *Server) reverifyFinding(nodeID int64) {
	v, err := s.m.pg.GetFindingVerification(nodeID)
	if err != nil || v == nil {
		return
	}
	status, note := s.assessFindingVerification(nodeID, v.POELevel, v.Quotes)
	if err := s.m.pg.SetFindingVerificationResult(nodeID, status, note); err != nil {
		log.Printf("[verification] store verdict for finding node %d: %v", nodeID, err)
	}
}

// seedFindingVerificationSchema adds the optional poe_level / evidence_quotes
// properties to the stored report_finding schema of existing databases. The
// stored schema overrides the code schema, so without this the model would never
// see the new fields. Idempotent and edit-preserving (ported from ARTEX): a
// property is added only when the stored schema lacks it, nothing else changes.
func (s *Server) seedFindingVerificationSchema() {
	for _, seed := range agent.BuiltinToolSeeds() {
		if seed.Key != "report_finding" {
			continue
		}
		raw, err := json.Marshal(seed.Schema)
		if err != nil {
			return
		}
		var obj struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if json.Unmarshal(raw, &obj) != nil {
			return
		}
		for _, name := range []string{"poe_level", "evidence_quotes"} {
			prop := obj.Properties[name]
			if len(prop) == 0 {
				continue
			}
			if _, err := s.m.pg.Exec(`UPDATE tools SET schema=jsonb_set(schema, ARRAY['properties',$2::text], $3::jsonb, true), updated_at=now()
WHERE key=$1 AND system AND NOT (COALESCE(schema->'properties','{}'::jsonb) ? $2)`, "report_finding", name, string(prop)); err != nil {
				log.Printf("[verification] add report_finding.%s: %v", name, err)
			}
		}
		return
	}
}
