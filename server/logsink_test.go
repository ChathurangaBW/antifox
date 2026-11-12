package server

import "testing"

func TestLevelOfEnglishKeywords(t *testing.T) {
	cases := map[string]string{
		"[worker] request failed":     "error",
		"[worker] packet drop":        "error",
		"[worker] connection refused": "error",
		"[worker] host unreachable":   "error",
		"[worker] feature disabled":   "warn",
		"[worker] retry the request":  "warn",
		"[worker] everything is fine": "info",
	}
	for in, want := range cases {
		if got := levelOf(in); got != want {
			t.Errorf("levelOf(%q) = %q, want %q", in, got, want)
		}
	}
}
