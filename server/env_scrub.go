package server

import (
	"os"

	"antifox/agent"
	"antifox/db"
)

// secretEnv lists the credentials read from the environment only at startup:
// ANTIFOX_PG_DSN by NewManager (config.PGDSN), the API keys and the LLM proxy by
// agent.FromEnv at the top of New. ANTIFOX_LLM_PROXY is included because a proxy
// URL can carry its own credentials. Ported from ARTEX (#136, #146).
var secretEnv = []string{"ANTIFOX_PG_DSN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "ANTIFOX_LLM_PROXY"}

// ScrubSecretEnv removes secretEnv from the process environment once startup has
// read it. Child processes (agent shells, custom tools, MCP stdio servers) copy
// os.Environ() when they are spawned, so they no longer inherit these values.
// Without this, an agent steered by planted target content could print the
// database password and model keys with a single env command.
//
// Limits: os.Unsetenv changes only the Go runtime's copy. A process running as
// the same user can still read /proc/<pid>/environ of the server, and in Docker
// of PID 1.
func ScrubSecretEnv() {
	for _, k := range secretEnv {
		_ = os.Unsetenv(k)
	}
}

// Option configures New. Only the binary opts into scrubbing: tests create
// several servers in one process and still need ANTIFOX_PG_DSN for later ones.
type Option func(*Server)

// WithSecretScrub makes New drop the startup secrets right after reading them,
// before anything that can spawn a child process starts.
func WithSecretScrub() Option { return func(s *Server) { s.scrubSecrets = true } }

// captureEnvLLM reads the environment model configuration once and keeps it,
// both for the engine's startup precedence and as the fallback profile for
// features that look up a saved profile. It must run before ScrubSecretEnv.
func (s *Server) captureEnvLLM() {
	cfg, ok := agent.FromEnv()
	s.envLLM, s.envLLMOK = cfg, ok
	if !ok {
		db.SetEnvFallbackProfile(nil)
		return
	}
	db.SetEnvFallbackProfile(&db.LLMProfile{
		Name:    "environment",
		Format:  cfg.Provider(),
		Model:   cfg.Model,
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Proxy:   cfg.Proxy,
	})
}

// SecretEnvKeys returns the startup secret variable names, for the binary's
// re-exec handoff (cmd/antifox), which must strip exactly the same set.
func SecretEnvKeys() []string { return append([]string(nil), secretEnv...) }
