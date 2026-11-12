package report

import "testing"

func TestCanonSeverityAliases(t *testing.T) {
	cases := map[string]string{
		"Critical":        "critical",
		"严重":              "critical",
		"嚴重":              "critical",
		"Serious":         "critical",
		"High":            "high",
		"High risk":       "high",
		"高危":              "high",
		"Medium":          "medium",
		"Medium risk":     "medium",
		"中危":              "medium",
		"It's dangerous.": "medium",
		"Low":             "low",
		"Low risk":        "low",
		"Low risk.":       "low",
		"低危":              "low",
		"Info":            "info",
		"Information":     "info",
		"信息":              "info",
		"unknown":         "info",
	}
	for in, want := range cases {
		if got := canonSeverity(in); got != want {
			t.Errorf("canonSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeNameFullWidthParens(t *testing.T) {
	cases := map[string]string{
		"Acme（HK）":  "Acme(HK)",
		"Acme (HK)": "Acme(HK)",
		"Acme　(HK)": "Acme　(HK)", // U+3000 is not stripped, matching upstream
	}
	for in, want := range cases {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanTitleSeverityPrefixes(t *testing.T) {
	cases := map[string]string{
		"【高危】SQL Injection":         "SQL Injection",
		"[High risk] SQL Injection": "SQL Injection",
		"CRITICAL: SQL Injection":   "SQL Injection",
		"严重：SQL 注入":                 "SQL 注入",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
