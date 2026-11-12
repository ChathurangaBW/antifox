package server

import (
	"net/http"

	"antifox/traffic"
)

// Hole↔Traffic-related interface. Association in PG,Message metadata from flow library(SQLite)Press id Parsing.
// We're moving here manually.(source=manual);Mission engines can be switched directly in the future. pg.LinkFindingTraffic(..., "auto") Automatically records.

// POST /api/findings/{id}/traffic — body {exchange_ids:[]} Link traffic to loopholes.
func (s *Server) linkFindingTraffic(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	var req struct {
		ExchangeIDs []string `json:"exchange_ids"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Request format error")
		return
	}
	if err := pg.LinkFindingTraffic(nodeID, req.ExchangeIDs, "manual"); err != nil {
		writeErr(w, 500, "Association failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "linked": len(req.ExchangeIDs)})
}

// GET /api/findings/{id}/traffic — lists the traffic associated with the finding (with metadata; the body text is fetched separately when needed).
func (s *Server) listFindingTraffic(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	ids, err := pg.ListFindingTrafficIDs(nodeID)
	if err != nil {
		writeErr(w, 500, "Failed to fetch")
		return
	}
	var metas []traffic.ExchangeMeta
	if tr := s.m.Traffic(); tr != nil && len(ids) > 0 {
		metas, _ = tr.MetaByIDs(ids)
	}
	writeJSON(w, 200, map[string]any{"exchanges": trafficDTOs(metas)})
}

// DELETE /api/findings/{id}/traffic/{eid} — Disconnect.
func (s *Server) unlinkFindingTraffic(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	if err := pg.UnlinkFindingTraffic(nodeID, r.PathValue("eid")); err != nil {
		writeErr(w, 500, "Undo Failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
