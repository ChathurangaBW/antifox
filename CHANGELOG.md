# Update Log - Changelog

This project's version update summary. The format follows [Keep a Changelog](https://keepachangelog.com/), and versions follow [SemVer](https://semver.org/).

## v2.1.7 "Kit"

Task status reads the same everywhere, and inactivity no longer looks like a reason to rerun.

### Fixed

- **One task status for the header and the Overview**: both now read a single shared observation of the task's stats, taken from the backend's real `engine_mode`, so the header can no longer say Idle while the Overview says Stalled. A finished (terminal) task always shows as finished. If the stats are stale or cannot be fetched, the page says so instead of showing old data as current, and actions are disabled until fresh data arrives. The status updates together after pause and resume.
- **Recent activity shows its real time**: it read a field the backend does not send and showed "—"; it now converts `last_activity_unix` in the selected language.
- **Rerun only for finished tasks**: an inactive task no longer shows a rerun control on the task page or in the task list. Finished tasks keep it, labelled "Rerun (resets run clock)".
- The "LLM not configured" badge no longer appears when a provider is configured; types, mock data and translations match the backend's actual responses.

### Added

- `scripts/check-task-status-ui.cjs`: a read-only browser check of 12 status scenarios that refuses any write request or external address. `scripts/engine-state-analysis.md` records the analysis behind this fix. Stall detection in the backend is unchanged.

### Verified

- In a clean checkout: `go vet`, and the full suite against PostgreSQL 16 + pgvector (203 passing tests, no failures or skips); frontend type check and static export; documentation link check.

## v2.1.6 "Bengal"

Login and setup screens in the new brand.

### Changed

- **Branded login and setup screens**: the stock city photo with a blue overlay is replaced by a backdrop drawn from the guardian shield: a charcoal field, nested shield outlines, the logo's "A" chevron, an orange glow behind the logo and an orange baseline. The "AntiFox" name is cream with an orange "Fox" (it was blue and purple). The setup screen, previously the plain theme colour with rings, uses the same backdrop. The backdrop is inline SVG in the logo's sampled colours (`web/src/components/brand-backdrop.tsx`), so it needs no image request and stays sharp at every size.
- `login.jpg` (395 KB) is no longer used and is removed from the bundle.

### Verified

- Frontend type check and static export passed. In Chromium against the embedded UI: the setup screen (empty database) and the login screen (copy of production data) render the backdrop and the logo with no console errors or failed requests.

## v2.1.5 "Swift"

New logo, and the UI no longer shows a stale one after an upgrade.

### Changed

- **Guardian shield logo** on the login screen, the setup screen, the sidebar, the browser tab icon, the Apple touch icon and the README.

### Fixed

- **Old logo and pages could be shown from cache after an upgrade**: the embedded UI was served without cache headers, and `/logo.png`, the icons and the pages keep the same URL across builds. Files under `_next/static/` (content-hashed names) are now cached for a year as immutable; every other UI file is sent with `Cache-Control: no-cache`, so the browser revalidates it and shows the new version.

### Verified

- `go vet`, and the full suite against PostgreSQL 16 + pgvector: 203 passing tests, also under the race detector, no failures or skips.
- In Chromium against the embedded UI: `/logo.png` is the new artwork (byte-identical to `web/public/logo.png`) with `no-cache`; the login panel and the sidebar render it; hashed assets carry the immutable policy.

## v2.1.4 "Corsac"

Closes the follow-ups from the v2.1.3 review: task references are now enforced by the database itself, non-text tool output has a size limit, cancellations record a stable cause code, and the runbook and audit actually ship. An upgrade and rollback rehearsal on a copy of production data found one more gap, fixed here: after rolling back, the older version's task deletion left dangling references.

### Fixed

