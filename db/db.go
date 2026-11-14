// Package db is the PostgreSQL data source for AntiFox (Replace old graph Single file SQLite).
// It opens connections, applies. schema, and seed Internal agent with the variable directory.
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"antifox/config"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql driver ("pgx")
)

//go:embed schema.sql
var schemaSQL string

// Maximum connection pool (see Open). Conservative values: single cases deployed sufficient to PG Other client leftovers.
const (
	maxOpenConns    = 25
	maxIdleConns    = 10
	connMaxLifetime = 55 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

// DSN resolves the PostgreSQL connection string and reports where it came from.
// Precedence: env ANTIFOX_PG_DSN > config file (config.json). There is no
// built-in default — it errors if neither source is configured.
func DSN() (dsn, source string, err error) {
	return config.PostgresDSN()
}

// DB wraps the shared *sql.DB. PG handles its own connection pool + concurrency
// (MVCC), so unlike the old SQLite store there is no process-wide write mutex.
type DB struct{ *sql.DB }

// ensureDatabase connects to the postgres system database and creates the target
// database if it does not exist. dsn must be a postgres:// URL.
func ensureDatabase(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil // unparseable DSN — let the normal Open fail with a clear error
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" || dbName == "postgres" {
		return nil
	}
	// connect to the postgres maintenance database instead
	adminDSN := *u
	adminDSN.Path = "/postgres"
	admin, err := sql.Open("pgx", adminDSN.String())
	if err != nil {
		return nil // best-effort; let Open surface the real error
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		return nil
	}
	var exists bool
	_ = admin.QueryRow(`SELECT true FROM pg_database WHERE datname=$1`, dbName).Scan(&exists)
	if !exists {
		if _, err := admin.Exec(`CREATE DATABASE "` + dbName + `"`); err != nil {
			return fmt.Errorf("create database %q: %w", dbName, err)
		}
	}
	return nil
}

// Open connects, applies the schema (idempotent), and seeds builtin rows.
func Open(dsn string) (*DB, error) {
	if err := ensureDatabase(dsn); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping postgres (%s): %w", config.Redact(dsn), err)
	}
	// Maximum number of connective pools: limit co-opted database connections and avoid requesting storms to run out of connections leading to failed queries.PG Default
	// max_connections=100,Leave a full amount to other clients here.psql/pg_dump/backup;free connection recovery
	// Avoid long-term presence, with the maximum life of connections preventing half-dead network layers of connections.
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	// Serialize schema migration + seeding across concurrent openers (e.g. parallel
	// test packages) using a session-level advisory lock. The lock is released
	// automatically when the connection is returned to the pool or closed.
	if _, err := sqlDB.Exec(`SELECT pg_advisory_lock(7337741001)`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("advisory lock: %w", err)
	}
	// pgx runs multi-statement Exec via the simple protocol when there are no args.
	if _, err := sqlDB.Exec(schemaSQL); err != nil {
		sqlDB.Exec(`SELECT pg_advisory_unlock(7337741001)`) //nolint:errcheck
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	d := &DB{sqlDB}
	if err := d.seedBuiltins(); err != nil {
		sqlDB.Exec(`SELECT pg_advisory_unlock(7337741001)`) //nolint:errcheck
		sqlDB.Close()
		return nil, fmt.Errorf("seed builtins: %w", err)
	}
	if _, err := sqlDB.Exec(`SELECT pg_advisory_unlock(7337741001)`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("advisory unlock: %w", err)
	}
	return d, nil
}

// builtinAgent describes one of the fixed agents and its prompt-variable catalog.
type builtinAgent struct {
	key, name, role, desc string
	vars                  []promptVar
	interactiveShell      bool // Default interactive on construction shell Switches;ON CONFLICT Do not overwrite user follow-up manual switches
	runSeconds            *int // Single time at construction run Wall clock ceiling(sec);nil=Use table default(600),0=Timeless
}

type promptVar struct{ name, desc, example, source string }

// intp Back v ♪ A pointer to give ♪ builtinAgent Optional Fields(Like runSeconds)visible value.
func intp(v int) *int { return &v }

