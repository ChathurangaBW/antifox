package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif" // Registration gif Codec
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"antifox/report"
)

// We're uploading evidence./Presentation/Management. File Storage dataDir/report_assets/evidence/<node_id>/(Whatever. report_assets Backup;
// The evidence page renders <img> from /api/findings/{id}/evidence/{file} (the diagrams require the same cookie) and embeds them in the report/MD; clicking a diagram opens it from the on-disk path.
//
// Security baseline (in itself a secure platform and the point of upload cannot be the entry point for malicious documents):
//   - No trust extension/Client MIME:Type determined by content + Decoding check is a true picture. + Recoding (Addression)/Embedded load,
//     EXIF, polyglot),Let's make it clean. png/jpg;
//   - Depressive bomb protection: read first DecodeConfig Validation of size caps and full decodes;
//   - Take Diagram Response Force X-Content-Type-Options: nosniff + Fixed Picture Content-Type + inline,Undo Storage XSS.

const (
	evidenceMaxBytes  = 16 << 20   // Single Chart 16MB
	evidenceMaxPixels = 50_000_000 // 5000 Maximum pixels (solved pressure bomb)
	evidenceMaxDim    = 20000
)

// sanitizeImage Verifying and recoding pictures by content; accepting only png/jpeg/gif,Unified Output png(or jpg).
func sanitizeImage(raw []byte) (out []byte, ext string, err error) {
	ct := http.DetectContentType(raw)
	switch ct {
	case "image/png", "image/jpeg", "image/gif":
	default:
		return nil, "", fmt.Errorf("Only support png/jpg/gif Picture (detected) %s)", ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("Not a valid picture")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > evidenceMaxDim || cfg.Height > evidenceMaxDim ||
		cfg.Width*cfg.Height > evidenceMaxPixels {
		return nil, "", fmt.Errorf("Image size exceeding limit")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("Image decode failed")
	}
	var buf bytes.Buffer
	if ct == "image/jpeg" {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", fmt.Errorf("Image processing failed")
		}
		return buf.Bytes(), ".jpg", nil
	}
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", fmt.Errorf("Image processing failed")
	}
	return buf.Bytes(), ".png", nil
}

// evidenceDir returns the list of evidence files (absolute paths) for a given finding.
func (s *Server) evidenceDir(nodeID int64) string {
	return filepath.Join(s.dataDir, "report_assets", "evidence", strconv.FormatInt(nodeID, 10))
}

// evidenceURL Construct frontend pickup URL.
func evidenceURL(nodeID int64, filename string) string {
	return fmt.Sprintf("/api/findings/%d/evidence/%s", nodeID, filename)
}