- **Task references are enforced in the database**: v2.1.3 checked task ids at the tool and API before writing, which could not stop a task being deleted while an asset write referencing it was in flight, and did nothing when a task was deleted. The database now refuses any new asset reference to a task that does not exist (trigger `trg_assets_task_ids`, which also locks the referenced task until the write commits), and deleting a task removes its id from every asset (trigger `trg_tasks_remove_asset_refs`). Both are in the database, so they hold for direct SQL and for an older application version run after a rollback. Rows that already hold an old dangling id can still be updated; only ids being added are checked.
- **Non-text tool output has a size limit**: v2.1.3 limited non-text blocks (such as images) only by count (8 per result). They now also have a total size limit of 10 MiB, measured as the serialized blocks; blocks past either limit are omitted and the notice states both limits. The size limit is deliberately far above the text budget so that full-size screenshots pass unchanged (a test keeps two 1 MB blocks under the default settings); it only stops pathological results. Today's tools return text only (MCP results are converted to text), so this protects future image-returning tools rather than changing current output.

### Added

- **Cancellation cause codes**: a cancelled run's result row stores `cancel_cause` alongside the readable reason: `replaced_by_message`, `operator_stop`, `task_paused`, `work_killed`, `task_completed`, `task_deadline`, `shutdown`, `run_deadline` or `unknown`. The activity APIs return it, so interruptions can be filtered and counted without parsing text. The transcript marks each result with `data-outcome` and `data-cancel-cause`.
- **Prepared repair for the task-75 references**: [the repair](scripts/repairs/2026-11-14-task-75/README.md) records why six production assets reference a task that never existed (one `insert_assets` call from task 2), with guarded apply and rollback scripts. The upgrade does not apply it; applying it is a separate operator step.

### Release process and documentation

- [The release runbook](docs/release-runbook.md) and [the read-only task-reference audit](scripts/audit-task-references.sql) are now committed. Neither was in the v2.1.3 tag. The runbook records the v2.1.4 rollback rehearsal.
- **Documentation link check**: the test workflow fails if any repository-relative link in `CHANGELOG.md`, `README*.md` or `docs/` does not resolve in a clean checkout (`scripts/check-doc-links.py`), so release and publish are gated on it too.

### Verified

- `go vet`, and the full suite against PostgreSQL 16 + pgvector: 202 passing tests (including subtests) on two consecutive runs and under the race detector, with no failures and no skipped tests. Frontend type check and static export passed.
- **Upgrade and rollback rehearsal** on a copy of the production database (restored from backup to a disposable database on an internal Docker network with no internet access, tasks paused): the new build upgraded it; the v2.1.2 image then ran against the upgraded copy (every read endpoint checked returned 200, it wrote an asset, deleted a task and stopped cleanly) and the database still refused a missing task id and cleaned the deleted task's references; the new build re-upgraded it. The operator's `list_assets` definition was unchanged throughout. Agent runs were not part of the rehearsal.
- **Browser check** in Chromium against the embedded UI and the same data copy: a cancelled result renders muted (grey, dashed border, italic) with its cause code, a failure renders red, a completed answer renders normally, with no console errors.
- The task-75 repair was rehearsed on copies at both the v2.1.2 and the v2.1.4 schema: apply, refused repeat, exact rollback, refused repeat rollback, re-apply.

### Upgrade notes

- Schema changes are additive: a `cancel_cause` column on both activity tables and two triggers. Creating the triggers does not touch existing rows.
- Production still holds six references to the nonexistent task 75 until the prepared repair is applied.

## v2.1.3 "Arctic"

Addresses the reviewed migration, ingestion-validation, cancellation-reporting, output-budget, startup and release-gate defects. Independent checks of the tagged source on PostgreSQL 16 + pgvector passed 196 tests and 196 race-detector tests with no failures or skips. These results do not establish deployment, historical-data repair, or upgrade/rollback compatibility on production data.

### Fixed

