package report

// Assembles the full reportDoc JSON (deterministic skeleton + AI narrative) that
// pybuilder/report_builder.py renders into a human-grade .docx.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"antifox/db"
)

// reAntiFoxDataPath / reVarFolders match PLATFORM-INTERNAL scratch paths only — the
// dataDir family (.../antifox...data/..., e.g. verify/full_poc.py) and macOS temp — so client
// reports don't leak them. Target-server paths in findings (LFI /etc/passwd Wait.) are NOT
// touched (they're legitimate vuln content, no antifox-data marker / not /var/folders).
var (
	reAntiFoxDataPath = regexp.MustCompile(`/[^\s,，、；）;")']*antifox[^\s,，、；）;")']*data[^\s,，、；）;")']*`)
	reVarFolders      = regexp.MustCompile(`(?:/private)?/var/folders/[^\s,，、；）;")']+`)
)

// redactInternalPaths collapses the platform's own absolute scratch paths to a bare
// filename so a client deliverable never shows "Evidentiary script:/.../antifox-data/verify/x.py".
func redactInternalPaths(s, dataDir string) string {
	strip := func(m string) string { return filepath.Base(m) }
	if d := strings.TrimSpace(dataDir); d != "" {
		if abs, err := filepath.Abs(d); err == nil && abs != "/" {
			s = regexp.MustCompile(regexp.QuoteMeta(abs)+`/[^\s,，、；）;")']+`).ReplaceAllStringFunc(s, strip)
		}
	}
	s = reAntiFoxDataPath.ReplaceAllStringFunc(s, strip)
	s = reVarFolders.ReplaceAllStringFunc(s, strip)
	return s
}

// redactLines applies redactInternalPaths to each string in a slice (in place-ish).
func redactLines(in []string, dataDir string) []string {
	for i := range in {
		in[i] = redactInternalPaths(in[i], dataDir)
	}
	return in
}

// FindingInput is one confirmed finding, flattened from a graph node.
type FindingInput struct {
	NodeID     int64 // exploration node id: used to load the evidence screenshots for this finding (finding_evidence)
	VulnClass  string
	Severity   string // high|medium|low|critical|info
	Summary    string
	PoC        string
	Target     string
	Transcript string // real work transcript that produced this finding (tool calls/requests/responses/analysis); grounds enrichment in what actually happened instead of expanding the one-line summary. Empty = enrich from summary/PoC only.
	// SetupContext holds the target's REAL account-registration / login / OTP / captcha
	// request packets pulled from recorded traffic, so the Precondition can spell out
	// "how to register a test account / log in" with verbatim packets — not an AI guess.
	SetupContext string
	// Reproduction is the STRUCTURED Recurrence process the agent recorded in-situ at report_finding
	// time (entry_point / prerequisites / verbatim steps / provenance). When present it
	// is the authoritative, real reproduction — rendered directly into test_process and
	// the AI is told to skip reconstructing it (saves tokens + can't fabricate). Empty =
	// fall back to AI-reconstructed test_process / PoC.
	Reproduction json.RawMessage
	CreatedAt    time.Time
	Enrichment   *Enrichment // pre-loaded persisted enrichment; when set the AI call is skipped
}

// Reproduction is the structured Recurrence process captured in-situ by the agent (mirrors the
// report_finding `reproduction` tool schema). Rendered verbatim into the report's
// test_process so a human can reproduce from zero.
type Reproduction struct {
	EntryPoint    string      `json:"entry_point"`
	Prerequisites []string    `json:"prerequisites"`
	Steps         []ReproStep `json:"steps"`
	Provenance    string      `json:"provenance"`
}

// ReproStep is one ordered reproduction step: the verbatim request, the expected
// response/behavior, and what it proves. ExchangeID (when resolved) links the step
// to the exact recorded traffic packet it cites, for jump-to-traffic / screenshot.
type ReproStep struct {
	Request    string `json:"request"`
	Response   string `json:"response"`
	Expected   string `json:"expected"`
	Proves     string `json:"proves"`
	ExchangeID string `json:"exchange_id,omitempty"`
}

// PersonInput is one Tester row from report settings.
type PersonInput struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Contact string `json:"contact"`
}

