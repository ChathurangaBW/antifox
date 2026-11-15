# Engine state: diagnosis and continuity-preserving correction

Reviewed 2026-11-15 (local date). Source commit:
`4021ceda702d337f5428cce8734a0ae58cef2aef`. Live health reports `v2.1.6`.
`git diff v2.1.6..HEAD -- server web agent db` is empty: the inspected application
source matches that release; the newer commit changes documentation. Those
revision comparisons describe committed source before the local patch below.

## Local implementation and verification

### Turbopack artifact browser verification (latest; isolated)

The generated `web/out` tree and current UI source were checked against the
`antifox-default-build-1y3hwpid` manifests. The isolated embedded UI tree was
replaced with that exact artifact and checked for equal file sets and hashes,
both before and after testing, preventing stale Webpack files from mixing in.

The actual login/task-list browser test passed against this Turbopack artifact
with Go race detection: incorrect password 401, valid password 200, visible task,
authenticated reload, zero JavaScript errors and zero external browser requests.
All 12 PostgreSQL-backed API checks also passed (healthy reads, telemetry/RBAC
failures, rejected reruns, and missing-token rejection). Engine remained inert;
task clocks/lifecycle were preserved. Login audit writes were confined to the
disposable database. `go vet ./server` passed and the container was removed.

Evidence, screenshot, source/artifact manifests, exact embedded-artifact identity,
test/build logs and cleanup record:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-pg-browser-fulm5jlg`.

This closes the latest-artifact browser gap noted below. Changes still remain
uncommitted and undeployed; no final deployed artifact has been verified.

### Default Turbopack build corrected (latest; local only)

Merriweather requested discrete weights 400 and 700. The fetched modern-browser
Google Fonts CSS repeats the same variable-font URLs for both weights; the
previous build failed in Turbopack's font URL processing with `next/font/google
queries have exactly one entry`. The registry now requests Merriweather's default
variable font instead of duplicate discrete weights, preserving its CSS variable
and availability of normal/bold weights.

After this targeted change, the unchanged default command `npm run build:static`
passed using Next.js 16.2.9 Turbopack: compilation, TypeScript, and 29 generated
pages. No Webpack override was used. All tracked UI source files matched the
isolated build snapshot. Evidence, baseline failure log, source SHA-256 manifest,
and generated-artifact SHA-256 manifest:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-default-build-1y3hwpid`.

This supersedes the unresolved default-build issue below. The preceding login
browser run used the earlier Webpack artifact; the new Turbopack artifact has
build verification only. All fixes remain local/uncommitted and undeployed;
final deployed artifact verification remains outstanding.

### Actual UI/password login verification (latest; isolated, not deployed)

Fresh static UI source was built and embedded using the real `webuiHandler`.
Chromium exercised the actual `/login/` form against `authLogin`, bcrypt, and a
disposable PostgreSQL RBAC account. Observed results:

```text
loginStatuses: [401, 200]
taskVisible: true
reloadAuthenticated: true
javascriptErrors: []
externalRequests: []
PASS TestDisposablePostgresBrowser (race detection enabled)
```

The invalid password stored no token; valid login stored a token, redirected to
the task list, and displayed the fixture task after reload. All 12 subsequent
browser API checks also passed. Task clocks/lifecycle remained unchanged and no
engine loops ran. Login used a separate fixture DB connection with write access
for normal login audit records; protected observation/control handlers used the
SELECT-only runtime role. Thus this run does not claim zero database writes.

`npm run build:static -- --webpack` passed compilation, TypeScript, and static
generation of 29 pages. Default Turbopack failed on Google-font processing;
that build-path failure remains unresolved. An initial dependency symlink was
also rejected by Turbopack and replaced with a copied dependency directory in
the isolated workspace. No production source was altered for these build steps.

