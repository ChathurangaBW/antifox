# Observation, authorization, and localization fixes

- Preserve terminal status and account for in-flight planner/worker calls in
  read-only telemetry. Failed database reads return 503 instead of partial success.
- Reject nonterminal rerun requests with 409 before reactivation.
- Reject protected requests when RBAC access resolution fails or is unavailable.
- Return an English direct-access proxy response, retaining HTTP 400.
- Request variable Merriweather to restore the default Turbopack static build.

No schema migration or version bump is included. Consumers must handle 503 for
unavailable telemetry/authorization and 409 for nonterminal reruns. Additional
telemetry fields are optional in the frontend contract.

Verification: 61 isolated tests/subtests passed normally and under race detection;
the default UI build and TypeScript passed; the exact Turbopack artifact passed
isolated password-login/task-list browser checks and 12 PostgreSQL-backed API
checks under race detection. See `scripts/verification/statusfix/README.md`.

Deployment is outstanding. These results do not establish a fully patched live
release. Terminal rerun execution/concurrency, persisted-task migration/resumption,
and application pages outside the fixture are not covered.
