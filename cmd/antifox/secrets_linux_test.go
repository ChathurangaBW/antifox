//go:build linux

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The test binary re-runs itself as a helper: with ANTIFOX_HANDOFF_HELPER set,
// TestMain calls handoffSecrets (which re-execs) and then reports what the
// re-executed process sees in /proc/self/environ and in its runtime env.
func TestMain(m *testing.M) {
	if os.Getenv("ANTIFOX_HANDOFF_HELPER") == "1" {
		handoffSecrets()
		start, _ := os.ReadFile("/proc/self/environ")
		leaked := strings.Contains(string(start), "sk-handoff-test") || strings.Contains(string(start), "postgres://handoff")
		os.Stdout.WriteString("pid-env-leaked=" + boolStr(leaked) + "\n")
		os.Stdout.WriteString("runtime-key=" + os.Getenv("OPENAI_API_KEY") + "\n")
		os.Stdout.WriteString("runtime-dsn=" + os.Getenv("ANTIFOX_PG_DSN") + "\n")
		os.Stdout.WriteString("fd-var=" + os.Getenv(secretsFDEnv) + "\n")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestHandoffSecretsKeepsStartEnvironmentClean(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		"ANTIFOX_HANDOFF_HELPER=1",
		"OPENAI_API_KEY=sk-handoff-test",
		"ANTIFOX_PG_DSN=postgres://handoff@db/x",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"pid-env-leaked=false",
		"runtime-key=sk-handoff-test",
		"runtime-dsn=postgres://handoff@db/x",
		"fd-var=\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in helper output:\n%s", want, got)
		}
	}
}