- **Tool upgrades no longer overwrite operator edits (D1)**: the `list_assets` upgrade decided "stock or customized" by the first words of the description and then replaced the whole definition, so an edited description that kept the stock opening, or an edit to a parameter, was lost. It now compares against every definition the tool actually shipped with (recorded from each release tag and upstream) and decides per part: the description and each parameter are replaced only if they exactly match a shipped version, and operator-added parameters are kept. The write applies only if the row is unchanged since it was read.
- **Upgrades are recorded only when they happened (D2)**: a current description with an old schema was marked done without an upgrade, and a missing tool row was marked done too. Completion is now recorded only after re-reading the row and confirming every current parameter is present; a missing row retries on the next start. Every migration now logs a failed completion write instead of discarding it. Databases re-check once under a new flag.
- **Invalid task references are rejected at the reviewed ingestion entry points (D3)**: `insert_assets` outside a task and `POST /api/assets` now reject supplied negative or nonexistent task ids; omitting the id inserts the asset unlinked. This pre-write lookup does not repair historical references or prevent a later/concurrent task deletion from leaving an orphan in the `task_ids` array, which has no foreign key.
- **Interrupted runs say why (D4)**: a turn replaced by your next chat message, an operator stop, a pause, the planner killing a work, a task deadline, a completed task and a server shutdown all showed the same "suspended / termination / restart" text. Gracefully cancelled runs now name the cause in result text ("Interrupted: replaced by a newer message"). Result rows carry an `outcome` (completed, cancelled, failed, budget reached); the cause is not a separate persisted code, and historical rows remain nullable. A cancellation is no longer marked as an error, and the transcript shows it in a muted style instead of error red. Abrupt process termination cannot guarantee a final persisted result.
- **Text output has an aggregate budget (R1)**: the cap now covers text across the result, with additional omission-notice overhead. Excess text uses spill-to-file storage when available, otherwise head/tail truncation. Non-text blocks have a separate count limit; excess non-text blocks are reported and omitted, not saved. This is not an overall byte limit on image payloads.
- **Startup cannot hang on the secret handoff (R2)**: the handoff wrote up to 32 KiB into a pipe before the reader existed; Linux shrinks pipes to 4 KiB under pipe-memory pressure, which could block startup. The payload limit is now 4 KiB and the write is non-blocking and single-shot: anything that does not fit falls back to the previous behavior and logs it, without the secret.

### Release process

- **Publishing is gated on tests (D5)**: a new `test` workflow runs on every push and pull request, and both `release` and `publish package` require it. It runs `go vet`, the full suite against PostgreSQL 16 + pgvector (any skipped test fails the gate), and the race detector, and keeps the JSON results and the tested commit as an artifact.
- **Image platform stated correctly (D6)**: the published container image is linux/amd64 only. Release binaries also cover linux/arm64, macOS and Windows. Unused arm64 steps were removed from the image job.
- **Release runbook (E1), documentation correction after the tag**: the original `v2.1.3` source archive did not include the locally ignored runbook. The working-tree correction makes [the runbook](docs/release-runbook.md) eligible for tracking and fixes validation, backup and rollback guidance; it is not retroactively part of the published tag. The [read-only task-reference audit](scripts/audit-task-references.sql) supports separate historical-data review.
- `docker-compose.yml` now points at this fork's image, `ghcr.io/chathurangabw/antifox`.

### Outstanding operational evidence

- Confirm the actual running image/revision, health version and migration state independently of release publication.
- Rehearse restoration, upgrade and rollback on an isolated production-data copy, and check cancellation rendering in a browser.
- Review provenance before repairing historical orphan task associations. New input validation is not a historical-data repair.

### Changed

- **Verification label wording (E3)**: "Verified" is now "Evidence verified", and its explanation says what the check proves (every quote appears word for word in the linked traffic, at proof level 3 or higher) and what it does not (the vulnerability itself still needs review).

### Upgrade notes (behavior changes since v2.1.1) (E2)

Deliberate changes that can affect existing setups:

