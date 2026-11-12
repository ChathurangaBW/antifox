package db

import (
	"strconv"
	"strings"
)

// Hole↔Flow-related data layers. Association in PG(Table),The metadata are in the flow library.(SQLite,See traffic Package).
// LinkFindingTraffic Reusable: flow pages manually associated Pass source="manual";Autorecord when mission engine digs Pass "auto".

// LinkFindingTraffic links a group of traffic exchanges to a finding (linking the same traffic again is not an error).
func (d *DB) LinkFindingTraffic(nodeID int64, exchangeIDs []string, source string) error {
	if nodeID <= 0 || len(exchangeIDs) == 0 {
		return nil
	}
	if source == "" {
		source = "manual"
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, ex := range exchangeIDs {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO finding_traffic(node_id, exchange_id, source)
			VALUES ($1,$2,$3) ON CONFLICT (node_id, exchange_id) DO NOTHING`, nodeID, ex, source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UnlinkFindingTraffic removes the link between a finding and a traffic exchange.
func (d *DB) UnlinkFindingTraffic(nodeID int64, exchangeID string) error {
	_, err := d.Exec(`DELETE FROM finding_traffic WHERE node_id=$1 AND exchange_id=$2`, nodeID, strings.TrimSpace(exchangeID))
	return err
}

// ListFindingTrafficIDs returns the IDs of the traffic exchanges linked to a finding (in the order they were linked).
func (d *DB) ListFindingTrafficIDs(nodeID int64) ([]string, error) {
	rows, err := d.Query(`SELECT exchange_id FROM finding_traffic WHERE node_id=$1 ORDER BY created_at, exchange_id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CountFindingTraffic Batch statistics on the number of associated traffic of a group of holes (list of found pages)"Related flows N Article"). Full parameterization.
func (d *DB) CountFindingTraffic(nodeIDs []int64) map[int64]int {
	out := map[int64]int{}
	if len(nodeIDs) == 0 {
		return out
	}
	parts := make([]string, len(nodeIDs))
	args := make([]any, len(nodeIDs))
	for i, id := range nodeIDs {
		parts[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	rows, err := d.Query(`SELECT node_id, count(*) FROM finding_traffic WHERE node_id IN (`+strings.Join(parts, ",")+`) GROUP BY node_id`, args...)
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
