package db

import (
	"encoding/json"
	"fmt"
)

// Machine-checked verification of a finding, adapted from ARTEX S1. Kept apart
// from the human disposition (enrichment status). Keyed by node_id, this
// platform's canonical finding key.
const (
	VerificationVerified    = "verified"
	VerificationUnverified  = "unverified"
	VerificationNotAssessed = "not_assessed"
)

// ValidVerificationStatus reports whether s is a known verification state.
func ValidVerificationStatus(s string) bool {
	switch s {
	case VerificationVerified, VerificationUnverified, VerificationNotAssessed:
		return true
	}
	return false
}

// FindingVerification is the verification state of one finding.
type FindingVerification struct {
	Status   string   `json:"verification_status"`
	Note     string   `json:"verification_note,omitempty"`
	POELevel int      `json:"poe_level"`
	Quotes   []string `json:"evidence_quotes,omitempty"`
}

// SetFindingVerificationInput stores what the agent declared: its POE level and
// the quotes it says appear in linked response bodies.
func (d *DB) SetFindingVerificationInput(nodeID int64, poe int, quotes []string) error {
	if poe < 0 || poe > 4 {
		return fmt.Errorf("invalid POE level %d (want 0-4)", poe)
	}
	if quotes == nil {
		quotes = []string{}
	}
	q, _ := json.Marshal(quotes)
	_, err := d.Exec(`UPDATE findings SET poe_level=$2, evidence_quotes=$3 WHERE node_id=$1`, nodeID, poe, string(q))
	return err
}

// SetFindingVerificationResult records a computed verdict.
func (d *DB) SetFindingVerificationResult(nodeID int64, status, note string) error {
	if !ValidVerificationStatus(status) {
		return fmt.Errorf("invalid verification status %q", status)
	}
	_, err := d.Exec(`UPDATE findings SET verification_status=$2, verification_note=$3 WHERE node_id=$1`, nodeID, status, note)
	return err
}

// GetFindingVerification returns the stored state for one finding, or nil when
// no finding row exists for nodeID. A row never assessed reads as unverified.
func (d *DB) GetFindingVerification(nodeID int64) (*FindingVerification, error) {
	m, err := d.FindingVerifications([]int64{nodeID})
	if err != nil {
		return nil, err
	}
	return m[nodeID], nil
}

// FindingVerifications returns the stored state for many findings, keyed by
// node id, for list views. Missing rows are absent from the map.
func (d *DB) FindingVerifications(nodeIDs []int64) (map[int64]*FindingVerification, error) {
	out := map[int64]*FindingVerification{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	ids, _ := json.Marshal(nodeIDs)
	rows, err := d.Query(`
SELECT DISTINCT ON (node_id) node_id, COALESCE(verification_status, 'unverified'), verification_note,
       poe_level, evidence_quotes::text
FROM findings
WHERE node_id = ANY(SELECT jsonb_array_elements_text($1::jsonb)::bigint)
ORDER BY node_id, id DESC`, string(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var v FindingVerification
		var quotes string
		if err := rows.Scan(&id, &v.Status, &v.Note, &v.POELevel, &quotes); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(quotes), &v.Quotes)
		out[id] = &v
	}
	return out, rows.Err()
}
