package report

// AI narrative layer: turns thin findings ({vulnclass, severity, summary, poc})
// into professional English report prose (description / test process / risk /
// remediation + CVSS) via the platform's active LLM profile. Report-level and
// per-finding calls run concurrently. All AI output is JSON; parse failures fall
// back to the raw finding fields (assembly in build.go), so generation never
// hard-fails on a bad LLM response.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"antifox/agent"
	"antifox/db"
	"antifox/engine/agentcore"
	acperm "antifox/engine/permission"
	actool "antifox/engine/tool"
)

// complete runs one system+user → text completion on the active LLM profile.
// Returns an error when no profile is configured (caller falls back to skeleton).
func complete(ctx context.Context, pg *db.DB, system, user string, maxTokens int) (string, error) {
	// Report/The role of eutrophication can be bound to a given LLM(Role division)——Unbound is activated with a global approach. It's a much stronger model.
	// (Like Claude)I'm not moving. agent The model.
	p, err := pg.ReportRoleProfile()
	if err != nil || p == nil || strings.TrimSpace(p.APIKey) == "" {
		return "", fmt.Errorf("no active LLM profile")
	}
	cfg := agent.ConfigFrom(p.Format, p.Model, p.BaseURL, p.APIKey, p.Proxy)
	cfg.RatePerSecond, cfg.RatePerMinute = p.RatePerSecond, p.RatePerMinute
	cfg.ReasoningEffort = p.ReasoningEffort
	prov, err := cfg.NewProvider()
	if err != nil {
		return "", err
	}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	out, err := agentcore.Run(cctx, agentcore.Options{
		Provider:       prov,
		SystemPrompt:   []string{system},
		Tools:          []actool.CoreTool{}, // non-nil empty → no default tools offered
		PermissionMode: acperm.ModeBypass,
		MaxTurns:       1,
		MaxTokens:      maxTokens,
	}, user)
	if err != nil {
		return "", err
	}
	return cleanJSON(out), nil
}

// ---------------------------------------------------------------------------
// Report-level narrative (background / service result / remediation / checklist)
// ---------------------------------------------------------------------------

type reportLevelOut struct {
	Background            []string `json:"background"`
	ServiceResult         string   `json:"service_result"`
	RemediationConclusion string   `json:"remediation_conclusion"`
	ChecklistHits         []string `json:"checklist_hits"`
}

const sysReportLevel = "You are a senior penetration-testing report writer for %s, specializing in professional, client-readable, and technically rigorous English penetration-testing reports. Output strictly JSON only, with no Markdown code block."

const userReportLevel = `Write the report overview using the following project and finding list.

Project name: %s
Client name: %s
Finding list:
%s
Requirements (professional delivery tone; do not use phrasing such as "the test was performed today"):
- background: a string array of two paragraphs. The first paragraph states that the client engaged %s to conduct penetration testing and gives the testing objectives. The second paragraph describes the test scope and the combination of manual and tool-assisted testing. Each paragraph is 60-130 words.
- service_result: one paragraph summarizing which systems were tested and the number and severity distribution of the findings.
- remediation: 2-4 entries, one sentence each, specific and actionable - which point to change, how to change it, and what to validate. Do not use empty advice such as "increase security awareness" or "perform regular security audits".
- checklist_hits: a string array of standard test-item names inferred from the finding types (for example "SQL Injection", "Unauthorized Access", "Authentication Bypass", "Sensitive Information Disclosure", "Command Injection", "Cross-Site Scripting (XSS)", "Vertical Privilege Escalation").

Output JSON only: {"background":["...","..."],"service_result":"...","remediation_conclusion":"...","checklist_hits":["...","..."]}`

// reportLevelPromptData feeds the editable report-level prompt template.
type reportLevelPromptData struct {
	Company, Project, Client, FindingsList, LangDirective string
}

// vulnPromptData feeds the editable per-vuln prompt template.
type vulnPromptData struct {
	Company, VulnClass, Severity, Summary, PoC, LangDirective string
}

