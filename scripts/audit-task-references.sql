-- Read-only report. Query success does not mean zero orphan associations.
-- Run with psql -X --set=ON_ERROR_STOP=1 --file=scripts/audit-task-references.sql
\set ON_ERROR_STOP on
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '5s';

SELECT current_database() AS database_name,
       transaction_timestamp() AS audited_at,
       current_setting('transaction_read_only') AS read_only;

-- Count array entries, distinct assets and distinct missing ids separately.
SELECT count(*) AS orphan_reference_entries,
       count(DISTINCT a.id) AS affected_assets,
       count(DISTINCT ref.task_id) AS distinct_missing_task_ids,
       count(*) FILTER (WHERE ref.task_id IS NULL) AS null_reference_entries
FROM public.assets AS a
CROSS JOIN LATERAL unnest(a.task_ids) AS ref(task_id)
WHERE NOT EXISTS (SELECT 1 FROM public.tasks AS t WHERE t.id = ref.task_id);

SELECT ref.task_id AS missing_task_id,
       count(*) AS reference_entries,
       count(DISTINCT a.id) AS affected_assets
FROM public.assets AS a
CROSS JOIN LATERAL unnest(a.task_ids) AS ref(task_id)
WHERE NOT EXISTS (SELECT 1 FROM public.tasks AS t WHERE t.id = ref.task_id)
GROUP BY ref.task_id
ORDER BY ref.task_id NULLS LAST;

SELECT a.id AS asset_id, ref.task_id AS missing_task_id,
       ref.ordinality AS array_position,
       CASE WHEN ref.task_id IS NULL THEN 'null array element'
            ELSE 'missing task' END AS issue
FROM public.assets AS a
CROSS JOIN LATERAL unnest(a.task_ids) WITH ORDINALITY AS ref(task_id, ordinality)
WHERE NOT EXISTS (SELECT 1 FROM public.tasks AS t WHERE t.id = ref.task_id)
ORDER BY a.id, ref.ordinality;

COMMIT;