Evidence directory:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-pg-browser-8xywi5z6`
contains the successful test log, build logs, authenticated task-list screenshot,
eleven-file backend/test identity, all tracked UI-source hashes, UI artifact
hashes, and disposable-container cleanup evidence. All tracked UI source matched
the build snapshot. `go vet ./server` passed.

This covers actual login and task-list UI rendering, not every application page
or the live deployment. Auxiliary endpoints outside the selected fixture routes
are not integrated. Changes remain uncommitted and undeployed; final deployed
artifact verification and the default build failure remain outstanding.

### Disposable PostgreSQL and authenticated browser (latest; local only)

`server/postgres_browser_observation_test.go` now provides an explicitly opted-in
integration check. It applies the real database schema to a fresh PostgreSQL 16
container using `pgvector/pgvector:pg16`, seeds a fixture RBAC account/role, then
serves actual protected handlers with an inert engine. The runtime PostgreSQL
role has SELECT privileges only; the test checks that it cannot UPDATE tasks.
No application constructors, planner loops, or worker loops run.

A Chromium page makes real same-origin HTTP requests using a fixture-signed JWT.
The three phases passed under Go race detection:

| Phase | Stats | Task list | Nonterminal rerun | No token |
|---|---|---|---|---|
| Healthy database | 200 | 200 | 409 | 401 |
| Asset SELECT revoked | 503 | 200 | 409 | 401 |
| RBAC-user SELECT revoked | 503 | 503 | 503 | 401 |

All 12 browser requests matched expectations; task lifecycle and clocks were
preserved. `go vet ./server` passed. Eleven source/test files match the tested
snapshot. Container data used tmpfs and the container was removed in cleanup.
Logs, cleanup evidence, and SHA-256 manifest:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-pg-browser-8_r79d4o`.
An initial run could not launch Playwright's expected browser revision; the
successful run explicitly selected the installed Chromium executable.

Scope: this validates real PostgreSQL query execution and RBAC with browser HTTP
requests, not application-page rendering or the password-login flow. The task
object is an in-memory fixture; persisted task data migration/resumption is not
covered. The database begins with empty telemetry tables. Proxy localization's
real-entry check is covered separately by the prior combined suite. Changes
remain uncommitted and undeployed, so this is not a verified final release.

### Authorization fail-closed correction (latest; local only)

Protected requests now reject empty JWT subjects (401), unavailable manager/DB
dependencies (503), and failed or nil RBAC access resolution (503), before the
protected handler executes. Responses contain generic errors. Public-route
exceptions retain their existing behavior.

Isolated regressions cover user/role/permission lookup failures, absent user rows,
missing manager/database, empty subjects, and a successful-access control.
Rejected requests never reach the test handler. The fixture asserts zero SQL
writes; no task execution is started. A nil access result is guarded explicitly;
the existing concrete resolver's missing-user behavior is a no-rows error.

Combined suite: **61 tests/subtests passed normally and with race detection;
zero skips**. `go vet ./traffic ./server` passed, and ten source/test files match
the tested snapshot. Evidence and SHA-256 manifest:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-auth-closed-ovoes784`.

This supersedes the unresolved lookup-fallthrough concern in the earlier record.
Changes remain uncommitted and undeployed; authenticated browser and PostgreSQL
integration verification remain outstanding. No final-deployment claim is made.

### Combined authenticated verification (latest; local only)

The local control handler now rejects nonterminal reruns with HTTP 409 before
reactivation. Five state cases preserve lifecycle, pause state and task clocks.
The combined suite includes real proxy-entry localization, telemetry regressions,
rerun rejection, and JWT authentication with successful fixture-backed RBAC lookup.
Authenticated stats/task reads return 200, failed telemetry returns 503, rejected
reruns return 409, and missing/invalid credentials return 401. No engine loops
are started; the SQL fixture rejects writes and asserts zero writes.

Combined verification: **52 tests/subtests passed normally and with race detection,
zero skips**. `go vet ./traffic ./server` passed. Eight source/test files match
the tested snapshot; hashes and logs are recorded at
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-combined-uu7smwii`.

This is middleware/handler verification, not browser/login or PostgreSQL integration.
The auth middleware currently falls through when RBAC resolution fails; the test
explicitly verifies successful RBAC resolution so that behavior cannot substitute
for authentication coverage. That separate fail-open concern remains unresolved.
All follow-up changes remain local, uncommitted and undeployed; the live image's
previously verified identity is still `v2.1.6+statusfix`. A fully patched final
deployment has not been established. Older outstanding-work lists below describe
their respective historical verification points.

### Latest follow-up: real proxy entry and read-only telemetry (not deployed)

The proxy integration regression now starts the actual `go-mitmproxy` entry
handler on loopback, registers the application's `sink` addon, and sends a
direct request. It fails against the original dependency fallback and passes
with the localization override:

```text
real proxy entry: HTTP 400; body="This is a proxy server; direct requests are not supported.\n"
```

No forwarding destination, CONNECT request, application server, or task engine
is involved. The test creates its CA only inside a disposable test directory
and closes its listener during cleanup.

Additional local changes:

- `server/stats_observation.go` projects lifecycle, pause, running intent count,
  last activity, and the existing combined planner/worker execution counter.
  The counter is read atomically without creating an entry on a miss. It does
  not change execution, scheduling, retries, or task controls.
- Terminal tasks return `engine_mode: idle`, their terminal lifecycle in
  `active_task.status`, and false `running`/`stalled` flags. The existing mode
  enum remains compatible. Pause takes precedence over inactivity for other
  tasks.
