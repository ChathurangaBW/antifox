package report

// Retroactive finding extraction: pull confirmed vulnerabilities out of a task's
// agent transcript (synthesis + worker conclusions) that were described but never
// recorded via report_finding — so they can be back-filled as finding nodes.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"antifox/db"
)

// ExtractedFinding is one vuln pulled from the transcript.
type ExtractedFinding struct {
	VulnClass string `json:"vulnclass"`
	Severity  string `json:"severity"` // high|medium|low
	Summary   string `json:"summary"`
	Target    string `json:"target"`
	Evidence  string `json:"evidence"`
}

const sysExtract = "You are a senior penetration-testing specialist. Extract the confirmed findings from the penetration-testing execution and summary records. Output strictly JSON only, with no Markdown code block. Evidence must preserve the real original text from the records and must never be fabricated."

const userExtract = `From the following penetration-testing execution and summary records, extract all [confirmed findings] and [proven exposure / information-disclosure risks] (excluding unverified guesses and negative conclusions).

Recorded findings (do not repeat these):
%s
Execution / summary records:
%s

Requirements:
- Each entry is one confirmed finding: vulnclass (vulnerability type/name, e.g. "Stored XSS", "Hard-coded credentials", "Unauthorized access"), severity (critical|high|medium|low; critical = directly leads to complete compromise of the system or data), summary (one-sentence summary including the affected object), target (affected host or URL), evidence (key PoC / request / command / response text, preserving the real content). to the system./Data is completely lost.), summary(A summary of the sentence, containing the objects affected), target(Affected host or URL), evidence(Chile PoC/Request/Command/Respond to the original, keep the true content).
- Extract only findings and exposures that are **explicitly verified** in the records; do not repeat semantically identical entries from the "Recorded findings" list above.
- **Exposure / information-disclosure findings must also be extracted**: unauthorized sensitive, management, or debug interfaces that were actually accessed with response evidence in the records, Swagger/API documentation, exposed configuration or backup/source files, sensitive data leaks, and so on, with severity assigned by sensitivity (ordinary information disclosure=low, unauthorized sensitive/management surface=medium, direct takeover or exposed credentials/keys=high). Pure speculation is still excluded (version/CVE match, "looks injectable"); merely "an interface exists" without unauthorized exposure does not count.
- If there are no new findings or exposures, output an empty findings array.

Output JSON only: {"findings":[{"vulnclass":"...","severity":"high","summary":"...","target":"...","evidence":"..."}]}`

// ExtractFindings asks the active LLM to extract confirmed vulns from the
// transcript, excluding ones already recorded (passed as short labels).
func ExtractFindings(ctx context.Context, pg *db.DB, transcript string, existing []string) ([]ExtractedFinding, error) {
	if strings.TrimSpace(transcript) == "" {
		return nil, fmt.Errorf("Unanalytical Session Record")
	}
	var ex strings.Builder
	for _, e := range existing {
		ex.WriteString("- ")
		ex.WriteString(e)
		ex.WriteString("\n")
	}
	if ex.Len() == 0 {
		ex.WriteString("(None)\n")
	}
	raw, err := complete(ctx, pg, sysExtract, fmt.Sprintf(userExtract, ex.String(), transcript), 4000)
	if err != nil {
		return nil, err
	}
	var out struct {
		Findings []ExtractedFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("AI Output Resolution Failed")
	}
	return out.Findings, nil
}