// GenInput is everything the report needs. The handler builds it from the task
// + org-level report settings.
type GenInput struct {
	CompanyName   string // Author of the report
	CompanyNameEn string
	ClientName    string
	ProjectName   string // Short project name(Cover/Title)——Never plug in the whole list of targets.
	Scope         string // Full Test Range/Target System Description(Hello? AI No cover.)
	ReportTitle   string
	Version       string
	Locale        string
	LogoPath      string
	CVSSVersion   string
	Personnel     []PersonInput // from report settings; empty → neutral default
	Findings      []FindingInput
	AssetCounts   map[string]int
	SkillDir      string // platform skill root; enables editable report-generation skill
	DataDir       string // platform data dir; used to redact internal scratch paths from client reports
}

// BuildDoc assembles the report JSON: skeleton defaults + AI-written narrative
// (report-level + one call per finding, run concurrently, bounded).
func BuildDoc(ctx context.Context, pg *db.DB, in *GenInput) *reportDoc {
	in.Locale = normLocale(in.Locale)
	loc := in.Locale
	if strings.TrimSpace(in.CompanyName) == "" {
		in.CompanyName = pick(loc, "Security Service Team", "Security Services.", "Security Services Team")
	}
	// Editable report-generation skill (skeleton + prompts), read fresh each run
	// with a per-file fallback to the built-in defaults.
	sk := loadSkillContent(in.SkillDir, loc)
	doc := &reportDoc{
		Meta: docMeta{
			CompanyName: in.CompanyName,
			ClientName:  defaultStr(in.ClientName, clientDefault(loc)),
			ProjectName: in.ProjectName,
			ReportTitle: defaultStr(in.ReportTitle, defaultReportTitle(loc, in.ProjectName)),
			Version:     defaultStr(in.Version, "V1.0"),
			Date:        localizedDate(loc),
			LogoPath:    in.LogoPath,
			Locale:      loc,
			CVSSVersion: defaultStr(in.CVSSVersion, "3.1"),
		},
		Personnel:        personnelRows(loc, in.Personnel, sk.Personnel),
		MethodologyIntro: sk.MethodologyIntro,
		Methodology:      toNameDesc(sk.Methodology),
		Tools:            toNameDesc(sk.Tools),
		Standards:        sk.Standards,
	}

	// --- AI: report-level narrative only (Background/Service results/Correct conclusions; bottom-up, never blank) ---
	// The body of every loophole(Description/Risk/Rehabilitation)Re-use All: Report only confirmed(=Exploited)It's a loophole.
	// So there's always room for enrichment; there's no loophole.(Inappropriate Path)Leave space. Enrichment,assembleVuln Back to
	// Original summary/PoC.
	var rl reportLevelOut
	narr := make([]Enrichment, len(in.Findings))
	{
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); rl = aiReportLevel(ctx, pg, in, sk) }()
		for i := range in.Findings {
			if e := in.Findings[i].Enrichment; e != nil { // Reuse Persistence
				narr[i] = *e
			}
		}
		wg.Wait()
	}

	// group findings by target (each distinct target → one 3.x system section)
	groups := groupByTarget(loc, in.Findings, narr)

	// systems summary rows
	var systems []sysRow
	for _, g := range groups {
		c := countBySeverity(g.findings)
		systems = append(systems, sysRow{
			Name: systemDisplayName(loc, g.target), Address: g.target,
			Crit: c["critical"], High: c["high"], Med: c["medium"], Low: c["low"],
			Rating: severityName(loc, worstSeverity(g.findings)),
		})
	}

	doc.Summary = summarySection{
		Background: rl.Background, ServiceResult: rl.ServiceResult,
		Systems: systems, RemediationConclusion: rl.RemediationConclusion,
	}
	cAll := countAll(in.Findings)
	if len(doc.Summary.Background) == 0 {
		doc.Summary.Background = []string{fallbackBackground(loc, defaultStr(in.ClientName, clientDefault(loc)), in.CompanyName, in.ProjectName)}
	}
	if doc.Summary.ServiceResult == "" {
		doc.Summary.ServiceResult = fallbackServiceResult(loc, len(in.Findings), cAll["critical"], cAll["high"], cAll["medium"], cAll["low"], cAll["info"])
	}
	if doc.Summary.RemediationConclusion == "" {
		doc.Summary.RemediationConclusion = fallbackRemediationConclusion(loc)
	}
	doc.Checklist = buildChecklist(sk.Checklist, rl.ChecklistHits)
	doc.FindingsIntro = fallbackFindingsIntro(loc, in.CompanyName, cAll["critical"], cAll["high"], cAll["medium"], cAll["low"], cAll["info"])

	// per-finding assembly
	secIdx := 0
	var remItems []remedyRow
	var vlist []vulnListRow
	for _, g := range groups {
		secIdx++
		sd := systemDetail{Name: sectionSystemHeader(loc, secIdx, systemDisplayName(loc, g.target), g.target)}
		for vi, gf := range g.findings {
			evByStep := loadEvidenceByStep(pg, in.DataDir, gf.f.NodeID, loc) // The evidence's broken down into steps.
			detail := assembleVuln(loc, gf.f, gf.enr, fmt.Sprintf("3.%d.%d", secIdx, vi+1), in.DataDir, evByStep)
			sd.Vulns = append(sd.Vulns, detail)
			remItems = append(remItems, remedyRow{
				Idx: len(remItems) + 1, Object: vulnObject(loc, detail.Title, detail.Target),
				Conclusion: statusPending(loc), Remark: statusRemarkFirst(loc),
			})
			vlist = append(vlist, vulnListRow{
				Severity: severityName(loc, gf.f.Severity), Name: detail.Title,
				Desc: firstLine(detail.Description), Status: statusPending(loc),
			})
		}
		doc.SystemsDetail = append(doc.SystemsDetail, sd)
	}
	doc.Summary.RemediationItems = remItems
	doc.VulnList = vlist
	doc.OverallConcl = buildOverallConclusion(loc, in, systems)
	return doc
}

