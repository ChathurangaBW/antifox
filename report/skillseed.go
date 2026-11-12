package report

// Seeds the editable report-generation skill on startup by materializing the
// built-in Go defaults (defaults.go) into files under <skillDir>/report-generation/.
// Only writes files that are MISSING, so user edits in the Skills page are never
// overwritten. This makes the built-in skeleton + prompts editable without ever
// requiring a hand-maintained copy: the Go defaults remain the single source, and
// this projects them to disk once so they can be tuned and hot-reloaded.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var seedLocales = []string{"zh-CN", "zh-TW", "en-US"}

// SeedSkillFiles writes any missing report-generation skill files. Best-effort:
// returns the first write error but always attempts every file.
func SeedSkillFiles(skillDir string) error {
	if strings.TrimSpace(skillDir) == "" {
		return nil
	}
	base := filepath.Join(skillDir, reportSkillName)
	var firstErr error
	put := func(rel, content string) {
		if err := writeIfMissing(filepath.Join(base, rel), content); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	putJSON := func(rel string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		put(rel, string(b)+"\n")
	}

	put("SKILL.md", skillMD)
	put(filepath.Join("prompts", "report-level.tmpl"), promptReportLevelTmpl)
	put(filepath.Join("prompts", "vuln-narrative.tmpl"), promptVulnTmpl)

	for _, loc := range seedLocales {
		d := filepath.Join("skeleton", loc)
		putJSON(filepath.Join(d, "methodology.json"), map[string]any{
			"intro": methodologyIntro(loc), "phases": methodology(loc),
		})
		putJSON(filepath.Join(d, "tools.json"), tools(loc))
		putJSON(filepath.Join(d, "standards.json"), standards(loc))
		putJSON(filepath.Join(d, "checklist.json"), checklist(loc))
		putJSON(filepath.Join(d, "personnel.json"), defaultPersonnel(loc))
	}
	return firstErr
}

// writeIfMissing creates parent dirs and writes content only when path is absent.
func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // exists → keep user edits
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

const skillMD = `---
name: report-generation
description: Editable content for penetration-testing report generation - the AI rewriting prompts plus the fixed skeleton (methodology, test checklist, tools, reference standards, test personnel), split by language. The report engine reads these files on every generation, so edits take effect immediately.
---

# Report generation (editable content)

This skill stores the **editable content of penetration-testing reports**, read by the generation pipeline in Report Management. After editing, it **takes effect on the next generation** with no restart or recompilation; if any file is missing or corrupted, the engine automatically falls back to the built-in default.

## Structure

- ` + "`" + `prompts/report-level.tmpl` + "`" + ` - AI prompt for report-level narrative (background / service results / remediation conclusion / hit checklist items).
- ` + "`" + `prompts/vuln-narrative.tmpl` + "`" + ` - AI prompt for rewriting a single vulnerability (description / reproduction steps / risk / remediation / CVSS).
- ` + "`" + `skeleton/<language>/methodology.json` + "`" + ` - 2.2 test workflow: ` + "`" + `{intro, phases:[{name,desc}]}` + "`" + `.
- ` + "`" + `skeleton/<language>/checklist.json` + "`" + ` - 2.3 test checklist: ` + "`" + `[{category,name,desc}]` + "`" + `. Items the AI judges as hit are rendered as "failed".
- ` + "`" + `skeleton/<language>/tools.json` + "`" + ` - 2.4 test tools: ` + "`" + `[{name,desc}]` + "`" + `.
- ` + "`" + `skeleton/<language>/standards.json` + "`" + ` - 2.5 reference standards: ` + "`" + `["..."]` + "`" + `.
- ` + "`" + `skeleton/<language>/personnel.json` + "`" + ` - 2.1 default test personnel: ` + "`" + `[{name,role,contact}]` + "`" + ` (values the user enters in the report settings take precedence).

Language directories: ` + "`" + `zh-CN` + "`" + ` (Simplified) / ` + "`" + `zh-TW` + "`" + ` (Traditional) / ` + "`" + `en-US` + "`" + ` (English).

## Prompt template variables

The prompts are Go ` + "`" + `text/template` + "`" + ` documents and use ` + "`" + `===USER===` + "`" + ` to separate the system and user sections:

- Report level: ` + "`" + `{{.Company}}` + "`" + ` ` + "`" + `{{.Project}}` + "`" + ` ` + "`" + `{{.Client}}` + "`" + ` ` + "`" + `{{.FindingsList}}` + "`" + ` ` + "`" + `{{.LangDirective}}` + "`" + `
- Vulnerability level: ` + "`" + `{{.Company}}` + "`" + ` ` + "`" + `{{.VulnClass}}` + "`" + ` ` + "`" + `{{.Severity}}` + "`" + ` ` + "`" + `{{.Summary}}` + "`" + ` ` + "`" + `{{.PoC}}` + "`" + ` ` + "`" + `{{.LangDirective}}` + "`" + `

` + "`" + `{{.LangDirective}}` + "`" + ` injects the target-language instruction (Simplified, Traditional, or English), so the prompts themselves are language-independent and a single copy serves all three.
`

const promptReportLevelTmpl = `You are a senior penetration-testing report writer for {{.Company}}, specializing in professional, client-readable, and technically rigorous penetration-testing reports. Output strictly JSON only, with no Markdown code block.
===USER===
Write the report overview using the following project and finding list.

Project name: {{.Project}}
Client name: {{.Client}}
Finding list:
{{.FindingsList}}
Requirements (professional delivery tone; do not use phrasing such as "the test was performed today"):
- background: a string array of two paragraphs. The first paragraph states that the client engaged {{.Company}} to conduct penetration testing and gives the testing objectives. The second paragraph describes the test scope and the combination of manual and tool-assisted testing. Each paragraph is 60-130 words.
- service_result: one paragraph summarizing which systems were tested and the number and severity distribution of the findings.
- remediation_conclusion: one paragraph of remediation conclusion (for an initial penetration test, all findings are pending remediation; recommend prompt remediation and retesting).
- checklist_hits: a string array of standard test-item names inferred from the finding types (for example "SQL Injection", "Unauthorized Access", "Authentication Bypass", "Sensitive Information Disclosure", "Command Injection", "Cross-Site Scripting (XSS)", "Vertical Privilege Escalation").

Output JSON only: {"background":["...","..."],"service_result":"...","remediation_conclusion":"...","checklist_hits":["...","..."]}

Output-language requirement: {{.LangDirective}}
`

const promptVulnTmpl = `You are a senior penetration-testing report writer for {{.Company}}. Rewrite the raw vulnerability record as delivery-grade report prose - written by a senior consultant for the client: refined, professional, and strictly on point. Output strictly JSON only, with no Markdown code block, no pleasantries, no disclaimers, and no generic filler. Original text such as HTTP requests, commands, and responses must be preserved exactly and must never be fabricated.
===USER===
Rewrite the following vulnerability as report prose. The core requirement is refinement, specificity, and restraint - prefer omission over padding; do not pad the count, do not pad with filler, and do not state the obvious. Every sentence must carry real information specific to this target and this vulnerability.

Vulnerability type / original name: {{.VulnClass}}
Severity: {{.Severity}}
Original summary:
{{.Summary}}
Original reproduction steps / evidence:
{{.PoC}}

Field requirements:
- title: a standardized, professional vulnerability name (10-26 words, for example "External Account Authentication Bypass Leading to Administrator Takeover"). If the original name is already standardized (including a CVE identifier), keep it or make a minor adjustment; do not copy the summary.
- target: the affected host or URL, extracted from the summary and evidence; leave it as an empty string when it cannot be determined.
- description: one paragraph of 2-3 sentences. It must state only three things - what the vulnerability is, where it occurs, and its root cause. Do not stack OWASP categories, do not repeat the remediation, and do not pad with generalities.
- module: the functional module it belongs to (short, for example "External account login / unauthorized API").
- risk_analysis: 2-4 entries, one sentence each, describing the concrete consequences this specific target and vulnerability allow - who can do what, and which data or privileges they obtain. Write fewer entries when there is no concrete consequence; do not use filler such as "may affect system security".
- remediation: 2-4 entries, one sentence each, specific and actionable - which point to change, how to change it, and what to validate. Do not use empty advice such as "increase security awareness" or "perform regular security audits".
- CVSS 3.1: cvss_score (number), cvss_vector (for example "AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"), and cvss_metrics (eight English values, for example {"AV":"Network","AC":"Low","PR":"None","UI":"None","S":"Unchanged","C":"High","I":"High","A":"High"}).

Do not output test_process - the platform generates the reproduction steps directly from real evidence, so return "test_process":[].

Output JSON only: {"title":"...","target":"...","description":["..."],"module":"...","test_process":[],"risk_analysis":["..."],"remediation":["..."],"cvss_score":0,"cvss_vector":"...","cvss_metrics":{...}}

Output-language requirement: {{.LangDirective}}
`
