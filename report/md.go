package report

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"antifox/db"
)

// mdLeadNumRe Get rid of the system title. DOCX Chapter prefix for layout (e.g. "3.1 ")——MD It's an independent delivery, self-made. 1 Start numbering.
var mdLeadNumRe = regexp.MustCompile(`^\d+(\.\d+)*\s+`)

// BuildMarkdown Generate [standard of delivery]Markdown Text of the report + Annexed list of exhibits(Relative Name→Source Disk Path).
// Layout alignment manual delivery: header → Group by Objective → Every loophole(Properties Table + 7 Subsections) → Footer.
// Evidence chart with relative path assets/<name> Quoted by GenerateMarkdownFile Into the same one. zip(Not anymore. base64 Embedded,
// Hold .md It's clean, it's small. Typora/VSCode Direct editing, relative resolution of pictures).
func BuildMarkdown(ctx context.Context, pg *db.DB, in *GenInput) (string, map[string]string) {
	doc := BuildDoc(ctx, pg, in)
	loc := doc.Meta.Locale
	L := func(zh, tw, en string) string { return pick(loc, zh, tw, en) }
	assets := map[string]string{}
	var b strings.Builder

	// ── Synchronising folder ──
	b.WriteString("# " + doc.Meta.ReportTitle + "\n\n")
	b.WriteString("> **" + L("Author", "Author", "Prepared by") + "**:" + doc.Meta.CompanyName + "  \n")
	b.WriteString("> **" + L("Date of report", "Date of report", "Date") + "**:" + doc.Meta.Date +
		"　·　**" + L("Version", "Version", "Version") + "**:" + doc.Meta.Version + "  \n")
	var crit, high, med, low int
	for _, s := range doc.Summary.Systems {
		crit += s.Crit
		high += s.High
		med += s.Med
		low += s.Low
	}
	b.WriteString("> **" + L("Vulnerability Summary", "Vulnerability Summary", "Findings") + "**:" +
		fmt.Sprintf("%s %d · %s %d · %s %d · %s %d(%s %d)\n",
			L("Critical", "Critical", "Critical"), crit, L("High", "High", "High"), high,
			L("Medium", "Medium", "Medium"), med, L("Low", "Low", "Low"), low,
			L("Total", "Total", "total"), crit+high+med+low))
	b.WriteString("\n---\n\n")

	s1 := L("Vulnerability Description", "Vulnerability Description", "Description")
	s3 := L("Affected Module", "Affected Module", "Module")
	s4 := L("Testing Process", "Testing Process", "Testing Process")
	s5 := L("Risk Analysis", "Risk Analysis", "Risk Analysis")
	s6 := L("Remediation Recommendations", "Remediation Recommendations", "Remediation")
	s7 := L("Remediation Status", "Remediation Status", "Remediation Status")
	tTarget := L("Affected Target", "Affected Target", "Target")
	tRisk := L("Risk Level", "Risk Level", "Severity")
	tDate := L("Discovery Date", "Discovery Date", "Found")

	sysIdx := 0
	for _, sysd := range doc.SystemsDetail {
		sysIdx++
		b.WriteString(fmt.Sprintf("## %d %s\n\n", sysIdx, mdLeadNumRe.ReplaceAllString(sysd.Name, "")))
		vulnIdx := 0
		for _, v := range sysd.Vulns {
			vulnIdx++
			// Title:<sys>.<vuln> <name>(<severity>/CVSSValue<score>)——Self-made. 1 Start Numbering
			paren := v.Severity
			if v.CVSSScore > 0 {
				paren += "/" + L("CVSSValue", "CVSSValue", "CVSS ") + fmt.Sprintf("%.1f", v.CVSSScore)
			}
			b.WriteString(fmt.Sprintf("### %d.%d %s(%s)\n\n", sysIdx, vulnIdx, v.Title, paren))
			// Properties Table
			b.WriteString("| | |\n|---|---|\n")
			b.WriteString("| **" + tTarget + "** | " + mdCell(nz(v.Target, v.URL)) + " |\n")
			b.WriteString("| **" + tRisk + "** | **" + v.Severity + "** |\n")
			b.WriteString("| **" + tDate + "** | " + v.FoundDate + " |\n\n")
			// 1. Hole Description
			b.WriteString("**1. " + s1 + "**\n\n")
			for _, p := range v.Description {
				b.WriteString(p + "\n\n")
			}
			// 2. URL
			b.WriteString("**2. URL**\n\n")
			b.WriteString(nz(v.URL, v.Target) + "\n\n")
			// 3. The module to which you belong
			b.WriteString("**3. " + s3 + "**\n\n")
			b.WriteString(defaultStr(v.Module, "—") + "\n\n")
			// 4. Test process (step text) + Word by Word Request/Response + (Exemplary of evidence)
			b.WriteString("**4. " + s4 + "**\n\n")
			for _, blk := range v.TestProcess {
				switch blk.Type {
				case "code":
					b.WriteString("```\n" + strings.TrimRight(blk.Content, "\n") + "\n```\n\n")
				case "image":
					if blk.Path != "" && fileExists(blk.Path) {
						rel := filepath.Base(blk.Path)
						assets[rel] = blk.Path
						b.WriteString("![" + mdEscapeAlt(blk.Caption) + "](assets/" + rel + ")\n")
						if blk.Caption != "" {
							b.WriteString("*" + blk.Caption + "*\n")
						}
						b.WriteString("\n")
					}
				default:
					b.WriteString(blk.Content + "\n\n")
				}
			}
			// 5. Risk analysis
			b.WriteString("**5. " + s5 + "**\n\n")
			for _, r := range v.RiskAnalysis {
				b.WriteString("- " + r + "\n")
			}
			b.WriteString("\n")
			// 6. Recommendations for rehabilitation
			b.WriteString("**6. " + s6 + "**\n\n")
			for _, r := range v.Remediation {
				b.WriteString("- " + r + "\n")
			}
			b.WriteString("\n")
			// 7. Rehabilitation
			b.WriteString("**7. " + s7 + "**\n\n")
			b.WriteString(v.FixStatus + "\n\n")
			b.WriteString("---\n\n")
		}
	}
	b.WriteString("*" + fmt.Sprintf(
		L("This report was prepared by %s on %s.", "This report was prepared by %s on %s.", "Report prepared by %s on %s."),
		doc.Meta.CompanyName, doc.Meta.Date) + "*\n")
	return b.String(), assets
}