- **Agent shells no longer receive the database DSN or model API keys.** A custom tool or script that read `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `ANTIFOX_PG_DSN` or `ANTIFOX_LLM_PROXY` from its environment now gets nothing; pass what it needs explicitly.
- **The planner and main agent pass the guard.** Destructive commands are blocked for them as for workers. An intercept rule with action "ask" makes the main agent wait for your approval in the task chat, and makes the planner skip the command and hand it to a worker.
- **A first install needs a setup token** from the server log (`docker compose logs antifox | grep "Setup token"`).
- **Changing a model profile's provider or base URL requires re-entering its API key.**
- **Asset task ids must exist.** Integrations that posted made-up task ids to `POST /api/assets` now get 400.
- **Goal decomposition, report narrative and enrichment** run on environment-configured models, so model usage rises.

## v2.1.2 "Fennec"

Security hardening ported from ARTEX and Norma, plus fixes for the tool errors recorded in production. Every item below was reviewed against the reference implementation and covered by a test.

### Security

- **Secrets no longer reach agent shells**: the binary reads `ANTIFOX_PG_DSN`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` and `ANTIFOX_LLM_PROXY` at startup, then removes them from its environment before anything can spawn a child process. Agent shells, custom tools and MCP servers no longer inherit the database password or model keys. On Linux the binary goes one step further than ARTEX: it re-executes itself at startup (same PID) with the secrets passed through a pipe instead of the environment, so `/proc/<pid>/environ`, which an agent shell running as the same user could read, never contains them either. On other platforms the start environment keeps them, as in ARTEX. (ARTEX #136, #146, extended)
- **Planner and main agent shells are now gated**: both have shell and file tools and previously ran with no safety hooks. The main agent now passes the same guard as a worker, with approval requests appearing in the task stream. The planner gets the same deny rules; an approval rule blocks its call and tells it to hand the command to a worker, because no operator is watching the planner. (ARTEX #190, extended to user deny rules)
- **Prompt-injection rules**: code-owned rules telling every agent that target-derived content is data, never instructions, appended after the prompt so a UI edit cannot remove them. The planner's version is adapted to this platform's tools. (ARTEX #49, #189)
- **Saved API keys stay with their endpoint**: the connection test, the global LLM save and profile edits only reuse a saved key for the provider and base URL it was saved with. Changing the endpoint requires re-entering the key. Previously a request could name its own base URL and receive the saved key. (ARTEX #142, extended to the save and edit paths, which ARTEX still leaves open)
- **First-run setup token**: until an admin exists, `/setup` requires a one-time token printed to the server log (`docker compose logs antifox | grep "Setup token"`), so whoever reaches the port first can no longer claim a fresh install. (ARTEX #137)

### Fixed

- **Environment-only model configuration disabled four features**: goal decomposition, the AI report narrative, finding enrichment and transcript finding extraction only looked for a saved profile. Deployments configured through environment variables silently skipped all four; every task ran on its raw goal as a single goal. The startup environment config is now the fallback.
- **The server could crash on a tool call**: thirteen exploration-graph tools dereferenced a missing store when a chat agent (pentest, auto, custom) called them, and tools run on their own goroutine, so one call ended the process. They now return a clear task-context error, and the engine recovers from any tool panic.
- **Planner could not search assets**: its prompt told it to call `list_assets`, which it did not have; 23 production failures. The tool is now in the planner's set and bound on upgraded databases.
- **Invented asset IDs**: intents, facts, hints and findings now reject asset IDs that do not exist instead of failing on a foreign key.
- **Phantom tool names in prompts**: references to `asset_neighbors` and `update_skill`, which never existed, are removed.
- **Shells could hang on a pager**: every shell now gets `PAGER=cat`, `GIT_PAGER=cat`, `SYSTEMD_PAGER=cat` and `DEBIAN_FRONTEND=noninteractive`. (Norma)
- **Unbounded tool output**: a single cap now bounds every tool's output, including MCP and custom tools; oversized output spills to a file the agent can read, now also for the planner and main agent, so nothing is lost. Truncation no longer splits multi-byte characters, and spill files are owner-only. (Norma)
- **Semantic search could be disabled by a startup race** on `CREATE EXTENSION vector`.
- **Operator-edited prompts survive upgrades**: built-in prompt updates skip prompts not authored by the system.
- **`get_worker_output` reports the real lifecycle state**, and `insert_assets` keeps the bound task.

### Added

- **Machine-checked finding verification** (adapted from ARTEX S1): `report_finding` takes two optional fields, `poe_level` (proof-of-exploitation 0-4) and `evidence_quotes` (exact lines from a real response). A finding is **verified** only when the level is 3 or higher and every quote of 8+ characters appears verbatim, after whitespace normalization, in the response body of recorded traffic linked to it. It is checked right after recording (steps are linked to traffic at that moment) and again whenever traffic is linked or unlinked on the finding page. Unreadable, binary or over-cap bodies give **not assessed**, never a pass or a fail. Bodies are decoded (gzip, brotli, deflate, zstd) and large bodies are read from their blob file. Verification never blocks recording, and the agent sees the verdict and the reason in its tool reply. The findings list shows a Verified or Not assessed chip; the finding page shows every state with its reason. Existing findings read as unverified. Upgraded databases get the two new fields added to the stored `report_finding` schema without touching operator edits.
- **Attach a conversation to a task**: a chat conversation (pentest, auto or any custom agent) can be attached to a task from the new task picker beside the model picker, or with `task_id` when it is created. Attached, the agent's findings, facts and assets are saved to that task with the same traffic linking and verification as a worker's, and the agent is told which task it serves. Before, the pentest agent could never save a finding, in AntiFox, upstream AntiFox or ARTEX. Conversations started by a task event are attached to that task automatically. `PATCH /api/conversations/{id}/task` attaches or detaches (`null`).
- **Planner findings are linked to traffic**: the planner can record findings, but they were never linked to the traffic that proves them. It now uses the same record-time linking and verification as workers.
- **Prompt and binding drift checks**: a test fails when a built-in prompt names a tool its agent cannot call, and startup logs a warning when an edited prompt and its bindings drift apart.
- **`docker-compose.yml` uses `pgvector/pgvector:pg16`** so the knowledge base's semantic search works on a fresh deployment. Switching an existing `postgres:16-alpine` volume needs one `REINDEX DATABASE`.

### Tests

- Database-backed tests now run under `ANTIFOX_PG_DSN` instead of passing by skipping; two stale tests were fixed. The full suite passes twice in a row with no skips.

## v2.1.1

- **Docker Build Fix (`non-free` Debian Bookworm package removal)**: Removed Debian `non-free` `nikto` from the container `apt-get install` list and preinstalled `libnet-ssleay-perl`, `libxml-writer-perl`, `libio-socket-ssl-perl`, and `libjson-perl` from Debian `main` so container builds succeed reliably on `python:3.12-slim-bookworm`.
- **Up-to-Date Upstream Nikto Self-Installer**: Switched `nikto`'s primary `InstallMethod` in `KaliToolCatalog` from `apt` (2.1.5) to `git_script` (`sullo/nikto` 2.6+), retaining `apt` as fallback, so `ensure_kali_tool` and `antifox-tool install nikto` deploy current upstream Nikto checks cleanly.

## v2.1.0

- **Dynamic Kali & ProjectDiscovery Tool Catalog**: Seeded 21 curated security CLI tools (`subfinder`, `dnsx`, `naabu`, `httpx`, `nmap`, `whatweb`, `wafw00f`, `katana`, `ffuf`, `gobuster`, `feroxbuster`, `arjun`, `nuclei`, `nikto`, `dalfox`, `testssl`, `sslscan`, `sqlmap`, `interactsh_client`, `jwt_tool`, `hydra`) into the PostgreSQL `tools` catalog (`kind="shell"`), with live `[installed]` / `[not installed]` status hints in `Bash` and pre-installed reconnaissance/verification packages in the Docker image.
- **On-Demand Latest-Release Self-Installer**: Added `ensure_kali_tool`, `/api/tools/kali`, `/api/tools/kali/install`, and the persistent `<dataDir>/bin/antifox-tool` CLI helper so missing or outdated tools install and upgrade directly from upstream GitHub Releases, Git repositories, `pip`, or `apt` rather than freezing old binaries inside releases.
- **Deterministic Scanner Ingestion & Asset Auto-Enrichment**: Added `scan_file` and `scan_jsonl` parameters to `insert_assets` for direct JSON/JSONL ingestion from `subfinder`, `dnsx`, `naabu`, `httpx`, `katana`, and `ffuf`; wired automatic DNS resolution (`ResolveDomain`) and HTTP site probing (`ProbeSite`) on asset upsert; added the `tech` compatibility alias and endpoint IP fallback.
- **Finding Accuracy, Traffic Correlation & Guard Fixes**: Preserved 4-level severity (`critical` no longer downgraded to `high`); fixed finding deduplication collisions across 12+ character vulnerability class names; automatically populated `finding_traffic` links when reproduction steps resolve to MITM packets; disambiguated `ResolveSteps` by host and expected status code; and extended `guard` attribution to interactive `shell_open`/`shell_send` sessions.
- **Global English Prompts, Startup Schema Sync & Updater**: Enforced English across agent prompts and settlement wrap-ups, replaced garbled tool/prompt strings, added UTF-8 safe truncation, refreshed built-in tool schemas on startup for existing databases, and updated `selfupdate.Repo` to `ChathurangaBW/antifox`.

## v2.0.4

- **Severity normalization restored**: `Serious`, `High risk`, `Low risk.`, and `Information` map to their correct canonical levels again. The English conversion left the alias keys capitalized while the input was lower-cased, so those aliases never matched and findings were downgraded.
- **Legacy title cleanup restored**: Chinese severity prefixes, full-width parentheses, and `U+3000` in target names are normalized again, so data created before the conversion still produces clean report titles.
- **Log levels fixed**: `failed`, `drop`, `reject`, `refused`, `retry`, and `disabled` raise the intended level; the lower-cased message was previously compared against capitalized keywords.
- **Ellipsis restored**: truncated output, the API-key hint, and traffic snippet markers use `U+2026` again instead of three ASCII dots.
- **Tests**: regression coverage added for severity aliases, title prefixes, name normalization, and log levels; the `db` test package compiles again after a pre-existing `CreateTask` signature mismatch.

## v2.0.3

- **Custom LLM session headers**: Each LLM configuration can set an HTTP header whose value is automatically the current session ID, supporting gateways such as OpenCode that use sessions for prompt caching or sticky routing.
- **Global outbound proxy**: Target traffic from every agent can use one outbound proxy (HTTP/HTTPS/SOCKS5), hiding the source IP or using a jump host; includes a usability test that displays the proxy egress IP.
- **Per-task egress selection**: When creating a task, choose whether it uses the global proxy or connects directly.
- **Confirmation enriches immediately**: Manually confirmed vulnerabilities are automatically enriched into structured data and are ready for delivery.
- **Report management**: New reports list only confirmed vulnerabilities by default (including status and CVSS); the body reuses enriched data.
- **Automatic evidence captions**: Evidence screenshot captions are generated from reproduction-step text and remain editable.
- **Vulnerability detail refinements**: Status dropdown, section headings, and one-click copy.
- **Traffic task filtering fix**: Task filtering now combines the task activity window, so repeated tests of the same target no longer mix traffic.
- Changed the system configuration page to a responsive two-column layout.

## v2.0.2

- **One-click update**: Check for and install new GitHub releases from the system configuration page (SHA256 verification, atomic replacement, and automatic rollback on failure) without affecting data.
- Release packages include the `start.sh` daemon script and a systemd template.

## v2.0.1

- Automatically link real traffic packets when reproduction steps are saved.

## v2.0.0

- Evidence-driven delivery: save reproduction steps in place, organize discovery pages by exploit-chain steps, link a real traffic packet to every step, and generate manually graded trilingual reports.
