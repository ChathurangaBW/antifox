package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// DBFinding is a row in the standalone findings table (persists across task deletion).
type DBFinding struct {
	ID              int64
	TaskID          *int64
	NodeID          *int64
	VulnClass       string
	Severity        string
	Summary         string
	Evidence        string
	Worker          string
	AssetIDs        []int64
	CreatedAt       time.Time
	TaskDescription string          // populated via LEFT JOIN on tasks
	Reproduction    json.RawMessage // agent Structural reproduction of local records(Empty);null=None(We'll go to the bottom.)
}

// AddFinding inserts a finding into the standalone findings table. taskID and
// nodeID may be 0 (stored as NULL). Returns the new finding id.
func (d *DB) AddFinding(taskID, nodeID int64, vulnclass, severity, summary, evidence, worker string, assetIDs []int64) (int64, error) {
	aidsJSON, _ := json.Marshal(assetIDs)
	if assetIDs == nil {
		aidsJSON = []byte("[]")
	}
	var tid, nid *int64
	if taskID > 0 {
		tid = &taskID
	}
	if nodeID > 0 {
		nid = &nodeID
	}
	var id int64
	err := d.QueryRow(
		`INSERT INTO findings (task_id, node_id, vulnclass, severity, summary, evidence, worker, asset_ids)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		tid, nid, vulnclass, severity, summary, evidence, worker, string(aidsJSON),
	).Scan(&id)
	return id, err
}

// SetFindingReproduction stores the agent-captured structured reproduction recipe for
// the finding at nodeID (report_finding writes it inline) and auto-links any resolved
// step exchange IDs in finding_traffic. No-op on empty input.
func (d *DB) SetFindingReproduction(nodeID int64, repro json.RawMessage) error {
	if len(repro) == 0 {
		return nil
	}
	if _, err := d.Exec(`UPDATE findings SET reproduction=$1 WHERE node_id=$2`, string(repro), nodeID); err != nil {
		return err
	}
	var rp struct {
		Steps []struct {
			ExchangeID string `json:"exchange_id"`
		} `json:"steps"`
	}
	if json.Unmarshal(repro, &rp) == nil && len(rp.Steps) > 0 {
		var eids []string
		for _, st := range rp.Steps {
			if st.ExchangeID != "" {
				eids = append(eids, st.ExchangeID)
			}
		}
		if len(eids) > 0 {
			_ = d.LinkFindingTraffic(nodeID, eids, "auto")
		}
	}
	return nil
}

// GetFindingByNodeID returns the standalone finding row for an exploration node
// id (the canonical finding key), or (nil, nil) if none.
func (d *DB) GetFindingByNodeID(nodeID int64) (*DBFinding, error) {
	f := &DBFinding{}
	var aidsJSON, repro string
	err := d.QueryRow(`
		SELECT f.id, f.task_id, f.node_id, f.vulnclass, f.severity, f.summary,
		       f.evidence, f.worker, f.asset_ids, f.created_at,
		       COALESCE(t.description, '') AS task_description, COALESCE(f.reproduction::text, '')
		FROM findings f LEFT JOIN tasks t ON f.task_id = t.id
		WHERE f.node_id = $1
		ORDER BY f.id DESC LIMIT 1`, nodeID).
		Scan(&f.ID, &f.TaskID, &f.NodeID, &f.VulnClass, &f.Severity, &f.Summary,
			&f.Evidence, &f.Worker, &aidsJSON, &f.CreatedAt, &f.TaskDescription, &repro)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(aidsJSON), &f.AssetIDs)
	if repro != "" {
		f.Reproduction = json.RawMessage(repro)
	}
	return f, nil
}

// ListFindings returns all findings (newest first), joined with task description.
func (d *DB) ListFindings(limit int) ([]*DBFinding, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := d.Query(`
		SELECT f.id, f.task_id, f.node_id, f.vulnclass, f.severity, f.summary,
		       f.evidence, f.worker, f.asset_ids, f.created_at,
		       COALESCE(t.description, '') AS task_description
		FROM findings f
		LEFT JOIN tasks t ON f.task_id = t.id
		ORDER BY f.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*DBFinding{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		f := &DBFinding{}
		var aidsJSON string
		if err := rows.Scan(&f.ID, &f.TaskID, &f.NodeID, &f.VulnClass, &f.Severity,
			&f.Summary, &f.Evidence, &f.Worker, &aidsJSON, &f.CreatedAt, &f.TaskDescription); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(aidsJSON), &f.AssetIDs)
		out = append(out, f)
	}
	return out, rows.Err()
}
