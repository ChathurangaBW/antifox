---
name: report-generation
description: Editable content for penetration-testing report generation - the AI rewriting prompts plus the fixed skeleton (methodology, test checklist, tools, reference standards, test personnel), split by language. The report engine reads these files on every generation, so edits take effect immediately.
---

# Report generation (editable content)

This skill stores the **editable content of penetration-testing reports**, read by the generation pipeline in Report Management. After editing, it**takes effect on the next generation** with no restart or recompilation; if any file is missing or corrupted, the engine automatically falls back to the built-in default.

## Structure

- `prompts/report-level.tmpl` - AI prompt for report-level narrative (background / service results / remediation conclusion / hit checklist items).
- `prompts/vuln-narrative.tmpl` - AI prompt for rewriting a single vulnerability (description / reproduction steps / risk / remediation / CVSS).
- `skeleton/<language>/methodology.json` - 2.2 test workflow: `{intro, phases:[{name,desc}]}`.
- `skeleton/<language>/checklist.json` - 2.3 test checklist: `[{category,name,desc}]`. Items the AI judges as hit are rendered as "failed".
- `skeleton/<language>/tools.json` - 2.4 test tools: `[{name,desc}]`.
- `skeleton/<language>/standards.json` - 2.5 reference standards: `["..."]`.
- `skeleton/<language>/personnel.json` - 2.1 default test personnel: `[{name,role,contact}]` (values the user enters in the report settings take precedence).

Language directories: `zh-CN` (Simplified) / `zh-TW` (Traditional) / `en-US` (English).

## Prompt template variables

The prompts are Go `text/template` documents and use `===USER===` to separate the system and user sections:

- Report level: `{{.Company}}` `{{.Project}}` `{{.Client}}` `{{.FindingsList}}` `{{.LangDirective}}`
- Vulnerability level: `{{.Company}}` `{{.VulnClass}}` `{{.Severity}}` `{{.Summary}}` `{{.PoC}}` `{{.LangDirective}}`

`{{.LangDirective}}` injects the target-language instruction (Simplified, Traditional, or English), so the prompts themselves are language-independent and a single copy serves all three.