- A nonzero execution counter prevents the zero-worker-intents heuristic from
  marking an executing planner as stalled. `active_task.in_flight` retains its
  running-intent meaning; the additive `executions_in_flight` field represents
  planner and worker calls together. It does not prove progress or distinguish
  planner calls from worker calls. TypeScript describes both additive fields
  as optional for compatibility with the deployed older server.
- `/api/stats` returns generic HTTP 503 errors on traffic, asset, exploration,
  intent, or goal telemetry failure. `/api/tasks` does the same for token,
  activity, or goal aggregate failures. Neither handler emits a partial success
  with failure-derived empty counters. Existing UI failure handling can therefore
  mark observations stale/unavailable.

`server/stats_observation_test.go` exercises the real HTTP handlers and database
decoding with a read-only SQL fixture driver. Query errors, scan errors, and
row-iteration errors are tested for seven reads (21 failure cases), with a
separate closed disposable SQLite fixture for traffic-count failure. Other cases
cover planner/worker observations, pause, all terminal statuses, and inactivity
boundaries. Repeated status reads must preserve task clocks/status and must not
create execution counters or issue SQL writes. Planner/worker objects are inert;
no engine loop or LLM call is started. PostgreSQL dialect/server integration is
not established by the SQL fixture tests.

Verification from an isolated snapshot based on `f55dc85`:

```text
PASS: real proxy entry regression detects the original HTTP 400 non-English fallback
PASS: HTTP telemetry regressions fail against the unpatched handlers
PASS: go vet ./traffic ./server
PASS: proxy-normal:     2 top-level, 6 including subtests, 0 skipped
PASS: proxy-race:       2 top-level, 6 including subtests, 0 skipped
PASS: telemetry-normal: 4 top-level, 32 including subtests, 0 skipped
PASS: telemetry-race:   4 top-level, 32 including subtests, 0 skipped
```

Evidence:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-observations-fqzrv77p`
contains baseline failure logs, normal/race JSON test logs, vet/format logs,
source hashes in `identity.json`, and `summary.json`.

**Current assessment:** proxy localization and the targeted read-only telemetry
corrections are locally implemented and regression-tested, but uncommitted and
undeployed. Server-side rerun restrictions and authenticated live-browser
verification remain outstanding. These checks do not establish a fully patched
final deployment. Prior deployment evidence below describes the UI patch only.

### Follow-up: proxy response localization (local, not deployed)

The live direct-request response on port 8788 was confirmed as HTTP 400 with
Chinese text (`此为代理服务器，不能直接发起请求`). This comes from
`github.com/lqqyt2423/go-mitmproxy v1.9.2`, `proxy/entry.go:200-212`, rather than
the frontend translations. The dependency calls addon `AccessProxyServer` hooks
for direct requests before emitting its fallback when no addon has responded.
CONNECT handling precedes that branch; absolute-URL forwarding follows it.

The local patch overrides `sink.AccessProxyServer` in `traffic/traffic.go`, using
the existing registered addon. It returns HTTP 400 with plain UTF-8 English:

```text
This is a proxy server; direct requests are not supported.
```

No module-cache edits, dependency replacements, task changes, or live service
restarts were made for this follow-up. This localization patch is not present in
the deployed `v2.1.6+statusfix` binary and remains uncommitted locally.

`traffic/proxy_message_test.go` exercises the actual callback through a loopback
HTTP test server: root GET, nested GET with an unreflected query value, POST, and
HEAD. Assertions cover status, response text, content type, `nosniff`, and HEAD's
empty response body. The original callback test uses no database, task engine,
or forwarding destination. The later integration test above additionally reaches
the dependency's proxy dispatcher; neither exercises CONNECT/forwarding.

Evidence from an isolated source snapshot based on
`f55dc85ace7de4a7aa11308437d48fa70a53bb77`:

```text
PASS: new regression fails on the unpatched callback (status 200, no message)
PASS: gofmt and go vet ./traffic
PASS: normal tests: 12 top-level, 23 including subtests, 0 skipped
PASS: race tests:   12 top-level, 23 including subtests, 0 skipped
```

The baseline's HTTP 200 is from testing the inherited no-op callback alone;
the full dependency's fallback is what supplies the live HTTP 400 and Chinese
text. This distinction is intentional in the regression evidence.

Evidence directory:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-proxy-message-1seec538`
contains baseline failure output, normal/race test JSON logs, vet/format logs,
`identity.json` with source hashes, and `summary.json`.

At this earlier verification point, only localization was fixed locally. The
latest follow-up above records the subsequent read-only telemetry work. The
deployed UI patch's evidence below is unchanged.

### Previously deployed UI patch

