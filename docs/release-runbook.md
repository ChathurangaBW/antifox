# Release validation and recovery runbook

Keep source, test, publication and deployment evidence separate. A successful
release workflow does not establish that a running instance has been upgraded.
This document was absent from the published v2.1.3 tag and first ships in
v2.1.4. CI checks that every repository-relative link in `CHANGELOG.md`,
`README*.md` and `docs/` resolves in a clean checkout
(`scripts/check-doc-links.py`), so a link to an uncommitted file fails the gate.

## 1. Record source and documentation identity

Record the full commit, tag (if any), local diff and untracked deliverables. A
modified checkout must not be described as the exact tagged source. Preserve
existing work and retain evidence outside the repository, without credentials.

The release title comes from the top version heading in `CHANGELOG.md`. Before
claiming this runbook shipped, confirm that both `docs/release-runbook.md` and
`scripts/audit-task-references.sql` exist in the intended commit/source archive.
Being unignored or present locally is not sufficient.

## 2. Preserve test exit status and reject skips

Use a disposable PostgreSQL 16 + pgvector database, never a production DSN.
Wait for readiness and arrange cleanup of the disposable instance. The example
below assumes that instance is already ready and `ANTIFOX_PG_DSN` refers to it.
Run from the repository root using Bash, Go, a C compiler for the race detector,
and Python 3. `EVIDENCE_DIR` must be an absolute directory outside the checkout.

```bash
set -euo pipefail
: "${ANTIFOX_PG_DSN:?Set the disposable test database DSN}"
: "${EVIDENCE_DIR:?Set an absolute evidence directory outside the checkout}"
mkdir -p "$EVIDENCE_DIR"
go vet ./... > "$EVIDENCE_DIR/vet.log" 2>&1
for suite in normal race; do
  flags=()
  if [ "$suite" = race ]; then flags=(-race); fi
  set +e
  CGO_ENABLED=1 go test -count=1 -p 1 -json -timeout=240s "${flags[@]}" ./... \
    > "$EVIDENCE_DIR/$suite.jsonl" 2> "$EVIDENCE_DIR/$suite.stderr"
  status=$?
  set -e
  printf '%s exit=%s\n' "$suite" "$status"
  if [ "$status" -ne 0 ]; then exit "$status"; fi
  python3 - "$EVIDENCE_DIR/$suite.jsonl" <<'PY'
import json, pathlib, sys
events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text().splitlines()]
no_tests = {e.get("Package") for e in events if e.get("Action") == "output"
            and not e.get("Test") and e.get("Output", "").rstrip().endswith("[no test files]")}
bad = [e for e in events if e.get("Action") == "fail" or
       (e.get("Action") == "skip" and (e.get("Test") or e.get("Package") not in no_tests))]
passed = sum(e.get("Action") == "pass" and bool(e.get("Test")) for e in events)
for event in bad:
    print(event.get("Action"), event.get("Package"), event.get("Test", ""))
print("passed tests (including subtests):", passed)
print("packages with no test files:", len(no_tests))
sys.exit(1 if bad or not passed else 0)
PY
done
```

The test process exit status is authoritative; filtering output is not a gate.
Malformed/empty JSON, skipped individual tests and unexplained package skips
also fail this example. Go's package-level skips explicitly marked `[no test files]`
are counted separately, not mistaken for skipped tests. Retain logs even on
failure. Record frontend checks separately: static build/type checking
does not replace lint or browser validation. Prior passing results apply only
to their recorded source identity and environment.

## 3. Backup and actual restore verification

Record the PostgreSQL version, extensions, collation, source database identity,
backup timestamp and checksum. Use restricted storage outside the checkout.
This Bash example creates a database backup; it does not verify restoration.
Set the container name, version label and an absolute backup root for the
instance being backed up rather than copying machine-specific paths.

```bash
set -euo pipefail
umask 077
: "${DB_CONTAINER:?Set the existing PostgreSQL container name}"
: "${BACKUP_ROOT:?Set an absolute restricted backup directory}"
: "${VERSION:?Set a version label such as v2.1.3}"
[[ "$BACKUP_ROOT" = /* ]] || { echo 'BACKUP_ROOT must be absolute' >&2; exit 1; }
[[ "$VERSION" =~ ^[A-Za-z0-9._-]+$ ]] || { echo 'Invalid version label' >&2; exit 1; }
mkdir -p "$BACKUP_ROOT"
backup_dir=$(mktemp -d "$BACKUP_ROOT/pre-$VERSION-XXXXXXXX")
backup="$backup_dir/database.dump"
docker exec "$DB_CONTAINER" sh -eu -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup.partial"
test -s "$backup.partial"
mv "$backup.partial" "$backup"
sha256sum "$backup" > "$backup.sha256"
docker exec -i "$DB_CONTAINER" pg_restore --list < "$backup" > "$backup.toc"
```

