package server

import "testing"

func TestNormalizeSeverityAliases(t *testing.T) {
	cases := map[string]string{
		"Critical":    "high",
		"严重":          "high",
		"嚴重":          "high",
		"Serious":     "high",
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