// builtinAgents mirrors docs §5(a). Built-in tools are not stored; only the agent
// and its variable catalog are seeded here. Note: the interactive shell for
// planner/worker/mainagent/auto defaults to true via the interactive_shell_default_v1
// block below (which respects later toggles); interactiveShell here only applies to
// new agents that must start with it enabled.
var builtinAgents = []builtinAgent{
	{"goals", "Goal Decomposition", "goals", "Decomposes a penetration-testing task goal into several independent, verifiable sub-goals.", []promptVar{
		{"EngagementDescription", "Task description (target / background)", "Test the example.com site", "exploration"},
		{"Now", "Current server time (RFC3339)", "2026-06-25T00:00:00Z", "runtime"},
	}, false, nil},
	{"planner", "Planner", "planner", "Reads the current state, determines whether goals are met, and adds exploration intents only for genuinely uncovered new directions (one planning loop per task).", []promptVar{
		{"Goal", "Overall task goal", "Obtain administrator access on example.com", "exploration"},
		{"AssetSummary", "Asset count / type distribution summary (optional)", "domain:3 ip:5 site:2", "distilled"},
	}, false, nil},
	{"mainagent", "Main Agent", "main", "Human interface: observes progress and turns the operator's intent into a hint or a high-priority intent.", []promptVar{
		{"Goal", "Current task goal", "Obtain administrator access on example.com", "exploration"},
		{"AssetSummary", "Opening state summary (optional)", "domain:3 ip:5", "distilled"},
		{"FindingsSummary", "Summary of confirmed findings (optional)", "high:1 medium:2", "distilled"},
	}, false, nil},
	{"worker", "Executor", "worker", "Claims one intent, executes it, writes the discovered facts and findings back to the knowledge graph, and then stops.", []promptVar{
		{"ProxyAddr", "Recording proxy address (drives the dual-text switch)", "127.0.0.1:8080", "runtime"},
		{"WorkerName", "Worker self-identifier (optional)", "worker-1", "runtime"},
	}, false, intp(1200)}, // worker single-round wall-clock default 1200s (hard targets and slow networks often exceed 600s; see AntiFox 0.3.3)
	// Auto: the built-in platform-operator agent. It does not take part in the penetration
	// orchestration loop; it is driven from the chat page and operates the platform via tools.
	{"auto", "Auto", "assistant", "Platform operator assistant: uses tools to manage tasks (create / inspect / pause / hint) and assets, and to create or modify skills, custom tools, and MCP servers.", nil, false, nil},
	// Penetration testing: the built-in independent penetration agent. Driven from the chat
	// page, a single operator runs the entire chain from reconnaissance to closeout,
	// planning, executing, and adversarially verifying its own work. The interactive
	// shell is enabled by default.
	{"pentest", "Penetration Testing", "assistant", "Independent penetration-testing agent: one operator covers the entire chain from reconnaissance to closeout - reconnaissance -> attack-surface discovery -> exploitation -> validation -> closeout - planning, executing, and adversarially verifying its own work.", nil, true, intp(0)},
}

