-- name: OverviewTotals :one
SELECT COUNT(*) AS requests,
       COUNT(*) FILTER (WHERE status='succeeded') AS succeeded,
       COUNT(*) FILTER (WHERE status='failed') AS failed,
       COUNT(*) FILTER (WHERE status='cancelled') AS cancelled,
       COUNT(DISTINCT session_id) AS sessions,
       COUNT(*) FILTER (WHERE started_at IS NOT NULL AND finished_at IS NOT NULL) AS finished,
       COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at-started_at))*1000),0)::double precision AS avg_duration_ms,
       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (
         ORDER BY EXTRACT(EPOCH FROM (finished_at-started_at))*1000
       ),0)::double precision AS p95_duration_ms,
       COALESCE(SUM(COALESCE(input_tokens,0)+COALESCE(output_tokens,0)),0)::bigint AS processing_units
FROM runs
WHERE created_at >= sqlc.arg('from_time') AND created_at < sqlc.arg('to_time');

-- name: OverviewStepLimitHits :one
SELECT COUNT(*) AS hits
FROM runs JOIN agents ON agents.id=runs.agent_id
WHERE runs.created_at >= sqlc.arg('from_time') AND runs.created_at < sqlc.arg('to_time')
  AND runs.iterations >= agents.max_iterations;

-- name: OverviewDaily :many
-- Every day in the range gets a row, including days without any request, so the
-- trend chart cannot silently drop quiet days and distort the shape.
SELECT day::date AS day,
       COALESCE(counted.playground,0)::bigint AS playground,
       COALESCE(counted.api,0)::bigint AS api,
       COALESCE(counted.failed,0)::bigint AS failed
FROM generate_series(
       date_trunc('day',sqlc.arg('from_time')::timestamptz AT TIME ZONE sqlc.arg('time_zone')::text),
       date_trunc('day',(sqlc.arg('to_time')::timestamptz - interval '1 microsecond') AT TIME ZONE sqlc.arg('time_zone')::text),
       interval '1 day'
     ) AS day
LEFT JOIN (
  SELECT date_trunc('day',created_at AT TIME ZONE sqlc.arg('time_zone')::text) AS bucket,
         COUNT(*) FILTER (WHERE source='playground') AS playground,
         COUNT(*) FILTER (WHERE source='api') AS api,
         COUNT(*) FILTER (WHERE status='failed') AS failed
  FROM runs
  WHERE created_at >= sqlc.arg('from_time') AND created_at < sqlc.arg('to_time')
  GROUP BY bucket
) AS counted ON counted.bucket=day
ORDER BY day;

-- name: OverviewTopAgents :many
SELECT runs.agent_id,
       agents.name AS agent_name,
       COUNT(*) AS requests,
       COUNT(*) FILTER (WHERE runs.status='succeeded') AS succeeded,
       COUNT(*) FILTER (WHERE runs.status='failed') AS failed,
       COUNT(*) FILTER (WHERE runs.started_at IS NOT NULL AND runs.finished_at IS NOT NULL) AS finished,
       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (
         ORDER BY EXTRACT(EPOCH FROM (runs.finished_at-runs.started_at))*1000
       ),0)::double precision AS p95_duration_ms,
       COALESCE(SUM(COALESCE(runs.input_tokens,0)+COALESCE(runs.output_tokens,0)),0)::bigint AS processing_units
FROM runs JOIN agents ON agents.id=runs.agent_id
WHERE runs.created_at >= sqlc.arg('from_time') AND runs.created_at < sqlc.arg('to_time')
GROUP BY runs.agent_id,agents.name
ORDER BY requests DESC,agents.name ASC
LIMIT sqlc.arg('row_limit');

-- name: OverviewTopErrors :many
SELECT error_code::text AS error_code,
       COUNT(*) AS occurrences,
       MAX(created_at)::timestamptz AS last_seen_at,
       ((ARRAY_AGG(id ORDER BY created_at DESC))[1])::uuid AS sample_run_id
FROM runs
WHERE created_at >= sqlc.arg('from_time') AND created_at < sqlc.arg('to_time')
  AND error_code IS NOT NULL
GROUP BY error_code
ORDER BY occurrences DESC,error_code ASC
LIMIT sqlc.arg('row_limit');

-- name: OverviewTopTools :many
-- Scoped by the run's created_at, like every other figure on the report, so
-- "this period" means the same thing in each block. Filtering on the span's own
-- start time would pull in calls from runs the rest of the report excludes.
SELECT spans.tool_name::text AS tool_name,
       COUNT(*) AS calls,
       COUNT(*) FILTER (WHERE spans.status='error') AS errors,
       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY spans.duration_ms),0)::double precision AS p95_duration_ms
FROM spans JOIN runs ON runs.id=spans.run_id
WHERE spans.kind='tool_call' AND spans.tool_name IS NOT NULL
  AND runs.created_at >= sqlc.arg('from_time') AND runs.created_at < sqlc.arg('to_time')
GROUP BY spans.tool_name
ORDER BY calls DESC,tool_name ASC
LIMIT sqlc.arg('row_limit');