func assembleVuln(loc string, f FindingInput, enr Enrichment, secNo, dataDir string, evByStep map[int][]procBlock) vulnDetail {
	// Vuln NAME: AI-polished title → vulnclass (already a name, e.g. "SSL POODLE
	// (CVE-2014-3566)") → cleaned summary as last resort. Summary is the long
	// description, never the name.
	title := cleanTitle(nz(enr.Title, nz(f.VulnClass, f.Summary)))
	target := nz(enr.Target, f.Target)
	d := vulnDetail{
		SecNo: secNo, Title: title,
		Severity: severityName(loc, f.Severity), Target: target, URL: target,
		FoundDate: f.CreatedAt.Format("2006-01-02"), FixStatus: statusPending(loc),
	}
	if len(enr.Description) > 0 {
		d.Description = enr.Description
	} else if strings.TrimSpace(f.Summary) != "" {
		d.Description = []string{f.Summary}
	}
	d.Module = defaultStr(enr.Module, "—")
	// test_process priority: (1) Structural reproduction of local records (verbatim, authoritative — the
	// agent recorded it while testing, so it can't be fabricated); (2) AI-reconstructed
	// test_process; (3) raw PoC. Captured reproduction wins because it's the real thing.
	if b := reproToBlocks(f.Reproduction, loc, evByStep); len(b) > 0 {
		d.TestProcess = b // Structured recurrence: evidence inserted in step
	} else {
		if len(enr.TestProcess) > 0 {
			d.TestProcess = enr.TestProcess
		} else if strings.TrimSpace(f.PoC) != "" {
			d.TestProcess = proofToBlocks(f.PoC)
		}
		for _, k := range sortedIntKeys(evByStep) { // Unstructured recurrence: consolidation of evidence at the end
			d.TestProcess = append(d.TestProcess, evByStep[k]...)
		}
	}
	d.RiskAnalysis = enr.RiskAnalysis
	d.Remediation = enr.Remediation
	// Client reporting dissensitisation: AI Vulcanization/PoC Inside the platform quoted scratch Path to file name.
	d.Description = redactLines(d.Description, dataDir)
	d.RiskAnalysis = redactLines(d.RiskAnalysis, dataDir)
	d.Remediation = redactLines(d.Remediation, dataDir)
	for i := range d.TestProcess {
		d.TestProcess[i].Content = redactInternalPaths(d.TestProcess[i].Content, dataDir)
	}
	d.CVSSScore = enr.CVSSScore
	d.CVSSVector = enr.CVSSVector
	if len(enr.CVSSMetrics) > 0 {
		m := map[string]string{}
		for k, v := range enr.CVSSMetrics {
			if v != "" {
				m[strings.ToUpper(k)] = v
			}
		}
		d.CVSSMetrics = m
	}
	return d
}

// ---- grouping ----

type groupedFinding struct {
	f   FindingInput
	enr Enrichment
}
type targetGroup struct {
	target   string
	findings []groupedFinding
}