// seedBuiltins inserts the fixed built-in agents and their variable catalog (idempotent).
func (d *DB) seedBuiltins() error {
	for _, a := range builtinAgents {
		var agentID int64
		err := d.QueryRow(`
INSERT INTO agents(key, name, description, role, builtin, enabled, interactive_shell, run_seconds)
VALUES ($1, $2, NULLIF($3,''), $4, true, true, $5, COALESCE($6, 600))
ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description
RETURNING id`, a.key, a.name, a.desc, a.role, a.interactiveShell, a.runSeconds).Scan(&agentID)
		if err != nil {
			return fmt.Errorf("agent %s: %w", a.key, err)
		}
		for _, v := range a.vars {
			if _, err := d.Exec(`
INSERT INTO agent_prompt_vars(agent_id, var_name, description, example, source)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (agent_id, var_name) DO UPDATE
  SET description = EXCLUDED.description, example = EXCLUDED.example, source = EXCLUDED.source`,
				agentID, v.name, v.desc, v.example, v.source); err != nil {
				return fmt.Errorf("agent %s var %s: %w", a.key, v.name, err)
			}
		}
	}
	// Drop catalog entries for variables that were renamed, so the white-list no
	// longer advertises a name templates can't resolve (EngagementTitle→Description).
	if _, err := d.Exec(`DELETE FROM agent_prompt_vars WHERE var_name IN ('EngagementTitle', 'CoverageGaps')`); err != nil {
		return fmt.Errorf("cleanup renamed vars: %w", err)
	}
	// Default-on interactive_shell for the runtime agents (planner/worker/mainagent/auto)
	// ONCE — respects a later user toggle-off (guarded by a settings flag). goals(one-shot
	// decomposer) stays off. Runs after the column exists (schema applied before seed).
	if v, _, _ := d.GetSetting("interactive_shell_default_v1"); v != "true" {
		if _, err := d.Exec(`UPDATE agents SET interactive_shell=true WHERE key IN ('planner','worker','mainagent','auto')`); err != nil {
			return fmt.Errorf("seed interactive_shell defaults: %w", err)
		}
		_ = d.SetSetting("interactive_shell_default_v1", "true")
	}
	// Seed built-in MCP servers — insert-if-absent so user edits survive restart.
	// browser (Playwright): headless browser automation, MITM recording proxy auto-
	// injected when traffic capture is on (server.Manager.syncBrowserMCPProxy).
	if _, err := d.Exec(`
INSERT INTO mcp_servers(name, transport, command, args, env, enabled)
VALUES ('browser', 'stdio', 'npx', $1, '{}', true)
ON CONFLICT (name) DO NOTHING`,
		`["@playwright/mcp","--headless"]`); err != nil {
		return fmt.Errorf("seed browser mcp: %w", err)
	}
	// filesystem: read/write files during pentest (payloads, output, configs).
	if _, err := d.Exec(`
INSERT INTO mcp_servers(name, transport, command, args, env, enabled, description)
VALUES ('filesystem', 'stdio', 'npx', $1, '{}', true, 'Read and write local file systems——Access to payloads, script output, profile found, etc. in permeability tests')
ON CONFLICT (name) DO NOTHING`,
		`["@modelcontextprotocol/server-filesystem","/tmp/antifox-fs"]`); err != nil {
		return fmt.Errorf("seed filesystem mcp: %w", err)
	}
	// github: recon via GitHub API — search repos, find exposed secrets, analyze code.
	// Seeded DISABLED: it needs a real GITHUB_PERSONAL_ACCESS_TOKEN, so leaving it
	// enabled with the placeholder guarantees "connection closed" error spam on every
	// deploy. Operator enables it after setting a token on the MCP page.
	if _, err := d.Exec(`
INSERT INTO mcp_servers(name, transport, command, args, env, enabled, description)
VALUES ('github', 'stdio', 'npx', $1, $2, false, 'GitHub Warehouse reconnaissance——Search for possible leaking certificates, keys, sensitive configurations in the open code repository, or analyze the target organization open source project code (required) MCP Page Filling GITHUB_PERSONAL_ACCESS_TOKEN )')
ON CONFLICT (name) DO NOTHING`,
		`["@modelcontextprotocol/server-github"]`, `{"GITHUB_PERSONAL_ACCESS_TOKEN":"<your-token>"}`); err != nil {
		return fmt.Errorf("seed github mcp: %w", err)
	}
	if err := d.seedBuiltinMCPVisibility(); err != nil {
		return fmt.Errorf("seed mcp visibility: %w", err)
	}
	// NOTE: the placeholder ScopeSentry data-source MCP (empty URL + empty X-API-Key,
	// disabled) is seeded directly in schema.sql §F so a raw `psql < schema.sql` init
	// also gets it. schema.sql is Exec'd on every startup, so it stays idempotent.
	if err := d.seedBuiltinSkillVisibility(); err != nil {
		return fmt.Errorf("seed skill visibility: %w", err)
	}
	if err := d.seedDefaultInterceptRules(); err != nil {
		return fmt.Errorf("seed intercept rules: %w", err)
	}
	d.ensureKnowledgeVectorSchema() // best-effort; degrades to keyword search if pgvector absent
	return nil
}