Status: **display-only patch implemented, validated, committed and deployed to the
local compose service on 2026-11-15.** The original diagnosis and upstream
references below refer to the base revision, so their line numbers are not line
numbers in the modified working tree.

Implemented:

- `web/src/hooks/use-task-observation.ts`: one read-only tasks/stats observation
  shared by header and Overview, with task identity validation, cancellation,
  request-generation protection, bounded requests and stale/unavailable states.
  Successful observations retain their local receipt timestamp. Freshness here
  means a recent successful HTTP observation, not proof of engine health.
- `web/src/components/task-observation-status.tsx`: consistent status projection,
  terminal lifecycle precedence in the UI, honest inactivity wording and shared
  observation freshness labels.
- The detail and Overview pages consume that shared observation. Recent activity
  uses `last_activity_unix * 1000` with the selected locale. An unconfigured LLM
  no longer receives a configured badge.
- Detail and task-list rerun presentation now requires a terminal lifecycle;
  inactivity alone does not expose it. The existing terminal action explicitly
  says it resets the run clock. Detail execution controls are disabled when the
  status observation is stale/unavailable.
- The stats contract describes the actual nested flags/counters without inventing
  a nested mode. Mock stats matches that shape; the unused string timestamp alias
  was removed. All three locale resources include the new labels.

### Verification evidence

An isolated working-tree snapshot was built and exercised with Chromium. No Go
service, database, or live task was connected to the fixture.

```text
npm-ci: PASS
frontend-build: PASS
PASS mapping-and-unix-timestamp
PASS shared-refresh
PASS outage-retains-marked-stale-data
PASS observation-recovery
PASS initial-outage
PASS reject-wrong-task-observation
PASS terminal-done
PASS terminal-failed
PASS terminal-timeout
PASS missing-time-and-unconfigured-provider
PASS late-response-after-task-switch
PASS task-list-no-inactivity-rerun
{"result":"PASS","scenarios":12,"writeRequests":0,"externalRequests":0,"javascriptErrors":0}
```

