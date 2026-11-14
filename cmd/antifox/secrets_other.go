//go:build !linux

package main

// handoffSecrets is Linux-only (it re-executes the binary via execve); other
// platforms keep the secrets in the start environment, scrubbed from children
// by server.ScrubSecretEnv as before.
func handoffSecrets() {}
