# Repair: assets referencing task 75 (which never existed)

Status: **prepared and rehearsed, not applied.** Applying it changes production
data and is the operator's decision.

## What is wrong

Six assets list task `75` in `task_ids`. No task 75 has ever existed: the task
id sequence on the production database had reached 3 when this was examined.

| asset | type        | value                   | task_ids |
|------:|-------------|-------------------------|----------|
| 132   | ip          | 10.129.205.13           | 2,75     |
| 134   | root_domain | bedside.htb             | 2,75     |
| 135   | service     | http://bedside.htb/     | 2,75     |
| 138   | service     | http://10.129.205.13/   | 75       |
| 139   | root_domain | 10.129.205.13           | 75       |
| 141   | service     | ssh 10.129.205.13:22    | 2,75     |

Assets 138 and 139 reference only the missing task, so they are linked to no task
in the UI.

## Where the id came from

All six were written by one tool call, recorded in task 2's activity:

- Activity **5123**: worker `work#3` of task **2** ("Bedside", exploration 2),
  `insert_assets`, 2026-11-13 07:04:52.914 UTC. Its stored input (complete,
  744 characters) has five assets (the IP, `bedside.htb`, `http://bedside.htb/`,
  `http://10.129.205.13/`, SSH on port 22) and `"task_id": 75`.
- Activity **5124**: the result of that call. It returns asset ids 132, 134,
  135, 138 and 141, and is the only activity on any task in that time window.
- Asset **139** is the root domain `10.129.205.13` that `UpsertHTTPService`
  (`db/assets.go`) creates, with the same task id, for the service
  `http://10.129.205.13/`. It was created at 07:04:52.934, between 138 (.932)
  and 141 (.941), inside the call (5123 at .914, result 5124 at .946).
- No other activity on any task names task id 75.

The model supplied a task id that did not exist. In the version running then,
a caller-supplied `task_id` was written as given. v2.1.3 rejects a nonexistent
id at the tool and API, and v2.1.4 enforces it in the database.

**Conclusion:** the assets were discovered by task 2. The repair replaces 75 with
2 on exactly these six rows; the four that already list 2 end up with `{2}`.

## Files

- `apply.sql`: the repair. All six rows must be in exactly the state above or
  nothing changes (so a second run is refused). It checks that task 2 exists and
  task 75 still does not.
- `rollback.sql`: restores the exact original arrays, only from the repaired
  state (if later work changed the rows, it refuses rather than lose that
  change). On v2.1.4+ the asset reference trigger would refuse to re-add the
  missing task 75, so the rollback disables it for its own transaction only;
  `ALTER TABLE` is transactional, so an error leaves it enabled.

Both run under `ON_ERROR_STOP` in one transaction and lock `assets` against
concurrent writes while they check and update.

## Procedure

1. Take a fresh backup and record its checksum (see
   `docs/release-runbook.md`, section 3).
2. Run the read-only audit and confirm it reports exactly these six references:
   `psql -X --set=ON_ERROR_STOP=1 --file=scripts/audit-task-references.sql`
3. Apply: `psql -X --set=ON_ERROR_STOP=1 --file=scripts/repairs/2026-11-14-task-75/apply.sql`
   The final query lists every row as `2`.
4. Re-run the audit: zero orphan references expected.

To undo: `psql -X --set=ON_ERROR_STOP=1 --file=scripts/repairs/2026-11-14-task-75/rollback.sql`

With docker compose, pipe the file in:
`docker compose exec -T postgres psql -U antifox -d antifox -X --set=ON_ERROR_STOP=1 < scripts/repairs/2026-11-14-task-75/apply.sql`

## Rehearsal

Done 2026-11-14 on two disposable copies restored from the production backup
`pre-v2.1.3-20261114-191132.dump`: one left at the v2.1.2 schema, one upgraded
with this release's `db/schema.sql`. On each: apply succeeded; a second apply
was refused with nothing changed; rollback restored the original arrays exactly;
a second rollback was refused; apply after rollback succeeded. On the upgraded
copy the reference trigger was enabled after the rollback and still refused a
nonexistent task id.

The repair is independent of the upgrade: it can be applied before or after
deploying v2.1.4.