Evidence directory:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-status-fix-_7bwmmo_`

It contains `identity.json` (base commit and SHA256 of modified source/test files),
`frontend-build.log`, `browser-results.json`, and screenshots for each scenario.
The build includes TypeScript checking. `git diff --check` also passed.

The maintained regression harness is `scripts/check-task-status-ui.cjs`.
To exercise an existing static export served on loopback, run it with
`STATIC_UI_URL` set to that server's origin and Playwright installed or provided
through `PLAYWRIGHT_PATH`; `CHROMIUM_PATH` can select an installed browser.
The harness rejects external origins and all non-GET API requests. It does not
exercise control handlers or establish live database persistence guarantees.

### Deployment evidence (2026-11-15)

Image `antifox:v2.1.6-statusfix` was assembled from the patched working tree:
isolated `npm run build:static` export embedded into a `-tags embedui` Go binary
(version stamp `v2.1.6+statusfix`), copied into a container based on the exact
running image `ghcr.io/chathurangabw/antifox:v2.1.6` (base layer
`sha256:170177dd2dfafeea0547d6576227ef2da55fc28c8a86bc05a3b9bdc781a1b3fe`).
The non-clean semver stamp also disables the built-in one-click updater for this
build, so an in-app update cannot silently replace the patch.

Before touching production:

- `pg_dump -Fc` of the live database plus SHA-256 checksum, compose files,
  `.env`/`.env.example`, a task count, and a per-task row snapshot were written
  to `/home/Sniffer/antifox-backups/pre-deploy-20261115-025814/`.
- The image passed a disposable-database smoke test (pgvector/pgvector:pg16,
  separate network): `/api/health` reported `v2.1.6+statusfix`, the embedded SPA
  served, and both new UI labels were present in the binary.

Deployment replaced only the `antifox` service image in
`docker-compose.override.yml`; PostgreSQL and the `data/` and `skills/` bind
mounts were untouched. After recreation:

- `/api/health` reports `{"ok":true,"service":"antifox","version":"v2.1.6+statusfix"}`.
- The running binary contains the new labels (`resets run clock`,
  `No recent recorded activity`).
- All five persisted task rows are unchanged (tasks 1-4 `done`, task 5
  `created`, none paused; exploration 5 still has 829 activity rows).
- As expected for this application, restarting the process resumed the
  active task: the planner ran a round for task 5 immediately after startup
  and completed normally (logs `22:58:31 -> 22:58:50` local container time).
  No planner error appears in the inspected startup logs.

No registry push was performed; the patched image exists only on this host, and
the compose override now references it. A live authenticated browser pass was
not performed (no credentials in this session); the UI behavior is verified by
the offline scenarios above and by the embedded-label checks below.

### Remaining scope

For the previously deployed UI patch, `server`, `agent`, and `db` were unchanged;
backend telemetry and server-side rerun restrictions were not included. The
latest local changes above address the targeted telemetry concerns but have not
been deployed. The Phase A/B design below is therefore not implemented in full
on the live service. The earlier database backup and row snapshot record the
pre-deploy state, and post-deploy queries checked selected task fields. Those
checks do not prove that all saved data was unchanged: startup resumed task work,
which can write new activity and other task data.

## Conclusion

Two different issues must remain distinct:

1. The latest task (#5) ended its last planner turn normally, with unfinished
   goals and no queued/running work items. Its recorded explanation is an
   external prerequisite blocker. No evidence in the inspected logs establishes
   an engine crash. The reported external cause was not independently tested.
2. There are reproducible API/UI status defects, plus a control-action coupling
   that makes a naive display fix unsafe for run-clock continuity. Renaming
   every `stalled` state to `blocked` would not be a sufficient correction.

The original investigation changed no application source or live state. The
subsequent local display-only implementation is documented above; the broader
solution below remains partly proposed and is not deployed. Browser checks use
an isolated static build with mocked API responses and no live backend.

## Upstream attribution: confirmed 2026-11-15

**The identified status defects and rerun/reset coupling were inherited from
upstream. They were not introduced by this fork's branding or release changes.**
This conclusion applies to the specific paths below, not every fork change or
the unverified external prerequisite reported by task 5.

### Revision identity and method

- GitHub's commits API resolved upstream `AntiFox-AI/antifox` `main` to
  [`070f798bf590667bc2a0a13c687dd87d8a4413bf`][upstream-commit], dated
  `2026-10-31T07:46:04Z`.
- The API reports tree `c508aee3f989446e4add05e5c3a8861c3a56d9ef`.
  The existing local Git object resolves to exactly that tree. No upstream
  merge or checkout was required.
- `git merge-base HEAD 070f798bf590667bc2a0a13c687dd87d8a4413bf`
  returns the upstream commit itself: it is an ancestor of this fork's inspected
  HEAD, `4021ceda702d337f5428cce8734a0ae58cef2aef`.
- Compared upstream blobs with HEAD and traced relevant lines using
  `git blame --root`. The relevant implementation already appears in upstream's
  root snapshot, [`cfebba61f980484bec397684f551b64b89c89d00`][upstream-root].
  That is the earliest evidence in this repository, not proof of who originally
  wrote it or whether it predates the import into AntiFox.
- Upstream attribution is based on source/history. The browser reproductions
  below used the fork revision; an upstream service was not deployed or tested.

### Attribution matrix

All line references in this table point to the immutable upstream revision.

| Finding | Upstream evidence | Fork comparison / attribution |
| --- | --- | --- |
| Header reads nonexistent nested mode | [Detail lines 88-109][upstream-detail] reads `active_task.engine_mode`; [stats lines 868-889][upstream-stats] emits top-level `engine_mode` | Inherited. Detail file differs only in one translated comment; stats handler is identical. |
| Header becomes stale while Overview refreshes | [Detail lines 77-109][upstream-detail] refresh approvals separately; [Overview lines 58-85][upstream-overview] polls its own state every 3 seconds | Inherited. Entire Overview file is byte-identical. |
| Activity timestamp missing | [TaskDTO line 42][upstream-dto] emits `last_activity_unix`; [Overview lines 125-127][upstream-overview] reads `last_activity` | Inherited. Same serialized field, client access and lack of normalization in [api.ts line 135][upstream-api]. |
| Mock masks real contract mismatch | [Mock lines 93-111][upstream-mock] includes nested mode; [Stats interface lines 648-654][upstream-types] omits `active_task` | Inherited. Mock stats function is unchanged; assertion and incomplete contract predate the fork. |
| Inactivity ignores planner count and terminal precedence; errors discarded | [Stats lines 844-889][upstream-stats] counts running intent rows, uses the same 60-second rule, and discards lookup errors | Inherited. Entire stats handler is byte-identical. |
| Started marker is not a health probe | [Engine lines 152-156][upstream-engine] checks map membership; [counter lines 40-50][upstream-counter] exists separately | Inherited. Relevant functions are unchanged apart from surrounding comments. |
| Inactivity is coupled to a clock-resetting rerun | [Detail lines 181-184][upstream-detail] and [task list lines 419-447][upstream-list] treat stalled as stopped; [control lines 1009-1018][upstream-control] calls reactivation; [DB lines 175-184][upstream-reset] clears clocks | Inherited. UI logic and DB reset are unchanged; [manager lines 725-742][upstream-manager] also clears in-memory timestamps. |

Nine comparison units matched after removing only blank lines and standalone
`//` comments: stats handler, Started, in-flight counter helpers, manager
reactivation, database reactivation, mock stats, detail page, Overview, and task
list. This comparison does not remove executable statements or string literals.
Upstream blame attributes the decisive status/mapping/reset lines to the root
snapshot. Fork history starts after upstream with `d98512f` (English conversion);
the detail-page change translates a comment rather than changing the logic.

