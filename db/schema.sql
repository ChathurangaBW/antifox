-- AntiFox PostgreSQL schema (Single Data Source)
-- Idempotent: repeatable through IF NOT EXISTS / OR REPLACE / DROP TRIGGER IF EXISTS.

-- =====================================================================
-- 0. Universal:updated_at Trigger
-- =====================================================================
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- A. Asset level:companies / assets / company_scope
-- =====================================================================

CREATE TABLE IF NOT EXISTS companies (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    nkey       TEXT NOT NULL UNIQUE,
    logo       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_companies_nkey ON companies(nkey);
DROP TRIGGER IF EXISTS trg_companies_upd ON companies;
CREATE TRIGGER trg_companies_upd BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS assets (
    id              BIGSERIAL PRIMARY KEY,
    type            TEXT NOT NULL CHECK (type IN (
                        'root_domain','ip','subdomain','app','service','endpoint'
                    )),
    company_id      BIGINT REFERENCES companies(id) ON DELETE SET NULL,
    task_ids        BIGINT[] NOT NULL DEFAULT '{}',
    domain          TEXT,
    root_domain     TEXT,
    ip              TEXT,
    c_segment       CIDR,
    port            INTEGER CHECK (port BETWEEN 1 AND 65535),
    icp             TEXT,
    bound_domains   TEXT[]  NOT NULL DEFAULT '{}',
    open_ports      JSONB[] NOT NULL DEFAULT '{}',
    record_type     TEXT,
    record_value    TEXT[],
    bundle_id       TEXT,
    app_name        TEXT,
    category        TEXT,
    app_description TEXT,
    app_icp         TEXT,
    url             TEXT,
    service_type    TEXT CHECK (service_type IN ('http','other')),
    service_name    TEXT,
    favicon_mmh3    TEXT,
    status_code     INTEGER,
    content_length  BIGINT,
    page_title      TEXT,
    technologies    TEXT[]  NOT NULL DEFAULT '{}',
    auth            JSONB[] NOT NULL DEFAULT '{}',
    method          TEXT,
    params          JSONB[] NOT NULL DEFAULT '{}',
    extra           JSONB   NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_root_domain  ON assets(domain) WHERE type = 'root_domain';
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_ip           ON assets(ip)     WHERE type = 'ip';
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_subdomain    ON assets(domain, COALESCE(record_type,'')) WHERE type = 'subdomain';
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_app_bundle   ON assets(bundle_id) WHERE type = 'app' AND bundle_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_app_name     ON assets(app_name)  WHERE type = 'app' AND bundle_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_service_http ON assets(url) WHERE type = 'service' AND service_type = 'http';
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_service_other
    ON assets(COALESCE(domain,''), COALESCE(ip,''), port, service_name) WHERE type = 'service' AND service_type = 'other';
CREATE UNIQUE INDEX IF NOT EXISTS uq_av2_endpoint     ON assets(url, method) WHERE type = 'endpoint';
CREATE INDEX IF NOT EXISTS idx_av2_company      ON assets(company_id)       WHERE company_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_company_type ON assets(company_id, type) WHERE company_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_task_ids     ON assets USING GIN(task_ids);
CREATE INDEX IF NOT EXISTS idx_av2_domain       ON assets(domain)      WHERE domain IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_root_domain  ON assets(root_domain) WHERE root_domain IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_ip           ON assets(ip)          WHERE ip IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_c_segment    ON assets USING GIST(c_segment inet_ops) WHERE c_segment IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_av2_technologies ON assets USING GIN(technologies) WHERE type = 'service';
CREATE INDEX IF NOT EXISTS idx_av2_bound_domains ON assets USING GIN(bound_domains) WHERE type = 'ip';
CREATE INDEX IF NOT EXISTS idx_av2_open_ports   ON assets USING GIN(open_ports)    WHERE type = 'ip';
CREATE INDEX IF NOT EXISTS idx_av2_last_seen    ON assets(last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_av2_type_seen    ON assets(type, last_seen DESC);
DROP TRIGGER IF EXISTS trg_av2_upd ON assets;
CREATE TRIGGER trg_av2_upd BEFORE UPDATE ON assets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS company_scope (
    id         BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('domain','ip','cidr')),
    domain     TEXT,
    net        CIDR,
    raw        TEXT NOT NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_sv2_domain UNIQUE (company_id, domain),
    CONSTRAINT uq_sv2_net    UNIQUE (company_id, net)
);
CREATE INDEX IF NOT EXISTS idx_sv2_domain  ON company_scope(domain)   WHERE kind = 'domain';
CREATE INDEX IF NOT EXISTS idx_sv2_net     ON company_scope USING GIST(net inet_ops) WHERE kind IN ('ip','cidr');
CREATE INDEX IF NOT EXISTS idx_sv2_company ON company_scope(company_id);

-- =====================================================================
-- B. Educate Explorer
-- =====================================================================
CREATE TABLE IF NOT EXISTS explorations (
    id          BIGSERIAL PRIMARY KEY,
    description TEXT,
    goal        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open','achieved','failed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
DROP TRIGGER IF EXISTS trg_exp_upd ON explorations;
CREATE TRIGGER trg_exp_upd BEFORE UPDATE ON explorations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS exploration_nodes (
    id             BIGSERIAL PRIMARY KEY,
    exploration_id BIGINT NOT NULL REFERENCES explorations(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,
    payload        JSONB NOT NULL DEFAULT '{}',
    priority       INT  NOT NULL DEFAULT 0,
    state          TEXT NOT NULL DEFAULT 'open',
    origin         TEXT,
    owner          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ,
    CONSTRAINT ck_node_kind CHECK (kind IN ('begin','goal','intent','fact','finding','hint')),
    CONSTRAINT ck_node_state CHECK (
        (kind='begin'   AND state IN ('open')) OR
        (kind='intent'  AND state IN ('open','running','done','blocked','exhausted','stopped')) OR
        (kind='goal'    AND state IN ('open','met','abandoned')) OR
        (kind='fact'    AND state IN ('confirmed','dismissed','origin')) OR
        (kind='finding' AND state IN ('confirmed','dismissed')) OR
        (kind='hint'    AND state IN ('active','consumed'))
    )
);
CREATE INDEX IF NOT EXISTS idx_expnodes_part     ON exploration_nodes(exploration_id, kind);
CREATE INDEX IF NOT EXISTS idx_expnodes_frontier ON exploration_nodes(exploration_id, priority DESC)
    WHERE kind='intent' AND state='open';
DROP TRIGGER IF EXISTS trg_expnodes_upd ON exploration_nodes;
CREATE TRIGGER trg_expnodes_upd BEFORE UPDATE ON exploration_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS exploration_edges (
    exploration_id BIGINT NOT NULL REFERENCES explorations(id) ON DELETE CASCADE,
    src_id         BIGINT NOT NULL REFERENCES exploration_nodes(id) ON DELETE CASCADE,
    dst_id         BIGINT NOT NULL REFERENCES exploration_nodes(id) ON DELETE CASCADE,
    rel            TEXT NOT NULL CHECK (rel IN ('spawns','derived_from','yields','proves')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (exploration_id, src_id, rel, dst_id),
    CONSTRAINT ck_edge_noself CHECK (src_id <> dst_id)
);
CREATE INDEX IF NOT EXISTS idx_expedges_src ON exploration_edges(src_id, rel);
CREATE INDEX IF NOT EXISTS idx_expedges_dst ON exploration_edges(dst_id, rel);

CREATE TABLE IF NOT EXISTS exploration_anchors (
    node_id   BIGINT NOT NULL REFERENCES exploration_nodes(id) ON DELETE CASCADE,
    asset_id  BIGINT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    PRIMARY KEY (node_id, asset_id)
);
CREATE INDEX IF NOT EXISTS idx_anchor_asset ON exploration_anchors(asset_id);

CREATE TABLE IF NOT EXISTS activity (
    id                 BIGSERIAL PRIMARY KEY,
    exploration_id     BIGINT NOT NULL REFERENCES explorations(id) ON DELETE CASCADE,
    node_id            BIGINT REFERENCES exploration_nodes(id) ON DELETE SET NULL,
    worker             TEXT,
    kind               TEXT,
    tool               TEXT,
    tool_use_id        TEXT,
    is_error           BOOLEAN NOT NULL DEFAULT false,
    summary            TEXT,
    detail             TEXT,
    input_tokens       INTEGER,
    output_tokens      INTEGER,
    cache_read_tokens  INTEGER,
    cache_write_tokens INTEGER,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_act_node  ON activity(exploration_id, node_id, id);
CREATE INDEX IF NOT EXISTS idx_act_since ON activity(exploration_id, id);

-- =====================================================================
-- C. LLM profiles
-- =====================================================================
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS llm_profiles (
    id               BIGSERIAL PRIMARY KEY,
    name             TEXT NOT NULL UNIQUE,
    format           TEXT NOT NULL CHECK (format IN ('openai','anthropic')),
    base_url         TEXT,
    proxy            TEXT,
    model            TEXT NOT NULL,
    api_key          TEXT,
    api_key_hint     TEXT,
    rate_per_second  DOUBLE PRECISION NOT NULL DEFAULT 0,
    rate_per_minute  DOUBLE PRECISION NOT NULL DEFAULT 0,
    context_window_k INTEGER NOT NULL DEFAULT 0,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    is_default       BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_llm_one_default ON llm_profiles(is_default) WHERE is_default;
DROP TRIGGER IF EXISTS trg_llm_upd ON llm_profiles;
CREATE TRIGGER trg_llm_upd BEFORE UPDATE ON llm_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- LLM Rotation(Fragmentation):priority=Serial weights(DESC,Activate Configuration Always First);pool_exclude=Not as a bottom target.
ALTER TABLE llm_profiles ADD COLUMN IF NOT EXISTS priority     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE llm_profiles ADD COLUMN IF NOT EXISTS pool_exclude BOOLEAN NOT NULL DEFAULT false;
-- Custom session header: every time you are not empty LLM Please take that. HTTP Head(Head Value=Current Session session id),For press session
-- Do hint caches/Sticky path path gateway(Like opencode zen). Empty=Don't send.
ALTER TABLE llm_profiles ADD COLUMN IF NOT EXISTS session_header_key TEXT NOT NULL DEFAULT '';
-- Meltinger state: The process is the same, and the reservoir is used to restore the unexpired cooling window only after restart.
CREATE TABLE IF NOT EXISTS llm_profile_health (
    profile_id BIGINT PRIMARY KEY REFERENCES llm_profiles(id) ON DELETE CASCADE,
    fails      INTEGER NOT NULL DEFAULT 0,
    trips      INTEGER NOT NULL DEFAULT 0,
    open_until TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    last_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =====================================================================
-- D. Task Layer
-- =====================================================================
CREATE TABLE IF NOT EXISTS tasks (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT NOT NULL DEFAULT '',   -- Short Task Name/Objective(Like a network of officials. / Innernet testing);Empty=Back #id·Description
    description    TEXT NOT NULL,
    goal           TEXT NOT NULL,
    exploration_id BIGINT NOT NULL UNIQUE
                     REFERENCES explorations(id) ON DELETE RESTRICT,
    status         TEXT NOT NULL DEFAULT 'created'
                     CHECK (status IN ('created','running','paused','done','failed','timeout')),
    paused         BOOLEAN NOT NULL DEFAULT false,
    llm_profile_id BIGINT REFERENCES llm_profiles(id) ON DELETE SET NULL,
    company_id     BIGINT REFERENCES companies(id) ON DELETE SET NULL,
    parent_ref     TEXT,
    timeout_seconds INTEGER NOT NULL DEFAULT 0,
    first_run_at   TIMESTAMPTZ,
    deadline_at    TIMESTAMPTZ,
    deleted_at     TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';  -- Repository Supplement
-- Task export agent selection:''=Follow Global(I'll go with the agency.) | 'direct'=Force Direct Company | 'proxy'=Go global agent.
-- Task Run/Set on Activation MITM Upstream(Hide Source IP).
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS egress TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_tasks_alive  ON tasks(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)          WHERE deleted_at IS NULL;
DROP TRIGGER IF EXISTS trg_tasks_upd ON tasks;
CREATE TRIGGER trg_tasks_upd BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- E. Agents / Plugin Template / Variable Directory
-- =====================================================================
CREATE TABLE IF NOT EXISTS agents (
    id                BIGSERIAL PRIMARY KEY,
    key               TEXT NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z0-9_]*$'),
    name              TEXT NOT NULL,
    description       TEXT,
    role              TEXT NOT NULL,
    builtin           BOOLEAN NOT NULL DEFAULT true,
    enabled           BOOLEAN NOT NULL DEFAULT true,
    llm_profile_id    BIGINT REFERENCES llm_profiles(id) ON DELETE SET NULL,
    current_prompt_id BIGINT,
    max_turns         INTEGER NOT NULL DEFAULT 0,
    run_seconds       INTEGER NOT NULL DEFAULT 600,
    web_search        BOOLEAN NOT NULL DEFAULT false,
    interactive_shell BOOLEAN NOT NULL DEFAULT false,
    wrapup_prompt     TEXT NOT NULL DEFAULT '',
    wrapup_max_turns  INTEGER NOT NULL DEFAULT 0,
    task_timeout_wrapup_prompt    TEXT NOT NULL DEFAULT '',
    task_timeout_wrapup_max_turns INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agents_role_ck CHECK (role IN ('goals','main','planner','worker','assistant'))
);
DROP TRIGGER IF EXISTS trg_agents_upd ON agents;
CREATE TRIGGER trg_agents_upd BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS agent_prompts (
    id            BIGSERIAL PRIMARY KEY,
    agent_id      BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    version       INT NOT NULL,
    template_text TEXT NOT NULL,
    note          TEXT,
    updated_by    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent_id, version)
);
-- Loop external keys:agents.current_prompt_id → agent_prompts.id(After two tables are created)
DO $$ BEGIN
    ALTER TABLE agents ADD CONSTRAINT fk_agents_curprompt
        FOREIGN KEY (current_prompt_id) REFERENCES agent_prompts(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS agent_prompt_vars (
    id          BIGSERIAL PRIMARY KEY,
    agent_id    BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    var_name    TEXT NOT NULL,
    description TEXT,
    example     TEXT,
    source      TEXT NOT NULL CHECK (source IN ('exploration','runtime','distilled')),
    UNIQUE (agent_id, var_name)
);

-- =====================================================================
-- F. MCP Services
-- =====================================================================
CREATE TABLE IF NOT EXISTS mcp_servers (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    transport       TEXT NOT NULL CHECK (transport IN ('stdio','http')),
    command         TEXT,
    args            JSONB NOT NULL DEFAULT '[]',
    env             JSONB NOT NULL DEFAULT '{}',
    headers         JSONB NOT NULL DEFAULT '{}',  -- http transport request headers (separate from env)
    url             TEXT,
    description     TEXT NOT NULL DEFAULT '',
    timeout_seconds INT NOT NULL DEFAULT 30,      -- connect + tools/list timeout (0 = platform default ~90s)
    enabled         BOOLEAN NOT NULL DEFAULT true,
    status          TEXT NOT NULL DEFAULT '',     -- '' | 'ok' | 'error' (last health-check result)
    status_at       TIMESTAMPTZ,                  -- when the last health check ran
    status_error    TEXT NOT NULL DEFAULT '',      -- last error message (empty when ok)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
DROP TRIGGER IF EXISTS trg_mcp_upd ON mcp_servers;
CREATE TRIGGER trg_mcp_upd BEFORE UPDATE ON mcp_servers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Default data source location:ScopeSentry Asset Sync MCP(The address and authentication are empty and not enabled.
-- To check if data sources are configured for the Asset Sync page; users fill in the page url and X-API-Key .
-- Insert only when missing, never overwrite the user configured/Enabled server(s)schema.sql Every time it starts. Exec).
INSERT INTO mcp_servers (name, transport, url, env, enabled)
VALUES ('ScopeSentry', 'http', NULL, '{"X-API-Key":""}', false)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS mcp_tools_cache (
    id            BIGSERIAL PRIMARY KEY,
    server_id     BIGINT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_name     TEXT NOT NULL,
    description   TEXT,
    schema        JSONB,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (server_id, tool_name)
);

-- MCP Tool call statistics (observability, analogy) skill_stats). Every time MCP tool By agent Enforcement +1,
-- Press (server_name, tool_name, agent_key) Aggregation. For MCP Manages the page monitoring panel.
CREATE TABLE IF NOT EXISTS mcp_stats (
    server_name TEXT NOT NULL,
    tool_name   TEXT NOT NULL,
    agent_key   TEXT NOT NULL DEFAULT '',
    calls       BIGINT NOT NULL DEFAULT 0,
    last_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (server_name, tool_name, agent_key)
);
CREATE INDEX IF NOT EXISTS idx_mcp_stats_server ON mcp_stats(server_name);

-- Migration: add new columns to existing mcp_servers tables.
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '{}';
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS timeout_seconds INT NOT NULL DEFAULT 30;
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS status_at TIMESTAMPTZ;
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS status_error TEXT NOT NULL DEFAULT '';

-- =====================================================================
-- G. Visibility:agent × mcp / skill
-- =====================================================================
CREATE TABLE IF NOT EXISTS agent_visibility (
    agent_id      BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    resource_kind TEXT   NOT NULL CHECK (resource_kind IN ('mcp')),
    resource_id   BIGINT NOT NULL,
    mcp_tool_name TEXT   NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, resource_kind, resource_id, mcp_tool_name)
);
CREATE INDEX IF NOT EXISTS idx_vis_resource ON agent_visibility(resource_kind, resource_id);

CREATE TABLE IF NOT EXISTS agent_skill_visibility (
    agent_id   BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_name TEXT   NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, skill_name)
);
CREATE INDEX IF NOT EXISTS idx_askv_skill ON agent_skill_visibility(skill_name);

-- Skill Call statistics (renewal detectability): every time skill By agent Loading is +1,Press (skill,agent) Aggregation.
-- For 'Skills"Page Watch Panel——Which ones? skill It's re-used, frequency, by whom, last time.
CREATE TABLE IF NOT EXISTS skill_stats (
    skill_name TEXT NOT NULL,
    agent_key  TEXT NOT NULL DEFAULT '',
    calls      BIGINT NOT NULL DEFAULT 0,
    last_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (skill_name, agent_key)
);
CREATE INDEX IF NOT EXISTS idx_skill_stats_name ON skill_stats(skill_name);

-- =====================================================================
-- H. Internal Tool Directory
-- =====================================================================
CREATE TABLE IF NOT EXISTS tools (
    key         TEXT PRIMARY KEY,
    system      BOOLEAN NOT NULL DEFAULT true,
    description TEXT    NOT NULL DEFAULT '',
    schema      JSONB   NOT NULL DEFAULT '{}',
    agents      JSONB   NOT NULL DEFAULT '[]',
    enabled     BOOLEAN NOT NULL DEFAULT true,
    kind        TEXT    NOT NULL DEFAULT 'builtin',
    exec        JSONB   NOT NULL DEFAULT '{}',
    deferred    BOOLEAN NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
DROP TRIGGER IF EXISTS trg_tools_upd ON tools;
CREATE TRIGGER trg_tools_upd BEFORE UPDATE ON tools
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- I. Session (dialogue page)
-- =====================================================================
CREATE TABLE IF NOT EXISTS conversations (
    id             BIGSERIAL PRIMARY KEY,
    agent_key      TEXT NOT NULL,
    title          TEXT NOT NULL DEFAULT '',
    llm_profile_id BIGINT REFERENCES llm_profiles(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
DROP TRIGGER IF EXISTS trg_conversations_upd ON conversations;
CREATE TRIGGER trg_conversations_upd BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- A conversation can be attached to a task: its agent then records findings,
-- facts and assets into that task's graph (with traffic linking and
-- verification), instead of having no task context at all. NULL = not attached.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS task_id BIGINT REFERENCES tasks(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS conversation_activities (
    id                 BIGSERIAL PRIMARY KEY,
    conversation_id    BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    worker             TEXT,
    kind               TEXT,
    tool               TEXT,
    tool_use_id        TEXT,
    is_error           BOOLEAN NOT NULL DEFAULT false,
    summary            TEXT,
    detail             TEXT,
    input_tokens       INTEGER,
    output_tokens      INTEGER,
    cache_read_tokens  INTEGER,
    cache_write_tokens INTEGER,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_conv_act ON conversation_activities(conversation_id, id);

-- =====================================================================
-- J. Agent Trigger
-- =====================================================================
CREATE TABLE IF NOT EXISTS agent_triggers (
    id                          BIGSERIAL PRIMARY KEY,
    agent_key                   TEXT NOT NULL,
    enabled                     BOOLEAN NOT NULL DEFAULT true,
    interval_sec                INTEGER NOT NULL DEFAULT 0,
    on_finding                  BOOLEAN NOT NULL DEFAULT false,
    on_goal_met                 BOOLEAN NOT NULL DEFAULT false,
    on_task_timeout             BOOLEAN NOT NULL DEFAULT false,
    interval_message            TEXT NOT NULL DEFAULT '',
    finding_message             TEXT NOT NULL DEFAULT '',
    goal_message                TEXT NOT NULL DEFAULT '',
    task_timeout_message        TEXT NOT NULL DEFAULT '',
    last_fire                   TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_agent_triggers_agent ON agent_triggers(agent_key);
DROP TRIGGER IF EXISTS trg_agent_triggers_upd ON agent_triggers;
CREATE TRIGGER trg_agent_triggers_upd BEFORE UPDATE ON agent_triggers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS scheduler_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

-- =====================================================================
-- K. Interception rules
-- =====================================================================
CREATE TABLE IF NOT EXISTS intercept_rules (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    priority        INTEGER NOT NULL DEFAULT 0,
    match_target    TEXT NOT NULL CHECK (match_target IN ('tool_name', 'tool_input')),
    match_type      TEXT NOT NULL CHECK (match_type IN ('string', 'regex')),
    pattern         TEXT NOT NULL,
    action          TEXT NOT NULL CHECK (action IN ('allow', 'deny', 'ask')),
    message         TEXT NOT NULL DEFAULT '',
    timeout_enabled BOOLEAN NOT NULL DEFAULT true,
    timeout_seconds INTEGER NOT NULL DEFAULT 60,
    timeout_action  TEXT    NOT NULL DEFAULT 'deny',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DROP TRIGGER IF EXISTS trg_intercept_rules_upd ON intercept_rules;
CREATE TRIGGER trg_intercept_rules_upd BEFORE UPDATE ON intercept_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS intercept_pending (
    id              BIGSERIAL PRIMARY KEY,
    rule_id         BIGINT REFERENCES intercept_rules(id) ON DELETE SET NULL,
    conversation_id BIGINT REFERENCES conversations(id) ON DELETE CASCADE,
    task_id         TEXT,
    agent_name      TEXT NOT NULL DEFAULT '',
    tool_name       TEXT NOT NULL,
    tool_input      JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'allowed', 'denied', 'timeout')),
    decided_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_intercept_pending_status ON intercept_pending(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_intercept_pending_task   ON intercept_pending(task_id, created_at DESC);

-- =====================================================================
-- L. Persistence of loopholes
-- =====================================================================
CREATE TABLE IF NOT EXISTS findings (
    id          BIGSERIAL PRIMARY KEY,
    task_id     BIGINT REFERENCES tasks(id) ON DELETE SET NULL,
    node_id     BIGINT REFERENCES exploration_nodes(id) ON DELETE SET NULL,
    vulnclass   TEXT NOT NULL DEFAULT '',
    severity    TEXT NOT NULL DEFAULT '',
    summary     TEXT NOT NULL DEFAULT '',
    evidence    TEXT NOT NULL DEFAULT '',
    worker      TEXT NOT NULL DEFAULT '',
    asset_ids   JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_findings_task ON findings(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_findings_time ON findings(created_at DESC);
-- Structural reproduction of local records(entry_point/prerequisites/steps[{request,expected,proves}]/provenance):
-- agent While confirming the bug report_finding Fill,Request taken word for word from true execution→After that, only words are written and no more large-scale reconstructions are generated.
ALTER TABLE findings ADD COLUMN IF NOT EXISTS reproduction JSONB;
-- Machine-checked verification (adapted from ARTEX S1). Kept apart from any human
-- disposition. poe_level is the agent-declared proof-of-exploitation level (0-4);
-- evidence_quotes are exact lines it says appear in linked response bodies.
-- verification_status: verified | unverified | not_assessed; NULL reads as
-- unverified, so findings recorded before this column existed stay unverified.
ALTER TABLE findings ADD COLUMN IF NOT EXISTS poe_level INT NOT NULL DEFAULT 0;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS evidence_quotes JSONB NOT NULL DEFAULT '[]';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS verification_status TEXT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS verification_note TEXT NOT NULL DEFAULT '';

-- =====================================================================
-- M. Backend Last
-- =====================================================================
CREATE TABLE IF NOT EXISTS server_logs (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    level      TEXT NOT NULL DEFAULT 'info',
    tag        TEXT NOT NULL DEFAULT '',
    text       TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_server_logs_id ON server_logs(id DESC);

-- =====================================================================
-- N. Multiple Users RBAC(AntiFox Platform rights: users / Role / Permissions / (Authorized)
-- =====================================================================
CREATE TABLE IF NOT EXISTS rbac_users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    is_builtin    BOOLEAN NOT NULL DEFAULT false,
    token_version INT NOT NULL DEFAULT 0,  -- Reclassified/Discontinuation;JWT Carrying old versions is invalid (forced re-entry)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE rbac_users ADD COLUMN IF NOT EXISTS token_version INT NOT NULL DEFAULT 0;  -- Stock stock supplement (single etc.)

CREATE TABLE IF NOT EXISTS rbac_roles (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    scope       TEXT NOT NULL DEFAULT 'assigned',   -- all | assigned | own
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rbac_permissions (
    key         TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS rbac_role_permissions (
    role_id        TEXT NOT NULL REFERENCES rbac_roles(id) ON DELETE CASCADE,
    permission_key TEXT NOT NULL,
    PRIMARY KEY (role_id, permission_key)
);

CREATE TABLE IF NOT EXISTS rbac_user_roles (
    user_id TEXT NOT NULL REFERENCES rbac_users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES rbac_roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);
CREATE INDEX IF NOT EXISTS idx_rbac_user_roles_user ON rbac_user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_rbac_role_perms_role ON rbac_role_permissions(role_id);

-- =====================================================================
-- O. Audit log (platform operation)/Log-in audit; subsequent expansion to full audit centre)
-- =====================================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor      TEXT NOT NULL DEFAULT '',   -- Operator username
    category   TEXT NOT NULL DEFAULT '',   -- auth / rbac / task / ...
    action     TEXT NOT NULL DEFAULT '',   -- login / login_failed / create_user / ...
    result     TEXT NOT NULL DEFAULT 'success', -- success / fail
    ip         TEXT NOT NULL DEFAULT '',
    message    TEXT NOT NULL DEFAULT '',
    detail     JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_id ON audit_logs(id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor);
CREATE INDEX IF NOT EXISTS idx_audit_logs_cat ON audit_logs(category, action);

-- §P Finding completion: AI-generated or manually edited structured finding data, covering the payload of the original finding node above (the node itself is not modified).
-- Here. exploration Nodes id is the norm key; the report is shared with the discovery page.source: ai | manual.
CREATE TABLE IF NOT EXISTS finding_enrichment (
    node_id      BIGINT PRIMARY KEY,
    name         TEXT NOT NULL DEFAULT '',   -- Coloured bug name
    target       TEXT NOT NULL DEFAULT '',   -- Affected host / URL
    module       TEXT NOT NULL DEFAULT '',
    severity     TEXT NOT NULL DEFAULT '',   -- Manually covered (empty)=Continue Node)
    status       TEXT NOT NULL DEFAULT 'open', -- open|confirmed|fixed|false_positive|ignored
    cvss_score   REAL NOT NULL DEFAULT 0,
    cvss_vector  TEXT NOT NULL DEFAULT '',
    cvss_metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    description  JSONB NOT NULL DEFAULT '[]'::jsonb, -- string[]
    test_process JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{type,content}]
    risk_analysis JSONB NOT NULL DEFAULT '[]'::jsonb, -- string[]
    remediation  JSONB NOT NULL DEFAULT '[]'::jsonb, -- string[]
    source       TEXT NOT NULL DEFAULT 'ai',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_finding_enrichment_status ON finding_enrichment(status);

-- §P2 Gap evidence cut-off: press exploration Nodes id Association, File Storage dataDir/report_assets/evidence/<node_id>/.
-- Presentation of the Discovery Page + Report(DOCX/MD)(a) Embedded sharing in the recovery steps;step_index Corroborating step number(0=Overview).
CREATE TABLE IF NOT EXISTS finding_evidence (
    id          BIGSERIAL PRIMARY KEY,
    node_id     BIGINT NOT NULL,
    filename    TEXT NOT NULL,                -- report_assets/evidence/<node_id>/ Filename under
    caption     TEXT NOT NULL DEFAULT '',     -- Figure
    step_index  INT NOT NULL DEFAULT 0,       -- Corroborating step number(0=Not specified/Overview)
    sort_order  INT NOT NULL DEFAULT 0,       -- Show Order
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_finding_evidence_node ON finding_evidence(node_id);

-- §P3 Hole↔Flow linkages: recording flows(MITM Something in the index exchange)Close a gap as a source of evidence.
-- The metadata are in the flow library.(SQLite), Link here.(PG);source=manual(Flow Page Manual) | auto(Autorecord when mission engine digs).
CREATE TABLE IF NOT EXISTS finding_traffic (
    node_id     BIGINT NOT NULL,
    exchange_id TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (node_id, exchange_id)
);
CREATE INDEX IF NOT EXISTS idx_finding_traffic_node ON finding_traffic(node_id);

-- §Q Reporting management: organizational reporting settings(Single cases) + List of reports generated.
CREATE TABLE IF NOT EXISTS report_settings (
    id              INT PRIMARY KEY DEFAULT 1,
    company_name    TEXT NOT NULL DEFAULT '',   -- Author of the report(Chinese)
    company_name_en TEXT NOT NULL DEFAULT '',
    logo_path       TEXT NOT NULL DEFAULT '',   -- Relative Path(data/report_assets Down)
    personnel_json  JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{name,role,contact}]
    cvss_version    TEXT NOT NULL DEFAULT '3.1',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT report_settings_singleton CHECK (id = 1)
);
CREATE TABLE IF NOT EXISTS reports (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT,
    task_desc  TEXT NOT NULL DEFAULT '',
    filename   TEXT NOT NULL DEFAULT '',
    path       TEXT NOT NULL DEFAULT '',
    size       BIGINT NOT NULL DEFAULT 0,
    company    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reports_created ON reports(created_at DESC);

-- Named report template: multiple copies saved"Author of the report/logo/Tester/CVSS/Default Language"preset, select a new report.
-- Bones.(Methodology/List/Tools/Normative)By Editable skill The template is available by language and is not duplicated.
CREATE TABLE IF NOT EXISTS report_templates (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL DEFAULT '',
    company_name    TEXT NOT NULL DEFAULT '',
    company_name_en TEXT NOT NULL DEFAULT '',
    logo_path       TEXT NOT NULL DEFAULT '',
    personnel_json  JSONB NOT NULL DEFAULT '[]'::jsonb,
    cvss_version    TEXT NOT NULL DEFAULT '3.1',
    locale          TEXT NOT NULL DEFAULT 'zh-CN',  -- Default Report Language
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_report_templates_created ON report_templates(created_at DESC);

-- =====================================================================
-- R. Knowledge base (two floors) RAG:Global library + Task Library)
-- =====================================================================
-- Document level: scope=global (platform-wide findings, zero-days, skills, methodology) | task (a task's mandate, rating rules, scope, checkpoints).
-- Vector segment layer knowledge_chunks (created by the Go guard using a pgvector vector
-- (ensureKnowledgeVectorSchema)——CREATE EXTENSION vector Superuser preset required; when missing
-- Degraded to keyword search and start without blocking, so this must be successful. schema Lee.
CREATE TABLE IF NOT EXISTS knowledge_docs (
    id         BIGSERIAL PRIMARY KEY,
    scope      TEXT   NOT NULL DEFAULT 'global',           -- global | task
    task_id    BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
    category   TEXT   NOT NULL DEFAULT '',
    title      TEXT   NOT NULL DEFAULT '',
    content    TEXT   NOT NULL DEFAULT '',
    source     TEXT   NOT NULL DEFAULT 'manual',           -- manual | upload | migrated
    file_name  TEXT   NOT NULL DEFAULT '',
    bytes      BIGINT NOT NULL DEFAULT 0,
    indexed    BOOLEAN NOT NULL DEFAULT false,             -- Whether vector segments are generated
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_docs_scope ON knowledge_docs(scope);
CREATE INDEX IF NOT EXISTS idx_knowledge_docs_task ON knowledge_docs(task_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_docs_cat ON knowledge_docs(category);
DROP TRIGGER IF EXISTS trg_knowledge_docs_updated ON knowledge_docs;
CREATE TRIGGER trg_knowledge_docs_updated BEFORE UPDATE ON knowledge_docs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Search log (observability) / audit)
CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT,
    agent_key  TEXT   NOT NULL DEFAULT '',
    query      TEXT   NOT NULL DEFAULT '',
    scope      TEXT   NOT NULL DEFAULT '',
    mode       TEXT   NOT NULL DEFAULT '',                 -- semantic | keyword
    top_k      INT    NOT NULL DEFAULT 0,
    hits       INT    NOT NULL DEFAULT 0,
    doc_ids    BIGINT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_logs_created ON knowledge_retrieval_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_logs_task ON knowledge_retrieval_logs(task_id);

-- embedder Configure (single case):OpenAI Compatibility embeddings Endpoint.api_key No return. UI.
CREATE TABLE IF NOT EXISTS knowledge_config (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled    BOOLEAN NOT NULL DEFAULT false,             -- semantic search switches; off=Keywords only
    base_url   TEXT NOT NULL DEFAULT 'https://open.bigmodel.cn/api/paas/v4',
    model      TEXT NOT NULL DEFAULT 'embedding-3',
    api_key    TEXT NOT NULL DEFAULT '',
    dim        INT  NOT NULL DEFAULT 1024,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO knowledge_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