`pg_restore --list` only reads the table of contents. A nonempty dump, checksum
and listing do **not** prove a successful restore. Separately capture required
roles, configuration and external files; `pg_dump` alone is not a full-system
backup. Protect those records because they can contain credentials.

Restore into a newly created disposable database with compatible roles and
extensions. Use `pg_restore --exit-on-error`, retain its exit status and log,
and inspect restored schema, representative rows, relationships and counts.
Compare against evidence from the backup's consistent snapshot or a quiesced
source; live counts may change after a dump. Run the audit below on the copy.
An upgrade rehearsal must preserve operator-edited tool definitions and record
all migration outcomes. Keep the copy isolated from real integrations and work.

## 4. Read-only database audit

Run from the repository root. `PGSERVICE` should name a configured libpq service
with read-only credentials; do not put its password in the command or report.

```bash
psql -X --set=ON_ERROR_STOP=1 --file=scripts/audit-task-references.sql
```

The script uses a repeatable-read, read-only transaction with time limits. It
reports missing task ids (including invalid negative/zero ids), NULL array
elements, affected assets and individual references. Empty arrays are unlinked
assets, not errors. Its exit status indicates query success, **not** absence of
orphans; inspect the totals. It does not infer ownership or modify records.

Before repair, establish whether each id denotes historical provenance or an
active association. A prepared repair records its evidence, guarded apply and
rollback scripts and rehearsal next to each other; see
`scripts/repairs/2026-11-14-task-75/` for the format. Record the original values, evidence for the proposed
change, backup identity and a reviewed rollback plan. Do not guess a replacement
task or bundle historical repairs into an upgrade. From v2.1.4 the database
refuses a new reference to a missing task (`trg_assets_task_ids`) and removes a
deleted task's id from every asset (`trg_tasks_remove_asset_refs`), whichever
application version deletes it; references written before that remain until
repaired.

Also inspect extension and collation metadata:

```sql
BEGIN READ ONLY;
SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';
SELECT datname, datcollversion, pg_database_collation_actual_version(oid)
FROM pg_database;
COMMIT;
```

A collation mismatch needs a planned index-maintenance review. Version metadata
alone is not sufficient when changing libc providers (for example musl to
glibc). Determine affected indexes, rebuild them in a maintenance procedure and
only then refresh recorded collation metadata as applicable. None of those
writes are part of this read-only audit.

## 5. Publication versus running-instance evidence

Record publication status, workflow commit, artifact checksums and supported
platforms separately from the running instance. The v2.1.3 container publication
supports linux/amd64; binary archive platforms differ.

For an existing deployment, capture the running container image ID/digest and
revision label, `/api/health` version, readiness and startup/migration results.
Verify both activity outcome columns and the list-assets migration completion
flag. A tag label alone is insufficient; missing metadata remains unverified.
Review any migration failure or retained operator customization explicitly.

Record browser checks for login, normal completion, replacement by a new
message, operator stop, pause and genuine failures. Check result persistence as
well as visual styling. From v2.1.4 a cancelled result also stores a stable
`cancel_cause` code (`replaced_by_message`, `operator_stop`, `task_paused`,
`work_killed`, `task_completed`, `task_deadline`, `shutdown`, `run_deadline`,
`unknown`), and the transcript exposes `data-outcome`/`data-cancel-cause` on
each result for automated checks. Historical outcomes and causes may be NULL,
and an abruptly terminated process cannot guarantee a final result row.

If checking secret-environment handling, report variable names or presence
counts only, never values. The nonblocking handoff can fall back to the initial
environment on failure; startup availability is not proof of initial-environment
secrecy. Avoid printing a raw process environment into evidence logs.

## 6. Recovery evidence, not an assumed rollback guarantee

Additive schema changes do **not** prove that the previous application version
can safely use upgraded data, tool definitions or settings. Validate backward
compatibility on a disposable upgraded copy before claiming image-only rollback
is supported. Record prior configuration and image identity, including whether
an override file existed; do not assume a particular override file is present.

v2.1.4 rehearsal (2026-11-14, production backup restored to a disposable
database on an internal Docker network with no egress, tasks paused): the new
build upgraded the copy cleanly; the v2.1.2 image then started against the
upgraded copy, served every read endpoint checked, created an asset, deleted a
task and stopped cleanly; the new build re-upgraded it. The database triggers
kept enforcing references under v2.1.2 (a missing task id was refused, and the
deleted task's ids were removed), and the operator's `list_assets` definition
was unchanged throughout. This supports image-only rollback from v2.1.4 to
v2.1.2 for the paths checked; it does not cover agent runs, which were paused.

If restoring a pre-upgrade database is necessary, account for writes since the
backup and the associated data-loss window. Coordinate application/database/
external-file recovery and verify it on a disposable environment first. A
backup listing or successful publication does not close this requirement.
