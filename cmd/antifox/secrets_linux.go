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

// maxSecretsPayload is the largest handoff attempted. The payload is written
// into the pipe before exec, when no reader exists yet, so it must fit in the
// pipe's buffer. Linux gives a pipe 64 KiB by default but only one page (4 KiB)
// once the user is over fs.pipe-user-pages-soft, so 4 KiB is the size that
// always fits. Real payloads (a DSN and a few keys) are a few hundred bytes;
// anything larger keeps the previous behavior instead of risking a hang.
const maxSecretsPayload = 4 << 10

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
	if err != nil {
		return
	}
	if len(data) > maxSecretsPayload {
		fmt.Fprintf(os.Stderr, "[secrets] handoff skipped: payload %d bytes exceeds %d; continuing with secrets in the start environment\n", len(data), maxSecretsPayload)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	readFD, err := writeHandoff(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[secrets] handoff skipped, continuing with secrets in the start environment: %v\n", err)
		return
	}
	fd := uintptr(readFD)
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
	_ = syscall.Close(readFD)
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

// writeHandoff puts data into a new pipe and returns the read end, open across
// exec. The pipe is non-blocking and the write is a single attempt: if the pipe
// cannot take all of data at once, it fails immediately rather than waiting for
// a reader that does not exist yet (the reader is this process after exec).
func writeHandoff(data []byte) (int, error) {
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC|syscall.O_NONBLOCK); err != nil {
		return -1, fmt.Errorf("pipe: %w", err)
	}
	n, err := syscall.Write(p[1], data)
	_ = syscall.Close(p[1])
	if err != nil || n != len(data) {
		_ = syscall.Close(p[0])
		if err == nil {
			err = fmt.Errorf("short write: %d of %d bytes", n, len(data))
		}
		return -1, fmt.Errorf("write: %w", err)
	}
	// Keep the read end open across exec.
	if _, _, e := syscall.Syscall(syscall.SYS_FCNTL, uintptr(p[0]), syscall.F_SETFD, 0); e != 0 {
		_ = syscall.Close(p[0])
		return -1, fmt.Errorf("fcntl: %w", e)
	}
	return p[0], nil
}