func aiReportLevel(ctx context.Context, pg *db.DB, in *GenInput, sk *skillContent) reportLevelOut {
	var out reportLevelOut
	loc := normLocale(in.Locale)
	var sb strings.Builder
	// Feed the full test scope / target systems into the narrative through the
	// FindingsList channel (both the skill and fallback prompt paths carry it), so the
	// overview can name the specific systems, while the short cover project name
	// (ProjectName) stays unaffected.
	if sc := strings.TrimSpace(in.Scope); sc != "" {
		fmt.Fprintf(&sb, "[Test scope / target systems] %s\n\n[Finding list]\n", truncateRune(sc, 1500))
	}
	for _, f := range in.Findings {
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", severityName(loc, f.Severity), nz(f.VulnClass, "-"), truncateRune(f.Summary, 80))
	}
	dir := outputLangDirective(loc)
	data := reportLevelPromptData{Company: in.CompanyName, Project: in.ProjectName, Client: in.ClientName, FindingsList: sb.String(), LangDirective: dir}
	sys := renderPrompt(sk.reportLevelSys, data)
	if sys == "" {
		sys = fmt.Sprintf(sysReportLevel, in.CompanyName)
	}
	user := renderPrompt(sk.reportLevelUser, data)
	if user == "" {
		user = fmt.Sprintf(userReportLevel, in.ProjectName, in.ClientName, sb.String(), in.CompanyName) +
			"\n\nOutput-language requirement: " + dir
	}
	raw, err := complete(ctx, pg, sys, user, 4000)
	if err != nil {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// ---------------------------------------------------------------------------
// Per-finding narrative (description / test process / risk / remediation / CVSS)
// ---------------------------------------------------------------------------

type Enrichment struct {
	Title        string            `json:"title"`  // concise professional vuln name
	Target       string            `json:"target"` // affected host / URL, extracted from evidence
	Description  []string          `json:"description"`
	Module       string            `json:"module"`
	TestProcess  []procBlock       `json:"test_process"`
	RiskAnalysis []string          `json:"risk_analysis"`
	Remediation  []string          `json:"remediation"`
	CVSSScore    float64           `json:"cvss_score"`
	CVSSVector   string            `json:"cvss_vector"`
	CVSSMetrics  map[string]string `json:"cvss_metrics"`
}

const sysVuln = "You are a senior penetration-testing report writer for %s. Rewrite the raw vulnerability record as delivery-grade report prose - written by a senior consultant for the client: refined, professional, and strictly on point. Output strictly JSON only, with no Markdown code block, no pleasantries, no disclaimers, and no generic filler. Original text such as HTTP requests, commands, and responses must be preserved exactly and must never be fabricated."

const userVuln = `Rewrite the following vulnerability as report prose. The core requirement is refinement, specificity, and restraint - prefer omission over padding; do not pad the count, do not pad with filler, and do not state the obvious. Every sentence must carry real information specific to this target and this vulnerability.

Vulnerability type / original name: %s
Severity: %s
Original summary:
%s
Original reproduction steps / evidence:
%s

Field requirements:
- title: a standardized, professional vulnerability name (10-26 words, for example "External Account Authentication Bypass Leading to Administrator Takeover"). If the original name is already standardized (including a CVE identifier), keep it or make a minor adjustment; do not copy the summary.
- target: the affected host or URL, extracted from the summary and evidence; leave it as an empty string when it cannot be determined.
- description: one paragraph of 2-3 sentences. It must state only three things - what the vulnerability is, where it occurs, and its root cause. Do not stack OWASP categories, do not repeat the remediation, and do not pad with generalities.
- module: the functional module it belongs to (short, for example "External account login / unauthorized API").
- risk_analysis: 2-4 entries, one sentence each, describing the concrete consequences this specific target and vulnerability allow - who can do what, and which data or privileges they obtain. Write fewer entries when there is no concrete consequence; do not use filler such as "may affect system security".
- remediation: 2-4 entries, one sentence each, specific and actionable - which point to change, how to change it, and what to validate. Do not use empty advice such as "increase security awareness" or "perform regular security audits".
- CVSS 3.1: cvss_score (number), cvss_vector (for example "AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"), and cvss_metrics (eight English values, for example {"AV":"Network","AC":"Low","PR":"None","UI":"None","S":"Unchanged","C":"High","I":"High","A":"High"}).

Do not output test_process - the platform generates the reproduction steps directly from real evidence, so return "test_process":[].

Output JSON only: {"title":"...","target":"...","description":["..."],"module":"...","test_process":[],"risk_analysis":["..."],"remediation":["..."],"cvss_score":0,"cvss_vector":"...","cvss_metrics":{...}}`

func aiVulnNarrative(ctx context.Context, pg *db.DB, locale string, f FindingInput, companyName string, sk *skillContent) Enrichment {
	var out Enrichment
	loc := normLocale(locale)
	dir := outputLangDirective(loc)
	data := vulnPromptData{Company: companyName, VulnClass: nz(f.VulnClass, "-"), Severity: severityName(loc, f.Severity), Summary: nz(f.Summary, "-"), PoC: nz(f.PoC, "-"), LangDirective: dir}
	sys := renderPrompt(sk.vulnSys, data)
	if sys == "" {
		sys = fmt.Sprintf(sysVuln, companyName)
	}
	user := renderPrompt(sk.vulnUser, data)
	if user == "" {
		user = fmt.Sprintf(userVuln,
			nz(f.VulnClass, "-"), severityName(loc, f.Severity), nz(f.Summary, "-"), nz(f.PoC, "-")) +
			"\n\nOutput-language requirement: " + dir
	}
	// Ground the PROSE (description / risk / remediation) in the ACTUAL work transcript
	// (real tool calls / requests / responses / analysis recorded when the finding was
	// made) so it reflects what actually happened, not an expansion of the one-liner.
	if tr := strings.TrimSpace(f.Transcript); tr != "" {
		user += "\n\n=== The real work transcript recorded when this vulnerability was found (tool calls / requests / responses / analysis) ===\n" +
			"Base description and risk_analysis on the real observations in the transcript; never fabricate or speculate about anything the transcript does not contain.\n\n" + tr
	}
	// PROSE-ONLY, ALWAYS. We never ask the model to reconstruct a verbatim test_process:
	// that large generation is slow and unreliable (many models time out / return nothing
	// on it) AND risks fabricating a PoC (the user's key pain). Instead test_process is
	// filled DETERMINISTICALLY below from REAL evidence - the agent's in-situ
	// reproduction steps if present, else the raw PoC. The model only writes prose
	// (title/desc/module/risk/remediation/CVSS), which is small, fast, and reliable on any model.
	if len(f.Reproduction) > 2 { // ""/"{}" carry nothing
		user += "\n\n=== The reproduction steps were recorded in place by the test engine ===\n" +
			"The entry point, prerequisites, steps, and provenance of this vulnerability were recorded during testing and will be used directly for the report test process. You do not need to, and must not, rebuild test_process - just return \"test_process\":[]."
	} else {
		user += "\n\n=== test_process is provided directly from the real evidence ===\n" +
			"The test process will be generated by the platform directly from the real PoC and evidence. You do not need to, and must not, rebuild or fabricate test_process - just return \"test_process\":[]."
	}
	user += "Focus on making title, description, module, risk_analysis, remediation, and CVSS substantial and professional, grounded in the real observations in the summary and transcript above. Never fabricate anything the transcript does not contain."
	raw, err := complete(ctx, pg, sys, user, 4000)
	if err != nil {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	// Fill test_process from REAL evidence, never the model's reconstruction: the agent's
	// in-situ Recurrence process wins; else the raw PoC (deterministic). This makes every consumer
	// (persisted enrichment -> findings page UI, and the report) show real, non-fabricated
	// steps — and keeps the generation prose-only (small, reliable).
	if b := reproToBlocks(f.Reproduction, loc, nil); len(b) > 0 {
		out.TestProcess = b
	} else if len(out.TestProcess) == 0 && strings.TrimSpace(f.PoC) != "" {
		out.TestProcess = proofToBlocks(f.PoC)
	}
	return out
}

// EnrichFinding is the exported per-finding AI enrichment, shared by the report
// pipeline and the findings-management "AI Analysis" endpoint. locale controls the
// output language (zh-CN | zh-TW | en-US, all rendering English content); skillDir enables the editable
// report-generation prompt (falls back to the built-in prompt when unset/missing).
// Returns a zero Enrichment (not an error) when no LLM profile is configured —
// callers keep whatever raw fields the finding already has.
func EnrichFinding(ctx context.Context, pg *db.DB, skillDir, locale string, f FindingInput, companyName string) Enrichment {
	if strings.TrimSpace(companyName) == "" {
		companyName = pick(locale, "Security Services Team", "Security Services Team", "Security Services Team")
	}
	sk := loadSkillContent(skillDir, locale)
	return aiVulnNarrative(ctx, pg, locale, f, companyName, sk)
}

// cleanJSON strips markdown fences and surrounding prose from an LLM response.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1] // strip leading AND trailing prose around the JSON object
		}
	}
	return s
}
