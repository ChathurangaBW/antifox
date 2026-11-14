-- Rollback of apply.sql: puts the six assets back exactly as they were
-- (task 75 restored). Evidence and procedure: README.md in this directory.
--
--   psql -X --set=ON_ERROR_STOP=1 --file=rollback.sql
-- All or nothing, and only from the state apply.sql leaves (every row {2}). If
-- later work changed these rows, it refuses rather than discard that change.
-- Task 75 does not exist, so on v2.1.4+ the asset reference trigger would
-- refuse to add it back: the trigger is disabled for this transaction only and
-- re-enabled before COMMIT (ALTER TABLE is transactional; an error rolls both
-- back). Needs the table owner, which the application's database user is.
\set ON_ERROR_STOP on
BEGIN;
LOCK TABLE assets IN SHARE ROW EXCLUSIVE MODE;
DO $$
DECLARE
  repaired CONSTANT jsonb := '{"132":[2],"134":[2],"135":[2],"138":[2],"139":[2],"141":[2]}';
  actual jsonb;
  has_trigger boolean;
  n int;
BEGIN
  SELECT jsonb_object_agg(id::text, to_jsonb(task_ids)) INTO actual
    FROM assets WHERE id IN (132,134,135,138,139,141);
  IF actual IS DISTINCT FROM repaired THEN
    RAISE EXCEPTION 'assets are not in the repaired state (found %); nothing changed', actual;
  END IF;
  SELECT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_assets_task_ids'
                   AND tgrelid = 'assets'::regclass) INTO has_trigger;
  IF has_trigger THEN
    ALTER TABLE assets DISABLE TRIGGER trg_assets_task_ids;
  END IF;
  UPDATE assets SET task_ids = CASE WHEN id IN (138,139) THEN '{75}'::bigint[] ELSE '{2,75}'::bigint[] END,
                    updated_at = now()
   WHERE id IN (132,134,135,138,139,141);
  GET DIAGNOSTICS n = ROW_COUNT;
  IF has_trigger THEN
    ALTER TABLE assets ENABLE TRIGGER trg_assets_task_ids;
  END IF;
  IF n <> 6 THEN
    RAISE EXCEPTION 'expected 6 rows, updated %; nothing changed', n;
  END IF;
END
$$;
COMMIT;
SELECT 'after rollback' AS state, id, array_to_string(task_ids, ',') AS task_ids
  FROM assets WHERE id IN (132,134,135,138,139,141) ORDER BY id;