// POST /api/findings/{id}/evidence — multipart "file" + Optional caption / step_index.
func (s *Server) uploadFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil { // 16 MB
		writeErr(w, 400, "Upload resolution failed")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "Missing file")
		return
	}
	defer file.Close()
	// Read Memory(Limit 16MB)→Content Level Verification+Recode, untrusted extension/Client MIME.
	raw, err := io.ReadAll(io.LimitReader(file, evidenceMaxBytes+1))
	if err != nil {
		writeErr(w, 400, "Failed to read file")
		return
	}
	if len(raw) > evidenceMaxBytes {
		writeErr(w, 400, "Too big. 16MB)")
		return
	}
	clean, ext, err := sanitizeImage(raw)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	dir := s.evidenceDir(nodeID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, "Failed to create directory")
		return
	}
	name := fmt.Sprintf("ev-%d%s", time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(dir, name), clean, 0o644); err != nil {
		writeErr(w, 500, "Writing Failed")
		return
	}
	caption := strings.TrimSpace(r.FormValue("caption"))
	stepIndex, _ := strconv.Atoi(r.FormValue("step_index"))
	if caption == "" {
		// Autofill of the diagram: Use without handfill agent One sentence (editable, no additional) derived from the rectification of the local record LLM Call.
		caption = s.deriveEvidenceCaption(nodeID, stepIndex)
	}
	ev, err := pg.CreateFindingEvidence(nodeID, name, caption, stepIndex)
	if err != nil {
		_ = os.Remove(filepath.Join(dir, name)) // Roll Back
		writeErr(w, 500, "Save Failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"evidence": ev, "url": evidenceURL(nodeID, name)})
}

// deriveEvidenceCaption builds a default figure caption for an evidence shot from the
// agent-written reproduction step it belongs to(StepsN:What does it prove?——Figure by agent
// The duplicated document is generated automatically without any extra LLM adjustment, but it is still manual: step 0() returns early when there is no finding at all.
func (s *Server) deriveEvidenceCaption(nodeID int64, stepIndex int) string {
	if stepIndex <= 0 || s.m == nil || s.m.pg == nil {
		return ""
	}
	f, err := s.m.pg.GetFindingByNodeID(nodeID)
	if err != nil || f == nil || len(f.Reproduction) == 0 {
		return ""
	}
	var rp report.Reproduction
	if json.Unmarshal(f.Reproduction, &rp) != nil || stepIndex > len(rp.Steps) {
		return ""
	}
	st := rp.Steps[stepIndex-1]
	text := strings.TrimSpace(st.Proves)
	if text == "" {
		text = strings.TrimSpace(st.Expected)
	}
	if text == "" {
		return ""
	}
	text = strings.SplitN(text, "\n", 2)[0] // Concentrate to one line for the thumbnail note
	return clipUTF8(fmt.Sprintf("Steps%d:%s", stepIndex, text), 120)
}

// GET /api/findings/{id}/evidence — list all the evidence of a particular finding (with the diagrams inline).
func (s *Server) listFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	list, err := pg.ListFindingEvidence(nodeID)
	if err != nil {
		writeErr(w, 500, "Failed to fetch")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{"evidence": e, "url": evidenceURL(e.NodeID, e.Filename)})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

// PATCH /api/findings/{id}/evidence/{eid} — Diagram Note / Corresponding steps / Sort.
func (s *Server) updateFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	eid, ok := pathInt(r, "eid")
	if !ok {
		writeErr(w, 400, "Invalid evidence id")
		return
	}
	var req struct {
		Caption   string `json:"caption"`
		StepIndex int    `json:"step_index"`
		SortOrder int    `json:"sort_order"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Request format error")
		return
	}
	if err := pg.UpdateFindingEvidence(eid, req.Caption, req.StepIndex, req.SortOrder); err != nil {
		writeErr(w, 500, "Save Failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/findings/{id}/evidence/{eid} — Delete Record + A disc file.
func (s *Server) deleteFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	eid, ok := pathInt(r, "eid")
	if !ok {
		writeErr(w, 400, "Invalid evidence id")
		return
	}
	ev, err := pg.GetFindingEvidence(eid)
	if err != nil {
		writeErr(w, 404, "Evidence doesn't exist.")
		return
	}
	if err := pg.DeleteFindingEvidence(eid); err != nil {
		writeErr(w, 500, "Failed to delete: "+err.Error())
		return
	}
	_ = os.Remove(filepath.Join(s.evidenceDir(ev.NodeID), filepath.Base(ev.Filename)))
	writeJSON(w, 200, map[string]any{"deleted": eid})
}

// GET /api/findings/{id}/evidence/{file} — Take the evidence mapbase name,(b) Protection against crossing;cookie Equivalence logo).
func (s *Server) serveFindingEvidence(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid Hole id")
		return
	}
	file := filepath.Base(r.PathValue("file"))
	// Fixed Picture Content-Type + No sniffing. + Inline: Even if the files on the disk are tampered with, the browser will not act as if HTML/Script execution.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(filepath.Ext(file)) {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	default:
		writeErr(w, 404, "not found")
		return
	}
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, filepath.Join(s.evidenceDir(nodeID), file))
}
