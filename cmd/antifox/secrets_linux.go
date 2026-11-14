//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"antifox/server"
)

// secretsFDEnv names the inherited file descriptor carrying the secrets across
// the re-exec. It holds only a number, never a secret.
const secretsFDEnv = "ANTIFOX_SECRETS_FD"

// maxSecretsPayload keeps the handoff well under a pipe's buffer, so the write
// completes before exec without a reader.
const maxSecretsPayload = 32 << 10

// handoffSecrets keeps the startup secrets out of /proc/<pid>/environ.
//
// server.ScrubSecretEnv removes the secrets from the Go runtime's environment,
// so child processes do not inherit them. But /proc/<pid>/environ shows the
// environment the process was started with, and agent shells run as the same
// user, so `cat /proc/1/environ` in an agent shell still revealed the database
// DSN and model keys. This closes that: on first start, the binary writes the
// secrets into a pipe and re-executes itself (same PID, so a supervisor or
// Docker sees no change) with an environment that does not contain them. The
// new image reads them back from the pipe into the runtime environment only,
// where ScrubSecretEnv removes them as before.
//
// Every failure falls back to the previous behavior (secrets stay in the start
// environment); it never stops the server from starting.
func handoffSecrets() {
	if v := os.Getenv(secretsFDEnv); v != "" {
		if err := restoreSecrets(v); err != nil {
			fmt.Fprintf(os.Stderr, "[secrets] restore after re-exec failed: %v\n", err)
		}
		return
	}
	secrets := map[string]string{}
	for _, k := range server.SecretEnvKeys() {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			secrets[k] = v
		}
	}
	if len(secrets) == 0 {
		return
	}
	data, err := json.Marshal(secrets)
	if err != nil || len(data) > maxSecretsPayload {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	if _, err := w.Write(data); err != nil {
		r.Close()
		w.Close()
		return
	}
	w.Close()
	fd := r.Fd()
	// Keep the read end open across exec.
	if _, _, e := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0); e != 0 {
		r.Close()
		return
	}
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok {
			if _, secret := secrets[k]; secret || k == secretsFDEnv {
				continue
			}
		}
		env = append(env, kv)
	}
	env = append(env, secretsFDEnv+"="+strconv.FormatUint(uint64(fd), 10))
	err = syscall.Exec(exe, os.Args, env) // returns only on failure
	fmt.Fprintf(os.Stderr, "[secrets] re-exec failed, continuing with secrets in the start environment: %v\n", err)
	r.Close()
}

// restoreSecrets reads the handoff from the inherited descriptor into the
// runtime environment. With cgo disabled, os.Setenv changes only Go's copy, so
// /proc/<pid>/environ stays clean.
func restoreSecrets(fdStr string) error {
	_ = os.Unsetenv(secretsFDEnv)
	n, err := strconv.ParseUint(fdStr, 10, 32)
	if err != nil {
		return fmt.Errorf("bad %s: %w", secretsFDEnv, err)
	}
	f := os.NewFile(uintptr(n), "secrets")
	if f == nil {
		return fmt.Errorf("invalid descriptor %d", n)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSecretsPayload+1))
	if err != nil {
		return err
	}
	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, k := range server.SecretEnvKeys() {
		allowed[k] = true
	}
	for k, v := range secrets {
		if allowed[k] {
			_ = os.Setenv(k, v)
		}
	}
	return nil
}