// GenerateMarkdownFile Generate MD Delivery package zip(report.md + assets/ Evidence chart) zip Path.
// Pure Go, None python dependency; pictures are accompanied by real files, relative references, depressed and then Typora/VSCode Directly readable and editable.
func GenerateMarkdownFile(ctx context.Context, pg *db.DB, in *GenInput, workDir, taskID string) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", fmt.Errorf("Failed to create report directory: %w", err)
	}
	md, assets := BuildMarkdown(ctx, pg, in)
	zipPath := filepath.Join(workDir, fmt.Sprintf("task-%s-%s.zip", safeName(taskID), nowStamp(ctx)))
	zf, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("Failed to create report pack: %w", err)
	}
	defer zf.Close()
	zw := zip.NewWriter(zf)
	mw, err := zw.Create("report.md")
	if err != nil {
		return "", fmt.Errorf("Writing report.md Failed: %w", err)
	}
	if _, err := mw.Write([]byte(md)); err != nil {
		return "", fmt.Errorf("Writing report.md Failed: %w", err)
	}
	for rel, src := range assets {
		data, err := os.ReadFile(src)
		if err != nil {
			continue // The missing picture skips without affecting the package.
		}
		aw, err := zw.Create("assets/" + rel)
		if err != nil {
			continue
		}
		_, _ = aw.Write(data)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("Packaging failed: %w", err)
	}
	return zipPath, nil
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// mdCell The cell in the converted table will break markdown table.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// mdEscapeAlt Clear Picture alt Text ![]() Syntax:
func mdEscapeAlt(s string) string {
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, "\n", " ")
	if strings.TrimSpace(s) == "" {
		return "evidence"
	}
	return s
}