### How upstream builds it, and why a successful build missed this

The checked-in [release workflow][upstream-release] runs on `v*` tags:

1. Node 22 installs frontend dependencies and exports the Next.js UI as static
   files using `npm run build:static`.
2. The exported files are copied into `server/webui/dist` and embedded into Go
   binaries built with the `embedui` tag. Five OS/architecture combinations are
   packaged, followed by release checksums and publication.
3. The container job reuses prebuilt Linux binaries. Despite a multiarchitecture
   comment, its configured build target at this revision is `linux/amd64`.

This workflow has no `go test`, browser-test, or API/UI contract-test step.
The upstream [frontend package scripts][upstream-package] also have no test
script. This describes the checked-in automation; it does not establish whether
maintainers performed separate manual/private testing.

A successful TypeScript build cannot validate the real JSON shape when the
consumer asserts an incorrect shape. The inaccurate mock supplies precisely
the extra nested field the header expects. These are concrete coverage gaps;
the fork's earlier passing Go/race suites likewise did not establish correct
browser status behavior.

### Final correction decision

Keep this fork's saved data and apply the coordinated reporting correction in
Phases A and B below. Reverting to upstream would retain the diagnosed defects.
Fix field mapping, timestamp rendering, shared refresh, and the inactivity/action
coupling together; a one-line mode fix or relabeling alone is insufficient.
Use backend-shaped inert fixtures and require the acceptance matrix below before
calling the correction validated. Deployment remains a separate operation.

The rerun clock reset is explicitly documented upstream as intentional for a new
run after timeout. The defect is treating an inactivity indicator as a reason to
offer that operation under ambiguous continuation wording, not the mere existence
of a deliberate reset operation.

[upstream-commit]: https://github.com/AntiFox-AI/antifox/commit/070f798bf590667bc2a0a13c687dd87d8a4413bf
[upstream-root]: https://github.com/AntiFox-AI/antifox/commit/cfebba61f980484bec397684f551b64b89c89d00
[upstream-detail]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/app/%28main%29/function/tasks/detail/page.tsx#L77-L109
[upstream-stats]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/server.go#L813-L891
[upstream-overview]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/app/%28main%29/function/tasks/detail/_tabs/overview-tab.tsx#L58-L127
[upstream-dto]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/dto.go#L31-L50
[upstream-api]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/lib/api.ts#L135-L144
[upstream-mock]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/lib/mock/data.ts#L93-L111
[upstream-types]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/lib/types.ts#L648-L654
[upstream-engine]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/engine.go#L152-L156
[upstream-counter]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/engine_timeout.go#L40-L50
[upstream-list]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/src/app/%28main%29/function/tasks/page.tsx#L419-L447
[upstream-control]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/server.go#L990-L1029
[upstream-reset]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/db/tasks.go#L175-L184
[upstream-manager]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/server/manager.go#L725-L742
[upstream-release]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/.github/workflows/release.yml
[upstream-package]: https://github.com/AntiFox-AI/antifox/blob/070f798bf590667bc2a0a13c687dd87d8a4413bf/web/package.json#L5-L16

## Current-session evidence

Read-only database observation at `2026-11-14 22:27:44 UTC`:

- Task 5: persisted status `created`, paused false, timeout 0; first-run,
  deadline and completion timestamps NULL. The API derives a running status
  from in-memory engine flags, so the database value alone does not mean the
  task never executed.
- Two goals remain open. Seven intents are done, one blocked, and none are
  open/running.
- Last activity: row `20574`, planner result, outcome `completed`, timestamp
  `2026-11-14 22:00:26.395326 UTC`, no cancellation cause.
- The final result explicitly states that goals remain unmet and no new intents
  were created in that round. Its reported reason is an external connectivity
  prerequisite, not a planner exception.
- The inspected log window from `21:59 UTC` records planning at `21:59:54` and
  normal completion at `22:00:26`; no planner error, activity-write error or
  panic was found in that bounded window.

`completed` on an activity describes that agent turn, not completion of every
task goal. A normal planner return and a blocked overall task can coexist.
The runtime's 60-second inactivity rule is therefore consistent with these
observations, even though the label is insufficiently explanatory.

## Confirmed UI defects

### 1. Header reads the engine mode from the wrong object

