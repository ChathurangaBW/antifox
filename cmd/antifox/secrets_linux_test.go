//go:build linux

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary re-runs itself as a helper: with ANTIFOX_HANDOFF_HELPER set,
// TestMain calls handoffSecrets (which may re-exec) and then reports what the
// resulting process sees in /proc/self/environ and in its runtime env.
func TestMain(m *testing.M) {
	if os.Getenv("ANTIFOX_HANDOFF_HELPER") == "1" {
		handoffSecrets()
		start, _ := os.ReadFile("/proc/self/environ")
		leaked := false
		for _, kv := range strings.Split(string(start), "\x00") {
			if strings.HasPrefix(kv, "OPENAI_API_KEY=") || strings.HasPrefix(kv, "ANTIFOX_PG_DSN=") {
				leaked = true
			}
		}
		os.Stdout.WriteString("pid-env-leaked=" + boolStr(leaked) + "\n")
		os.Stdout.WriteString("runtime-key-len=" + itoa(len(os.Getenv("OPENAI_API_KEY"))) + "\n")
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// runHelper runs the helper with a hard deadline, so a handoff that blocks
// fails the test instead of hanging it. It returns stdout and stderr.
func runHelper(t *testing.T, extraEnv ...string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	// Start from this process's environment minus the secret variables, so a DSN
	// or key set for the surrounding test run cannot change the payload size.
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "ANTIFOX_PG_DSN", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTIFOX_LLM_PROXY", secretsFDEnv:
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, append([]string{"ANTIFOX_HANDOFF_HELPER=1"}, extraEnv...)...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("helper did not finish within the deadline: the handoff blocked")
		}
		t.Fatalf("helper failed: %v\nstdout:%s\nstderr:%s", err, out.String(), errOut.String())
	}
	return out.String(), errOut.String()
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in:\n%s", w, got)
		}
	}
}

func TestHandoffSecretsKeepsStartEnvironmentClean(t *testing.T) {
	const key = "sk-handoff-test-value"
	out, errOut := runHelper(t,
		"OPENAI_API_KEY="+key,
		"ANTIFOX_PG_DSN=postgres://handoff@db/x",
	)
	mustContain(t, out, "pid-env-leaked=false", "runtime-key-len=21", "runtime-dsn=postgres://handoff@db/x", "fd-var=\n")
	if strings.Contains(out+errOut, key) {
		t.Fatal("a secret value appeared in the helper's output or logs")
	}
}

// A payload just under the limit still hands off.
func TestHandoffAtSizeLimit(t *testing.T) {
	// JSON adds {"OPENAI_API_KEY":""} = 21 bytes around the value.
	key := strings.Repeat("k", maxSecretsPayload-21)
	out, errOut := runHelper(t, "OPENAI_API_KEY="+key)
	mustContain(t, out, "pid-env-leaked=false", "runtime-key-len="+itoa(len(key)))
	if strings.Contains(errOut, "skipped") || strings.Contains(out+errOut, key) {
		t.Fatalf("at-limit handoff skipped or leaked:\n%s", errOut)
	}
}

// Over the limit: no hang, the server still gets its secrets, the fallback is
// logged without the value.
func TestHandoffOverSizeLimitFallsBack(t *testing.T) {
	key := strings.Repeat("k", maxSecretsPayload)
	out, errOut := runHelper(t, "OPENAI_API_KEY="+key)
	mustContain(t, out, "runtime-key-len="+itoa(len(key)))
	mustContain(t, errOut, "handoff skipped: payload")
	if strings.Contains(errOut, key) {
		t.Fatal("the fallback log line contains the secret")
	}
}

// A restore that cannot read its descriptor logs and carries on.
func TestHandoffRestoreFailure(t *testing.T) {
	out, errOut := runHelper(t, secretsFDEnv+"=987")
	mustContain(t, out, "fd-var=\n", "runtime-key-len=0")
	mustContain(t, errOut, "restore after re-exec failed")
}

// writeHandoff never waits for a reader: data larger than the pipe can hold
// fails at once instead of blocking.
func TestWriteHandoffDoesNotBlockOnFullPipe(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		fd, err := writeHandoff(make([]byte, 2<<20)) // far beyond any pipe buffer
		if err == nil {
			_ = syscall.Close(fd)
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("an oversize write into an unread pipe reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writeHandoff blocked on a full pipe")
	}

	fd, err := writeHandoff([]byte(`{"OPENAI_API_KEY":"x"}`))
	if err != nil {
		t.Fatalf("small payload: %v", err)
	}
	_ = syscall.Close(fd)
}
