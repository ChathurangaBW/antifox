# Local patch verification and deployment status

This evidence accompanies the local patch commit, not a deployment. The most
recent live identity recorded during this work was `v2.1.6+statusfix`, containing
the earlier UI patch. No subsequent deployment is claimed.

## Review

Reviewed production diff: generic telemetry failures, read-only observation
projection, nonterminal rerun guard before reactivation, fail-closed RBAC lookup,
English proxy hook, optional API fields, and variable-font configuration.
No engine scheduling implementation, schema, or dependency manifest is changed.
Existing redundant conditionals in auth remain to minimize the tested diff.

This is AntiFox (Go/Next.js), not opencode-swarm. The latter's Bun/plugin invariant
commands and `.swarm` publication workflow do not apply. No repository AGENTS.md
or engineering-invariants.md was present. Released as v2.1.8.

## Recorded checks

- `isolated-normal.log`, `isolated-race.log`: 61 passing tests/subtests, zero skips.
- `integration.log`: actual UI password rejection/acceptance (401/200), task-list
  rendering and authenticated reload; no JavaScript errors or external browser
  requests. Twelve API checks against disposable PostgreSQL passed. Race enabled.
- `ui-build.log`: default `npm run build:static`, Turbopack compilation,
  TypeScript and 29 static pages passed.
- `vet.log`: successful `go vet ./server` (empty output).
- Identity manifests tie the source and exact embedded UI artifact to these runs.
- `cleanup.log`: disposable PostgreSQL container removed.

The committed build log has trailing whitespace removed; original output remains
in the local evidence directory referenced by the identity record.

Backend fixtures never start engine loops. Observation/control connections use a
SELECT-only database role; password login records audit entries through a separate
fixture connection. Task objects remain in-memory fixtures with preserved clocks.
These tests are not a whole-application security audit or concurrency proof.

## Outstanding deployment requirements

A live release must be built from the committed source, preserve/backup saved
state, and have its running binary/image identity compared with that artifact.
Its health/version, proxy response and authenticated UI require post-deployment
verification. Restarting the existing app can resume active task execution;
saved-state continuity is not proof of unchanged data or uninterrupted execution.
No deployment or restart was performed for this patch.

Earlier detailed chronology and local evidence paths are in
`scripts/engine-state-analysis.md`. Statements there about uncommitted changes
describe the historical verification points; this directory is committed with
the reviewed patch. No release version or final-completion claim is assigned.