- Backend: `server/server.go:899-919` puts `engine_mode` at the **top level** of
  `/api/stats`. `active_task` contains flags/counters, not `engine_mode`.
- UI: `web/src/app/(main)/function/tasks/detail/page.tsx:95-101` reads
  `at.engine_mode`, then line 181 falls back to `idle`.
- `TaskDTO` does not supply an engine mode from `/api/tasks` either
  (`server/dto.go:31-50`).

**Browser reproduction:** one backend-shaped fixture produces **Idle in the
header and Stalled in Overview**, simultaneously. The header also omits the
stalled action because neither the correct mode nor `active_task.stalled` is
transferred into its local task state.

### 2. Header and Overview have different refresh lifetimes

- Detail page `load()` runs initially and after selected local actions
  (`detail/page.tsx:88-109`); the 5-second timer above it only refreshes pending
  approvals, not task/engine state.
- Overview independently refreshes every 3 seconds
  (`detail/_tabs/overview-tab.tsx:58-85`).

**Browser reproduction:** start with a paused fixture, then change the mocked
API to exploring without clicking any execution controls. Overview changes to
**Exploring**, while the header and its button remain **Paused / Resume**.

### 3. Recent activity reads an absent timestamp field

- Task API emits `last_activity_unix` in seconds (`server/dto.go:42`;
  `server/server.go:946-950`). Stats separately emits numeric
  `active_task.last_activity`.
- Overview reads `task.last_activity` as a string
  (`overview-tab.tsx:125-127`). `api.tasks()` does not normalize or alias it
  (`web/src/lib/api.ts:135`).

**Browser reproduction:** a valid `last_activity_unix` is supplied, but the UI
renders **Recent activity: —**. Any fix must convert seconds to milliseconds;
passing Unix seconds directly to `Date` would introduce a different bug.

### Why the frontend type check did not catch this

`Stats` omits `active_task` (`web/src/lib/types.ts:659-665`). The page uses a
type assertion to treat that nested object as `Partial<Task>`, whose optional
`engine_mode` and alternate timestamp fields make the incorrect access legal.

The demo mock also includes a nested `active_task.engine_mode` that the real
server does not emit (`web/src/lib/mock/data.ts:103-110`). This can conceal the
contract mismatch in demo-driven UI review.

## Source-backed status and continuity concerns

These are confirmed code paths/limitations, not assertions that every case
occurred in task 5.

### 4. Stalled is an inactivity heuristic, not a diagnosis

`server/server.go:879-897` counts only intent nodes marked running, then checks
whether the last recorded activity is more than 60 seconds old.

- An executing planner is not part of that worker-intent count. The engine has
  a separate planner/worker in-flight counter (`server/engine_timeout.go:40-50`;
  `server/engine.go:433-436`), but stats does not consult it. A quiet planner
  request can therefore be labelled stalled while still in flight.
- Stats does not prioritize terminal task status, unlike `/api/tasks`
  (`server/server.go:935-943`). A terminal task with a retained started marker
  can be described as exploring/stalled even though execution refuses terminal
  tasks (`server/engine.go:410-414`).
- `Started()` checks map membership, not independently measured goroutine health
  (`server/engine.go:153-156`).
- Database read errors in the stats path are discarded. A failed intent lookup
  can look like an empty work set; unavailable telemetry should not establish
  a blocked or stalled state.

Do not interpret these observations as justification to restart anything.
They call for more accurate read-only status reporting.

### 5. A naive field fix can expose a clock-resetting action

`detail/page.tsx:183-184,217-220` treats `stalled` like a terminal/stopped task
and offers **Continue running**. That button sends `rerun` (lines 127-132).
The task-list menu has the same coupling (tasks/page.tsx:419-447).

`server/server.go:1040-1046` calls `ReactivateTask`, whose database update
(`db/tasks.go:181-184`) sets status to running and clears `completed_at`,
`first_run_at` and `deadline_at`. The in-memory timestamps are also cleared
(`server/manager.go:725-740`). It preserves the graph, but it is **not** merely
a display refresh or a continuation that preserves run-clock semantics.

The browser reproduction shows that this action is currently absent for the
stalled fixture because of defect 1. Correcting the mode binding without also
correcting action visibility can expose this unintended path. This is a
source-backed regression risk; no live control action was invoked to test it.

Task 5 currently has no timeout, so its existing deadline was not lost. The
clock-reset concern applies to timed tasks and to claims about continuity.

## Recommended solution

### Phase A: correct the display contract and protect continuity

1. **Use a typed stats contract matching the server.** Define the actual
   `active_task` shape separately from `Task`; read mode from
   `stats.engine_mode`. Remove the `Partial<Task>` assertion and align demo
   fixtures with real serialization. Do not add a second conflicting mode
   location just to accommodate the faulty client.
