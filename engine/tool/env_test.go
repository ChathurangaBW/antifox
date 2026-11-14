package tool

import (
	"strings"
	"testing"
)

func envValue(env []string, key string) string {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return kv[len(key)+1:]
		}
	}
	return ""
}

func countKey(env []string, key string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			n++
		}
	}
	return n
}

// Ported from Norma's TestWithEnvNonInteractiveDefaults, plus the case that
// matters in this deployment: an operator-exported pager must not leak through.
func TestWithEnvNonInteractiveDefaults(t *testing.T) {
	t.Setenv("PAGER", "less")
	env := withEnv(nil)
	want := map[string]string{
		"PAGER": "cat", "GIT_PAGER": "cat", "SYSTEMD_PAGER": "cat",
		"DEBIAN_FRONTEND": "noninteractive",
	}
	for k, v := range want {
		if got := envValue(env, k); got != v {
			t.Fatalf("%s=%q, want %q", k, got, v)
		}
	}
	if n := countKey(env, "PAGER"); n != 1 {
		t.Fatalf("inherited PAGER not dropped: %d entries", n)
	}
	host := withEnv([]string{"PAGER=more", "HTTP_PROXY=http://127.0.0.1:8788"})
	if got := envValue(host, "PAGER"); got != "more" {
		t.Fatalf("host override lost: PAGER=%q", got)
	}
	if got := envValue(host, "HTTP_PROXY"); got != "http://127.0.0.1:8788" {
		t.Fatalf("host var lost: HTTP_PROXY=%q", got)
	}
	if got := envValue(host, "GIT_PAGER"); got != "cat" {
		t.Fatalf("default must still apply when the host is silent: GIT_PAGER=%q", got)
	}
	if n := countKey(host, "PAGER"); n != 1 {
		t.Fatalf("duplicate PAGER entries: %d", n)
	}
}