// ensureKnowledgeVectorSchema creates the pgvector extension + knowledge_chunks
// table (the semantic layer). It is best-effort and NEVER fails boot: CREATE
// EXTENSION vector needs a superuser to have pre-installed it, so on a deployment
// where that hasn't happened the knowledge base degrades to keyword search.
// Records the outcome in the settings flag "knowledge.vector_available".
func (d *DB) ensureKnowledgeVectorSchema() {
	// CREATE EXTENSION IF NOT EXISTS is not safe under concurrency: two openers can
	// both pass the existence check, and the loser fails with a unique violation on
	// pg_extension even though the extension now exists. Re-check before treating
	// the error as "pgvector unavailable", or one lost race disables semantic search.
	if _, err := d.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil && !d.vectorExtensionPresent() {
		log.Printf("[knowledge] pgvector Not Available%v)——Semantic search is disabled, downgraded to keyword search."+
			"Super user execution is required once to enable CREATE EXTENSION vector", err)
		_ = d.SetBool("knowledge.vector_available", false)
		return
	}
	// embedding Uncertainty. vector Columns + Every chunk Remember embed_model/dim;Filter the current model when searching,
	// Change embedder Unstrengthed (old model lines are ignored, indexed and harmonized). Data levels can be accurately scanned.
	if _, err := d.Exec(`
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id          BIGSERIAL PRIMARY KEY,
    doc_id      BIGINT NOT NULL REFERENCES knowledge_docs(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL DEFAULT 0,
    content     TEXT NOT NULL DEFAULT '',
    embedding   vector,
    embed_model TEXT NOT NULL DEFAULT '',
    embed_dim   INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_doc ON knowledge_chunks(doc_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_model ON knowledge_chunks(embed_model);`); err != nil {
		log.Printf("[knowledge] Construction knowledge_chunks Failed%v)——Semantic Search Disable", err)
		_ = d.SetBool("knowledge.vector_available", false)
		return
	}
	_ = d.SetBool("knowledge.vector_available", true)
}

// builtinSkillVisibility maps a shipped skill's directory name → the built-in
// agent keys that should see it by default. The skill FILES themselves live on the
// filesystem (SkillDir, loaded by the engine at runtime); DB only carries this visibility
// binding. Skills omitted here (e.g. playwright-cli, scopesentry) ship invisible by
// default — the user turns them on per-agent when needed. scopesentry additionally
// declares `mcps: ScopeSentry`, which only takes effect once it's made visible and
// that MCP is enabled/configured.
var builtinSkillVisibility = map[string][]string{
	"api-recon": {"auto", "pentest", "worker"},
	// Moving from old AntiFox The method of penetration testing (tools irrelevant, Bash A public command line tool.
	// Default Test Class agent Visibleworker/auto/pentest);planner/mainagent Keep it clean.
	// incident-response(Blue team.) and security-awareness-training(Training orientation) Intentionally not defaulted to see——
	// Installed, user needs on Skills Page agent Open.
	"api-security-testing":        {"worker", "auto", "pentest"},
	"business-logic-testing":      {"worker", "auto", "pentest"},
	"cloud-security-audit":        {"worker", "auto", "pentest"},
	"command-injection-testing":   {"worker", "auto", "pentest"},
	"container-security-testing":  {"worker", "auto", "pentest"},
	"csrf-testing":                {"worker", "auto", "pentest"},
	"deserialization-testing":     {"worker", "auto", "pentest"},
	"file-upload-testing":         {"worker", "auto", "pentest"},
	"idor-testing":                {"worker", "auto", "pentest"},
	"ldap-injection-testing":      {"worker", "auto", "pentest"},
	"mobile-app-security-testing": {"worker", "auto", "pentest"},
	"network-penetration-testing": {"worker", "auto", "pentest"},
	"secure-code-review":          {"worker", "auto", "pentest"},
	"security-automation":         {"worker", "auto", "pentest"},
	"sql-injection-testing":       {"worker", "auto", "pentest"},
	"ssrf-testing":                {"worker", "auto", "pentest"},
	"vulnerability-assessment":    {"worker", "auto", "pentest"},
	"xpath-injection-testing":     {"worker", "auto", "pentest"},
	"xss-testing":                 {"worker", "auto", "pentest"},
	"xxe-testing":                 {"worker", "auto", "pentest"},
}

// seedBuiltinSkillVisibility binds the shipped built-in skills to their default
// agents. Insert-if-absent (ON CONFLICT DO NOTHING) so a user's later toggle-off is
// never resurrected on restart — matches the browser-MCP / intercept-rule seed policy.
func (d *DB) seedBuiltinSkillVisibility() error {
	for skillName, agentKeys := range builtinSkillVisibility {
		for _, key := range agentKeys {
			if _, err := d.Exec(`
INSERT INTO agent_skill_visibility(agent_id, skill_name, enabled)
SELECT id, $2, true FROM agents WHERE key=$1
ON CONFLICT (agent_id, skill_name) DO NOTHING`, key, skillName); err != nil {
				return fmt.Errorf("skill %s → agent %s: %w", skillName, key, err)
			}
		}
	}
	return nil
}

