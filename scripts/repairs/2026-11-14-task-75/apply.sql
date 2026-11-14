-- Repair: six assets reference task 75, which never existed. They belong to
-- task 2. Evidence and procedure: README.md in this directory.
--
-- Run once, with a fresh backup taken first:
--   psql -X --set=ON_ERROR_STOP=1 --file=apply.sql
-- All or nothing: every row must be in exactly the recorded state, or the
-- transaction is rolled back and nothing changes. Running it a second time is
-- refused for the same reason. Works on v2.1.2 and later schemas; on v2.1.4+
-- the asset reference trigger checks the added id (2), which exists.
\set ON_ERROR_STOP on
BEGIN;
LOCK TABLE assets IN SHARE ROW EXCLUSIVE MODE;
DO $$
DECLARE
  expected CONSTANT jsonb := '{"132":[2,75],"134":[2,75],"135":[2,75],"138":[75],"139":[75],"141":[2,75]}';
  actual jsonb;
  n int;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM tasks WHERE id = 2) THEN
    RAISE EXCEPTION 'task 2 does not exist; nothing changed';
  END IF;
  IF EXISTS (SELECT 1 FROM tasks WHERE id = 75) THEN
    RAISE EXCEPTION 'task 75 exists now; the evidence no longer applies, nothing changed';
  END IF;
  SELECT jsonb_object_agg(id::text, to_jsonb(task_ids)) INTO actual
    FROM assets WHERE id IN (132,134,135,138,139,141);
  IF actual IS DISTINCT FROM expected THEN
    RAISE EXCEPTION 'assets are not in the recorded state (found %); nothing changed', actual;
  END IF;
  UPDATE assets SET task_ids = '{2}', updated_at = now()
   WHERE id IN (132,134,135,138,139,141);
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n <> 6 THEN
    RAISE EXCEPTION 'expected 6 rows, updated %; nothing changed', n;
  END IF;
END
$$;
COMMIT;
SELECT 'after' AS state, id, array_to_string(task_ids, ',') AS task_ids
  FROM assets WHERE id IN (132,134,135,138,139,141) ORDER BY id;