func groupByTarget(loc string, fs []FindingInput, narr []Enrichment) []targetGroup {
	order := []string{}
	idx := map[string]int{}
	var groups []targetGroup
	for i, f := range fs {
		key := strings.TrimSpace(nz(narr[i].Target, f.Target)) // AI-extracted host preferred
		if key == "" {
			key = defaultTargetName(loc)
		}
		gi, ok := idx[key]
		if !ok {
			idx[key] = len(groups)
			gi = len(groups)
			groups = append(groups, targetGroup{target: key})
			order = append(order, key)
		}
		groups[gi].findings = append(groups[gi].findings, groupedFinding{f: f, enr: narr[i]})
	}
	_ = order
	return groups
}

// ---- severity ----

// sevCanonical maps severity words to the canonical set. Keys are lower-case
// because canonSeverity lower-cases its input; the Chinese aliases are kept so
// severity values stored before the English conversion still normalize.
var sevCanonical = map[string]string{
	"critical": "critical", "serious": "critical", "severe": "critical", "crit": "critical",
	"严重": "critical", "嚴重": "critical",
	"high": "high", "high risk": "high", "高危": "high", "高": "high",
	"medium": "medium", "medium risk": "medium", "moderate": "medium", "med": "medium",
	"中危": "medium", "中": "medium", "it's dangerous.": "medium",
	"low": "low", "low risk": "low", "low risk.": "low", "低危": "low", "低": "low",
	"info": "info", "information": "info", "informational": "info", "信息": "info", "資訊": "info",
}
var sevRankFull = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4, "": 5}

func canonSeverity(s string) string {
	if c, ok := sevCanonical[strings.ToLower(strings.TrimSpace(s))]; ok {
		return c
	}
	return "info"
}
func countBySeverity(fs []groupedFinding) map[string]int {
	c := map[string]int{}
	for _, gf := range fs {
		c[canonSeverity(gf.f.Severity)]++
	}
	return c
}
func worstSeverity(fs []groupedFinding) string {
	worst := "info"
	for _, gf := range fs {
		if sevRankFull[canonSeverity(gf.f.Severity)] < sevRankFull[worst] {
			worst = canonSeverity(gf.f.Severity)
		}
	}
	return worst
}
func countAll(fs []FindingInput) map[string]int {
	c := map[string]int{}
	for _, f := range fs {
		c[canonSeverity(f.Severity)]++
	}
	return c
}

// ---- skeleton helpers ----

func toNameDesc(items []kv) []nameDesc {
	out := make([]nameDesc, 0, len(items))
	for _, i := range items {
		out = append(out, nameDesc{Name: i.Name, Desc: i.Desc})
	}
	return out
}
func personnelFromMaps(rows []map[string]string) []person {
	out := make([]person, 0, len(rows))
	for _, p := range rows {
		out = append(out, person{Name: p["name"], Role: p["role"], Contact: p["contact"]})
	}
	return out
}

// personnelRows prefers the user-configured personnel (report settings); falls
// back to the skill/neutral default (skillDefault, already locale-resolved) when
// none is set.
func personnelRows(loc string, rows []PersonInput, skillDefault []map[string]string) []person {
	out := make([]person, 0, len(rows))
	for _, p := range rows {
		if strings.TrimSpace(p.Name) == "" && strings.TrimSpace(p.Role) == "" && strings.TrimSpace(p.Contact) == "" {
			continue
		}
		out = append(out, person{Name: p.Name, Role: p.Role, Contact: p.Contact})
	}
	if len(out) == 0 {
		return personnelFromMaps(skillDefault)
	}
	return out
}

// buildChecklist emits a stable "pass" | "fail" token in Result; the Python
// renderer localizes the display word and applies the red-bold "fail" styling.
func buildChecklist(items []checklistItem, hits []string) []checkRow {
	hitSet := map[string]bool{}
	for _, h := range hits {
		hitSet[normalizeName(h)] = true
	}
	out := make([]checkRow, 0, len(items))
	for _, c := range items {
		res := "pass"
		if hitSet[normalizeName(c.Name)] {
			res = "fail"
		}
		out = append(out, checkRow{Category: c.Category, Name: c.Name, Desc: c.Desc, Result: res})
	}
	return out
}
func buildOverallConclusion(loc string, in *GenInput, _ []sysRow) string {
	c := countAll(in.Findings)
	worst := severityName(loc, worstSeverityAll(in.Findings))
	return overallConclusion(loc, len(in.Findings), c["critical"], c["high"], c["medium"], c["low"], c["info"], worst)
}
func worstSeverityAll(fs []FindingInput) string {
	worst := "info"
	for _, f := range fs {
		if sevRankFull[canonSeverity(f.Severity)] < sevRankFull[worst] {
			worst = canonSeverity(f.Severity)
		}
	}
	return worst
}