// builtinMCPVisibility maps a shipped MCP server name → the built-in agent keys
// that should see it by default. browser/filesystem/github are useful for pentest
// agents (worker/auto/pentest); planner/mainagent stay clean.
var builtinMCPVisibility = map[string][]string{
	"browser":    {"worker", "auto", "pentest"},
	"filesystem": {"worker", "auto", "pentest"},
	"github":     {"worker", "auto", "pentest"},
}

// seedBuiltinMCPVisibility binds the shipped built-in MCP servers to their default
// agents via agent_visibility. Insert-if-absent (ON CONFLICT DO NOTHING) so a user's
// later toggle-off is never resurrected on restart.
func (d *DB) seedBuiltinMCPVisibility() error {
	for serverName, agentKeys := range builtinMCPVisibility {
		for _, key := range agentKeys {
			if _, err := d.Exec(`
INSERT INTO agent_visibility(agent_id, resource_kind, resource_id, enabled)
SELECT a.id, 'mcp', m.id, true
FROM agents a, mcp_servers m
WHERE a.key=$1 AND m.name=$2
ON CONFLICT (agent_id, resource_kind, resource_id, mcp_tool_name) DO NOTHING`, key, serverName); err != nil {
				return fmt.Errorf("mcp %s → agent %s: %w", serverName, key, err)
			}
		}
	}
	return nil
}

