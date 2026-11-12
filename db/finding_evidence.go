package db

import (
	"strconv"
	"strings"
	"time"
)

// Gap Evidence Interception Data Layer. FileBook dataDir/report_assets/evidence/<node_id>/,This table contains only metadata.
// Here. exploration Nodes id with finding_enrichment aligning;discovery pages / Report / MD Shared.

type FindingEvidence struct {
	ID        int64     `json:"id"`
	NodeID    int64     `json:"node_id"`
	Filename  string    `json:"filename"`
	Caption   string    `json:"caption"`
	StepIndex int       `json:"step_index"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateFindingEvidence A new record of evidence (document has been dropped),sort_order Automatically take current maximum+1.
func (d *DB) CreateFindingEvidence(nodeID int64, filename, caption string, stepIndex int) (*FindingEvidence, error) {
	var next int
	_ = d.QueryRow(`SELECT COALESCE(MAX(sort_order),0)+1 FROM finding_evidence WHERE node_id=$1`, nodeID).Scan(&next)
	var e FindingEvidence
	err := d.QueryRow(`
INSERT INTO finding_evidence(node_id, filename, caption, step_index, sort_order)
VALUES ($1,$2,$3,$4,$5)
RETURNING id, node_id, filename, caption, step_index, sort_order, created_at`,
		nodeID, filename, strings.TrimSpace(caption), stepIndex, next).
		Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListFindingEvidence returns the full evidence of a particular finding, in the order it was recorded.
func (d *DB) ListFindingEvidence(nodeID int64) ([]FindingEvidence, error) {
	rows, err := d.Query(`
SELECT id, node_id, filename, caption, step_index, sort_order, created_at
FROM finding_evidence WHERE node_id=$1 ORDER BY step_index, sort_order, id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FindingEvidence
	for rows.Next() {
		var e FindingEvidence
		if err := rows.Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountFindingEvidence Batch the number of individual evidence intercepts of a set of loopholes (as shown in the list on the discovery page)"Evidence N Chang!").
// node id Yes int64, Direct interlocking IN,No risk of injection.
func (d *DB) CountFindingEvidence(nodeIDs []int64) map[int64]int {
	out := map[int64]int{}
	if len(nodeIDs) == 0 {
		return out
	}
	// Full parameters $1,$2,...); even a node id is an int64, so there is no SQL-injection risk.
	parts := make([]string, len(nodeIDs))
	args := make([]any, len(nodeIDs))
	for i, id := range nodeIDs {
		parts[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := d.Query(`SELECT node_id, count(*) FROM finding_evidence WHERE node_id IN (`+strings.Join(parts, ",")+`) GROUP BY node_id`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var n int64
		var c int
		if rows.Scan(&n, &c) == nil {
			out[n] = c
		}
	}
	return out
}

// GetFindingEvidence Take a single bar (file name required when deleted + Verify node (c) Attribution.
func (d *DB) GetFindingEvidence(id int64) (*FindingEvidence, error) {
	var e FindingEvidence
	err := d.QueryRow(`
SELECT id, node_id, filename, caption, step_index, sort_order, created_at
FROM finding_evidence WHERE id=$1`, id).
		Scan(&e.ID, &e.NodeID, &e.Filename, &e.Caption, &e.StepIndex, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// UpdateFindingEvidence Diagram Note / Corresponding steps / Sort.
func (d *DB) UpdateFindingEvidence(id int64, caption string, stepIndex, sortOrder int) error {
	_, err := d.Exec(`UPDATE finding_evidence SET caption=$1, step_index=$2, sort_order=$3 WHERE id=$4`,
		strings.TrimSpace(caption), stepIndex, sortOrder, id)
	return err
}

// DeleteFindingEvidence Delete a record (name of the file to which the caller returns).
func (d *DB) DeleteFindingEvidence(id int64) error {
	_, err := d.Exec(`DELETE FROM finding_evidence WHERE id=$1`, id)
	return err
}