// ---- text utils ----

func defaultStr(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
func systemDisplayName(loc, target string) string {
	t := strings.TrimSpace(target)
	t = strings.TrimPrefix(t, "https://")
	t = strings.TrimPrefix(t, "http://")
	if i := strings.IndexAny(t, "/"); i > 0 {
		t = t[:i]
	}
	if t == "" {
		return defaultTargetName(loc)
	}
	return t
}
func firstLine(paras []string) string {
	if len(paras) == 0 {
		return ""
	}
	return paras[0]
}
func truncateRune(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
func normalizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "（", "(")
	s = strings.ReplaceAll(s, "）", ")")
	return s
}
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	prefixes := []string{
		"CRITICAL:", "CRITICAL：", "HIGH:", "HIGH：", "MEDIUM:", "MEDIUM：", "LOW:", "LOW：", "INFO:", "INFO：",
		"[严重]", "[高危]", "[中危]", "[低危]", "[信息]", "【严重】", "【高危】", "【中危】", "【低危】", "【信息】",
		"严重：", "高危：", "中危：", "低危：", "信息：",
		"[Serious]", "[High risk]", "[Medium]", "[Low risk]", "[Information]",
		"Serious:", "High risk:", "Medium:", "Low risk:", "Information:",
	}
	for {
		trimmed := s
		for _, p := range prefixes {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, p))
		}
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		if trimmed == s {
			break
		}
		s = trimmed
	}
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60])
	}
	return s
}

// reproToBlocks renders the agent's structured Recurrence process (captured in-situ) into
// ordered text/code procBlocks for the report's test_process: Source → Entry point →
// Precondition → Every step(Annotations text + True Request code) → No need. AI Reconstruction. Returns nil when the
// reproduction is absent or unparseable (caller falls back to AI/PoC).
// reproToBlocks Putting the process of structured recovery in order. text/code/image Blocks.evByStep It's an evidence screenshot of that loophole.
// Step Number(1..N;0=Overview)Group image Blocks——[Plugin) Rendering: A description of each step+Request+Follow the evidence chart of the step after the response;
// Evidence of non-attribution(Overview/Crossing the border)Add at the end.evByStep Yes nil Decline to plain text(Use when emulating).
func reproToBlocks(raw json.RawMessage, loc string, evByStep map[int][]procBlock) []procBlock {
	if len(raw) < 3 { // ""/"{}" carry nothing
		return nil
	}
	var r Reproduction
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	var blocks []procBlock
	addText := func(s string) {
		if strings.TrimSpace(s) != "" {
			blocks = append(blocks, procBlock{Type: "text", Content: strings.TrimSpace(s)})
		}
	}
	addCode := func(s string) {
		if strings.TrimSpace(s) != "" {
			blocks = append(blocks, procBlock{Type: "code", Content: s})
		}
	}
	used := map[int]bool{}
	if strings.TrimSpace(r.Provenance) != "" {
		addText(pick(loc, "Origin: ", "Origin: ", "Origin: ") + r.Provenance)
	}
	if strings.TrimSpace(r.EntryPoint) != "" {
		addText(pick(loc, "Entry point: ", "Entry point: ", "Entry point: ") + r.EntryPoint)
	}
	if pre := nonEmpty(r.Prerequisites); len(pre) > 0 {
		var sb strings.Builder
		sb.WriteString(pick(loc, "Prerequisites:", "Prerequisites:", "Prerequisites:"))
		for _, p := range pre {
			sb.WriteString("\n• " + strings.TrimSpace(p))
		}
		addText(sb.String())
	}
	if len(r.Steps) > 0 {
		addText(pick(loc, "Reproduction steps:", "Reproduction steps:", "Reproduction steps:"))
		lStep := pick(loc, "Step", "Step", "Step")
		lExp := pick(loc, "Expected", "Expected", "Expected")
		lProves := pick(loc, "Proves", "Proves", "Proves")
		for i, st := range r.Steps {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%s %d", lStep, i+1))
			if strings.TrimSpace(st.Expected) != "" {
				sb.WriteString(" — " + lExp + ": " + strings.TrimSpace(st.Expected))
			}
			if strings.TrimSpace(st.Proves) != "" {
				sb.WriteString(" | " + lProves + ": " + strings.TrimSpace(st.Proves))
			}
			addText(sb.String())
			addCode(st.Request)
			if strings.TrimSpace(st.Response) != "" {
				addText(pick(loc, "Response:", "Response:", "Response:"))
				addCode(st.Response)
			}
			if ev := evByStep[i+1]; len(ev) > 0 { // Plug-in: This step follows closely.
				blocks = append(blocks, ev...)
				used[i+1] = true
			}
		}
	}
	for _, k := range sortedIntKeys(evByStep) { // Overview(0) + Evidence of cross-border steps added at the end
		if !used[k] {
			blocks = append(blocks, evByStep[k]...)
		}
	}
	return blocks
}

