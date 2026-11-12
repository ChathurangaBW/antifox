package server

import "testing"

func TestNormalizeSeverityAliases(t *testing.T) {
	cases := map[string]string{
		"Critical":    "critical",
		"严重":          "critical",
		"嚴重":          "critical",
		"Serious":     "critical",
		"High risk":   "high",
		"高危":          "high",
		"Medium":      "medium",
		"Moderate":    "medium",
		"中危":          "medium",
		"Low":         "low",
		"Low risk.":   "low",
		"低危":          "low",
		"Info":        "low",
		"Information": "low",
		"信息":          "low",
		"unknown":     "medium",
	}
	for in, want := range cases {
		if got := normalizeSeverity(in); got != want {
			t.Errorf("normalizeSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContainsSimilarDedup(t *testing.T) {
	existing := []string{
		vulnDedupKey("SQL Injection", "SQL injection in /api/v1/login username parameter"),
		vulnDedupKey("Sensitive Information Disclosure", "Exposed .env file at /.env leaks DB credentials"),
		vulnDedupKey("Unauthorized Access", "Unauthenticated access to /api/v1/admin/users"),
	}

	// Distinct findings sharing the same long English VulnClass must NOT be dropped.
	distinct := []struct{ class, summary string }{
		{"SQL Injection", "SQL injection in /api/v1/orders id parameter"},
		{"Sensitive Information Disclosure", "Swagger UI exposed at /swagger-ui.html"},
		{"Unauthorized Access", "Unauthenticated access to /api/v1/metrics"},
	}
	for _, tc := range distinct {
		if containsSimilar(existing, vulnDedupKey(tc.class, tc.summary)) {
			t.Errorf("distinct finding %q / %q was falsely marked duplicate", tc.class, tc.summary)
		}
	}

	// True duplicates (exact or contained/extended summary) MUST be caught.
	dups := []struct{ class, summary string }{
		{"SQL Injection", "SQL injection in /api/v1/login username parameter"},
		{"SQL Injection", "SQL injection in /api/v1/login username parameter allows auth bypass"},
		{"Unauthorized Access", "Unauthenticated access to /api/v1/admin/users endpoint"},
	}
	for _, tc := range dups {
		if !containsSimilar(existing, vulnDedupKey(tc.class, tc.summary)) {
			t.Errorf("duplicate finding %q / %q was not caught", tc.class, tc.summary)
		}
	}
}