// seedDefaultInterceptRules inserts built-in safety intercept rules once on
// first startup. The seed is gated by a settings flag so user edits (disable,
// delete, re-order) are never overwritten on subsequent restarts.
func (d *DB) seedDefaultInterceptRules() error {
	if v, _, _ := d.GetSetting("intercept_default_rules_v1"); v == "done" {
		return nil
	}
	type rule struct {
		name     string
		target   string // tool_name | tool_input
		typ      string // string | regex
		pattern  string
		action   string
		message  string
		priority int
	}
	rules := []rule{
		// ── System Destructive Command (priority 100) ──────────────────────────────────
		{
			name:     "[Internal] Recursive Force Delete rm -rf",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\brm\b.{0,80}(?:-[a-z]*r[a-z]*f[a-z]*|-[a-z]*f[a-z]*r[a-z]*|--recursive|--no-preserve-root)`,
			action:   "deny",
			message:  "Prohibits the execution of recalculated forced deletion (rm -rf / rm --recursive),Possible permanent damage to systems or target environment",
			priority: 100,
		},
		{
			name:     "[Internal] Remove System Critical Directory",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\brm\b[^"'\n]{0,60}["'\s](/|/etc|/bin|/usr|/boot|/var|/lib|/sys|/proc|/dev|/sbin|/root)`,
			action:   "deny",
			message:  "Deleting System Critical Paths",
			priority: 100,
		},
		{
			name:     "[Internal] Disk Formatting mkfs",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\bmkfs\b`,
			action:   "deny",
			message:  "Disk Formatting Banmkfs)",
			priority: 100,
		},
		{
			name:     "[Internal] Overwrite Disk Device dd",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\bdd\b[^|\n]{0,100}\bof=\s*/dev/[a-zA-Z]`,
			action:   "deny",
			message:  "Prohibition of use dd Overwrite Disk Device",
			priority: 100,
		},
		{
			name:     "[Internal] Fork Bomb",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `:\(\)\s*\{[^}]*:\|:`,
			action:   "deny",
			message:  "Prohibition of enforcement Fork Bomb",
			priority: 100,
		},
		{
			name:     "[Internal] Shutdown / Restart",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\b(?:shutdown|reboot|halt|poweroff|init\s+[06])\b`,
			action:   "deny",
			message:  "Prohibit the execution of a shutdown or restart order",
			priority: 100,
		},
		{
			name:     "[Internal] Kill All Processes",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\bkill\s+-9\s+-1\b|\bkillall\s+-9\b`,
			action:   "deny",
			message:  "Ban kill -9 -1 or killall -9(Kill all processes)",
			priority: 100,
		},
		{
			name:     "[Internal] Disk Erase shred / wipe",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\b(?:shred|wipe)\b[^|\n]{0,80}/dev/[a-zA-Z]`,
			action:   "deny",
			message:  "Ban Disk Device Execution shred/wipe Erase",
			priority: 100,
		},
		{
			name:     "[Internal] Clear firewall rules",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `\biptables\s+(?:-F|--flush)\b|\bnft\s+flush\s+ruleset\b`,
			action:   "deny",
			message:  "Ban on clearing firewall rules.iptables -F / nft flush)",
			priority: 100,
		},
		// ── Database Disruption Operations (priority 90) ─────────────────────────────────
		{
			name:     "[Internal] SQL DROP DATABASE / TABLE / SCHEMA",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\bDROP\s+(?:DATABASE|TABLE|SCHEMA|INDEX|VIEW|TABLESPACE|USER|ROLE)\b`,
			action:   "deny",
			message:  "Prohibition of enforcement DROP Operation, possibly irreversible destruction of database objects",
			priority: 90,
		},
		{
			name:     "[Internal] SQL TRUNCATE",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\bTRUNCATE\s+(?:TABLE\s+)?\w`,
			action:   "deny",
			message:  "Prohibition of enforcement TRUNCATE,Could empty all data in the data sheet",
			priority: 90,
		},
		{
			name:     "[Internal] MongoDB drop / dropDatabase",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\.(?:dropDatabase|dropCollection|drop)\s*\(`,
			action:   "deny",
			message:  "Prohibition of enforcement MongoDB drop Operation",
			priority: 90,
		},
		{
			name:     "[Internal] Redis FLUSHALL / FLUSHDB",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\b(?:FLUSHALL|FLUSHDB)\b`,
			action:   "deny",
			message:  "Prohibition of enforcement Redis FLUSHALL / FLUSHDB,Could empty all cache data",
			priority: 90,
		},
		// ── HTTP Disruptive Request (priority 80) ──────────────────────────────────
		// Agent Send DELETE Three common ways to request:
		//   1. curl -X DELETE / --request DELETE(Bash Tools directly execute or write scripts)
		//   2. Python HTTP Client .delete() Methodology
		//   3. JS/In a generic script. method: 'DELETE' / method="DELETE"
		{
			name:     "[Internal] curl / wget Send DELETE Request",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\bcurl\b[^|\n&;"]{0,300}(?:-X\s*DELETE|--request\s+DELETE|-XDELETE)|\bwget\b[^|\n&;"]{0,300}--method[=\s]+DELETE`,
			action:   "deny",
			message:  "It's forbidden. curl/wget Send HTTP DELETE Request, possible deletion of target system data",
			priority: 80,
		},
		{
			name:     "[Internal] Python HTTP Client DELETE(requests/httpx/aiohttp)",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)\b(?:requests|httpx|aiohttp|urllib\.request)\.delete\s*\(|session\.delete\s*\(|client\.delete\s*\(`,
			action:   "deny",
			message:  "Prohibition of use Python HTTP Client Send DELETE Request",
			priority: 80,
		},
		{
			name:     "[Internal] Synchronising folder HTTP DELETE MethodologyJS/General)",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)axios\.delete\s*\(|method\s*[:=]\s*['"]DELETE['"]`,
			action:   "deny",
			message:  "Prohibit declaration and sending in scripts HTTP DELETE Request",
			priority: 80,
		},
		{
			name:     "[Internal] Batch Empty / Clear interface path",
			target:   "tool_input",
			typ:      "regex",
			pattern:  `(?i)/(?:clear|wipe|flush|purge|truncate|drop|destroy|factory[-_]reset|reset[-_]all)(?:[/?#"'\s]|$)`,
			action:   "deny",
			message:  "It is prohibited to call a batch to empty or destroy a class interface./clear /wipe /flush /purge Wait",
			priority: 80,
		},
	}
	for _, r := range rules {
		if _, err := d.Exec(`
INSERT INTO intercept_rules(name, enabled, priority, match_target, match_type, pattern, action, message, timeout_enabled, timeout_seconds, timeout_action)
VALUES ($1, true, $2, $3, $4, $5, $6, $7, false, 60, 'deny')
ON CONFLICT DO NOTHING`,
			r.name, r.priority, r.target, r.typ, r.pattern, r.action, r.message,
		); err != nil {
			return fmt.Errorf("rule %q: %w", r.name, err)
		}
	}
	return d.SetSetting("intercept_default_rules_v1", "done")
}

// vectorExtensionPresent reports whether the pgvector extension is installed.
func (d *DB) vectorExtensionPresent() bool {
	var ok bool
	if err := d.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')`).Scan(&ok); err != nil {
		return false
	}
	return ok
}