// sortedIntKeys Back map Yes. int Keysupper slice (ensure that the order of rendering of the evidence block is determined).
func sortedIntKeys(m map[int][]procBlock) []int {
	if len(m) == 0 {
		return nil
	}
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	return ks
}

// loadEvidenceByStep Load an evidentiary trail of a loophole and split it by step. image blocks;charts fill in default labels when empty
// (StepsN Evidence / An evidence cut-off chart, additional serial numbers) ensures that each is included in the report.
func loadEvidenceByStep(pg *db.DB, dataDir string, nodeID int64, loc string) map[int][]procBlock {
	if pg == nil || nodeID <= 0 {
		return nil
	}
	list, err := pg.ListFindingEvidence(nodeID)
	if err != nil || len(list) == 0 {
		return nil
	}
	ns := fmt.Sprintf("%d", nodeID)
	m := map[int][]procBlock{}
	seq := map[int]int{}
	for _, e := range list {
		seq[e.StepIndex]++
		cap := strings.TrimSpace(e.Caption)
		if cap == "" {
			if e.StepIndex > 0 {
				cap = fmt.Sprintf("%s %d %s", pick(loc, "Steps", "Step", "Step"), e.StepIndex, pick(loc, "Evidence", "Evidence.", "evidence"))
			} else {
				cap = pick(loc, "Evidence screenshot", "Evidence.", "Evidence")
			}
			if seq[e.StepIndex] > 1 {
				cap = fmt.Sprintf("%s %d", cap, seq[e.StepIndex])
			}
		}
		m[e.StepIndex] = append(m[e.StepIndex], procBlock{
			Type:    "image",
			Path:    filepath.Join(dataDir, "report_assets", "evidence", ns, e.Filename),
			Caption: cap,
		})
	}
	return m
}

// nonEmpty drops blank strings from a slice.
func nonEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// proofToBlocks converts raw PoC text into text/code blocks (brace-aware).
func proofToBlocks(proof string) []procBlock {
	for _, fence := range []string{"```json", "```bash", "```shell", "```http", "```html", "```xml", "```sql", "```python", "```js", "```javascript", "```text", "```go"} {
		proof = strings.ReplaceAll(proof, fence, "")
	}
	proof = strings.ReplaceAll(proof, "```", "")
	var blocks []procBlock
	var codeBuf []string
	depth := 0
	flush := func() {
		if len(codeBuf) > 0 {
			blocks = append(blocks, procBlock{Type: "code", Content: strings.Join(codeBuf, "\n")})
			codeBuf = nil
		}
		depth = 0
	}
	for _, line := range strings.Split(proof, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "### ")
		trimmed = strings.TrimPrefix(trimmed, "## ")
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		if trimmed == "" {
			if depth <= 0 {
				flush()
			} else {
				codeBuf = append(codeBuf, "")
			}
			continue
		}
		opens := strings.Count(trimmed, "{") + strings.Count(trimmed, "[")
		closes := strings.Count(trimmed, "}") + strings.Count(trimmed, "]")
		if depth > 0 || looksLikeCode(trimmed) {
			codeBuf = append(codeBuf, line)
			depth += opens - closes
			if depth < 0 {
				depth = 0
			}
		} else {
			flush()
			blocks = append(blocks, procBlock{Type: "text", Content: trimmed})
		}
	}
	flush()
	return blocks
}

func looksLikeCode(s string) bool {
	for _, p := range []string{"GET ", "POST ", "PUT ", "DELETE ", "HTTP/", "curl ", "sqlmap", "$ ", "# ", "http://", "https://", "{", "}", "<", "SELECT ", "UNION ", "&&", "||", "/*"} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

var _ = sort.Strings