2. **Share one task-state observation between header and Overview.** Refresh it
   consistently, scope responses to the selected task id, cancel/ignore stale
   requests, and retain an observation timestamp. A request failure should
   display stale/unavailable state, not silently imply that old state is fresh.
3. **Fix timestamp rendering.** Use `last_activity_unix * 1000` from tasks, or
   normalize the numeric stats timestamp in one adapter. Treat absent/zero
   values explicitly and use the selected locale instead of hardcoding zh-CN.
4. **Decouple inactivity from rerun controls in the same change.** A stale or
   blocked label must not expose a terminal-task restart action. Make any
   existing deliberate rerun clearly state that it resets run-clock semantics;
   reject inappropriate state-reset requests server-side as well as hiding
   misleading controls. Status polling must remain read-only.
5. **Use honest wording before adding new classification.** Keep the existing
   wire enum for compatibility initially, but describe the `stalled` heuristic
   as **No recent recorded activity** with its time and counter context. Show
   the latest planner explanation separately as **Reported reason**, with its
   activity id/time; do not turn arbitrary prose into a verified diagnosis.

These changes require no task-data migration, no resetting clocks or task ids,
and no changing orchestration behavior. They correct the operator's view.

### Phase B: improve observational state classification

Maintain a distinction between durable task lifecycle, observed execution phase,
and observation freshness. In the status projection:

- Terminal lifecycle remains terminal; don't label a completed task stalled
  merely because a loop was started earlier.
- User pause takes priority over inactivity inference.
- Account for known in-flight planner activity separately from running workers.
  An in-flight counter alone also does not prove progress: surface elapsed
  inactivity instead of treating it as an unconditional health guarantee.
- Only show **Waiting on prerequisite / Blocked** as a distinct state when an
  explicit, sourced wait reason exists. Otherwise retain an unknown/no-recent-
  activity explanation. Zero workers is insufficient evidence of the cause.
- On missing/erroring telemetry, expose unavailable/stale status rather than
  zero counters presented as a fresh successful observation.

No automatic retries, task restarts or new execution triggers are necessary
for this reporting solution. It does not claim to resolve task 5's external
prerequisite or guarantee automatic continuation when that prerequisite changes.

## Acceptance criteria before merging

| Scenario | Required result |
| --- | --- |
| Backend-shaped stalled response | Header and Overview agree; valid activity time appears |
| Paused to exploring through a newer observation | Both views update; no stale Resume presentation |
| Genuine individual task switch during a slow request | Old response cannot overwrite the newly selected task |
| Stats/network failure | Existing data is visibly stale/unavailable, not presented as fresh |
| Planner in flight with no running worker intents | Planner activity is not confused with an empty engine |
| Done/failed/timeout task with retained engine marker | Terminal lifecycle wins |
| No recent activity without a known reason | No fabricated connectivity or blocked diagnosis |
| Explicit reported prerequisite | Reason source/time shown; task remains unfinished |
| Viewing, refreshing or changing a status label | No control/mutation request; task graph, findings, transcript and run timestamps unchanged |
| Deliberate terminal rerun | Separate explicit action; reset semantics disclosed and tested |

Tests should exercise real response shapes rather than the current inaccurate
demo shape. Backend projection tests can use inert fixtures and a fake clock;
browser tests should mock APIs without running task work.

## Continuity and rollout limits

Saved-progress continuity and uninterrupted execution are different guarantees.
A read-only UI correction can preserve all saved state and leave the engine
untouched. The deployed Go binary embeds the frontend, however, so publishing
new code into that process normally entails a restart. Do not promise zero
interruption merely because the source change is UI-only. Validate in isolation
and coordinate deployment separately with active-session handling. Zero counted
worker intents alone does not prove that no requests or detached processes exist.

## Reproduction evidence

Isolated export built successfully (including TypeScript) from the commit above.
Chromium reproduced defects 1-3 with backend-shaped mocked responses:

- Header Idle / Overview Stalled, together.
- Recent activity blank despite valid Unix timestamp.
- Header Paused / Overview Exploring after a refreshed observation.

No JavaScript exceptions and no API mutation requests occurred in these fixtures.
These screenshots are synthetic UI reproductions, not live task screenshots.

Evidence directory:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\antifox-status-analysis-ll866d3i`

Files: `identity.json`, `frontend-build.log`, `browser-results.json`,
`header-idle-overview-stalled.png`, and
`header-stale-paused-overview-exploring.png`.

Harnesses:
`C:\Users\Sniffer\AppData\Local\Temp\opencode\diagnose-antifox-status.py` and
`diagnose-antifox-status.cjs` in the same temporary-tool directory.
