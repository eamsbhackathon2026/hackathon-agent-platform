-- name: CreateSession :execrows
INSERT INTO sessions (
  id,agent_id,source,created_by_user_id,created_by_api_key_id,external_key,title,created_at,updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (created_by_api_key_id,external_key) WHERE deleted_at IS NULL DO NOTHING;

-- name: GetSession :one
SELECT * FROM sessions WHERE id=$1 AND deleted_at IS NULL;

-- name: GetSessionForUpdate :one
SELECT * FROM sessions WHERE id=$1 AND deleted_at IS NULL FOR UPDATE;

-- name: GetSessionByExternalKey :one
SELECT * FROM sessions
WHERE created_by_api_key_id=$1 AND external_key=$2 AND deleted_at IS NULL
FOR UPDATE;

-- name: ListSessions :many
SELECT * FROM sessions
WHERE deleted_at IS NULL
  AND (sqlc.narg('agent_id')::uuid IS NULL OR agent_id=sqlc.narg('agent_id'))
  AND (sqlc.narg('source')::text IS NULL OR source=sqlc.narg('source'))
  AND (sqlc.narg('owner_user_id')::uuid IS NULL OR created_by_user_id=sqlc.narg('owner_user_id'))
  AND (sqlc.narg('owner_api_key_id')::uuid IS NULL OR created_by_api_key_id=sqlc.narg('owner_api_key_id'))
  AND (sqlc.narg('before_time')::timestamptz IS NULL OR (created_at,id) < (sqlc.narg('before_time'),sqlc.narg('before_id')::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg('limit');

-- name: ListSessionsByUpdatedAt :many
SELECT * FROM sessions
WHERE deleted_at IS NULL
  AND (sqlc.narg('agent_id')::uuid IS NULL OR agent_id=sqlc.narg('agent_id'))
  AND (sqlc.narg('source')::text IS NULL OR source=sqlc.narg('source'))
  AND (sqlc.narg('owner_user_id')::uuid IS NULL OR created_by_user_id=sqlc.narg('owner_user_id'))
  AND (sqlc.narg('owner_api_key_id')::uuid IS NULL OR created_by_api_key_id=sqlc.narg('owner_api_key_id'))
  AND (sqlc.narg('before_updated_at')::timestamptz IS NULL OR (updated_at,id) < (sqlc.narg('before_updated_at'),sqlc.narg('before_id')::uuid))
ORDER BY updated_at DESC,id DESC LIMIT sqlc.arg('limit');

-- name: SessionHasActiveRuns :one
SELECT EXISTS(SELECT 1 FROM runs WHERE session_id=$1 AND status IN ('queued','running'));

-- name: DeleteSession :execrows
UPDATE sessions SET deleted_at=$2,updated_at=$2 WHERE id=$1 AND deleted_at IS NULL;

-- name: UpdatePromptTokenCalibration :execrows
UPDATE sessions
SET last_prompt_estimated_tokens=sqlc.arg('estimated_tokens'),
    last_prompt_tokens=sqlc.arg('actual_tokens'),
    updated_at=GREATEST(updated_at,sqlc.arg('updated_at'))
WHERE id=sqlc.arg('session_id') AND deleted_at IS NULL;

-- name: AppendMessage :one
WITH next AS (
  UPDATE sessions
  SET next_message_seq=next_message_seq+1,updated_at=GREATEST(updated_at,sqlc.arg('created_at'))
  WHERE id=sqlc.arg('session_id') AND deleted_at IS NULL
  RETURNING next_message_seq-1 AS seq
)
INSERT INTO messages (
  id,session_id,run_id,seq,role,content,tool_calls,tool_call_id,tool_name,provider_meta,is_error,created_at
)
SELECT sqlc.arg('id'),sqlc.arg('session_id'),sqlc.narg('run_id'),next.seq,sqlc.arg('role'),sqlc.arg('content'),
       sqlc.arg('tool_calls'),sqlc.narg('tool_call_id'),sqlc.narg('tool_name'),sqlc.narg('provider_meta'),
       sqlc.arg('is_error'),sqlc.arg('created_at')
FROM next
RETURNING *;

-- name: ListRecentMessages :many
SELECT * FROM (
  SELECT * FROM messages WHERE session_id=$1 ORDER BY seq DESC,id DESC LIMIT $2
) recent ORDER BY seq,id;

-- name: ListMessages :many
SELECT * FROM messages
WHERE session_id=sqlc.arg('session_id')
  AND (sqlc.narg('after_seq')::bigint IS NULL OR (seq,id) > (sqlc.narg('after_seq'),sqlc.narg('after_id')::uuid))
ORDER BY seq,id LIMIT sqlc.arg('limit');

-- name: ListRunMessages :many
SELECT * FROM messages
WHERE session_id=sqlc.arg('session_id')
  AND run_id=sqlc.arg('run_id')
  AND (sqlc.narg('after_seq')::bigint IS NULL OR (seq,id) > (sqlc.narg('after_seq'),sqlc.narg('after_id')::uuid))
ORDER BY seq,id LIMIT sqlc.arg('limit');

-- name: ListMessagesAfterSeq :many
SELECT * FROM messages
WHERE session_id=sqlc.arg('session_id') AND seq > sqlc.arg('after_seq')
ORDER BY seq,id LIMIT sqlc.arg('limit');

-- name: GetLatestContextSnapshot :one
SELECT * FROM session_context_snapshots
WHERE session_id=$1
ORDER BY version DESC
LIMIT 1;

-- name: CreateContextSnapshotCAS :execrows
INSERT INTO session_context_snapshots (
  id,session_id,source_run_id,version,covered_through_seq,summary,input_tokens,output_tokens,created_at
)
SELECT sqlc.arg('id'),sqlc.arg('session_id'),sqlc.narg('source_run_id'),sqlc.arg('version'),
       sqlc.arg('covered_through_seq'),sqlc.arg('summary'),sqlc.narg('input_tokens'),
       sqlc.narg('output_tokens'),sqlc.arg('created_at')
WHERE COALESCE((
  SELECT MAX(snapshot.version) FROM session_context_snapshots AS snapshot
  WHERE snapshot.session_id=sqlc.arg('session_id')
),0)=sqlc.arg('expected_version')
  AND COALESCE((
    SELECT MAX(snapshot.covered_through_seq) FROM session_context_snapshots AS snapshot
    WHERE snapshot.session_id=sqlc.arg('session_id')
  ),0) < sqlc.arg('covered_through_seq')
ON CONFLICT DO NOTHING;

-- name: CreateRun :exec
INSERT INTO runs (
  id,agent_id,session_id,mode,status,source,triggered_by_user_id,triggered_by_api_key_id,input,output,
  error_code,error_message,iterations,input_tokens,output_tokens,metadata,cancel_requested_at,queued_at,
  started_at,finished_at,created_at,webhook_url
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22);

-- name: GetRun :one
SELECT * FROM runs WHERE id=$1;

-- name: StartQueuedRun :one
UPDATE runs SET status='running',started_at=$2
WHERE id=$1 AND status='queued'
RETURNING *;

-- name: ListRuns :many
SELECT * FROM runs
WHERE (sqlc.narg('agent_id')::uuid IS NULL OR agent_id=sqlc.narg('agent_id'))
  AND (sqlc.narg('status')::text IS NULL OR status=sqlc.narg('status'))
  AND (sqlc.narg('source')::text IS NULL OR source=sqlc.narg('source'))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR created_at < sqlc.narg('to_time'))
  AND (sqlc.narg('owner_user_id')::uuid IS NULL OR triggered_by_user_id=sqlc.narg('owner_user_id'))
  AND (sqlc.narg('owner_api_key_id')::uuid IS NULL OR triggered_by_api_key_id=sqlc.narg('owner_api_key_id'))
  AND (sqlc.narg('before_time')::timestamptz IS NULL OR (created_at,id) < (sqlc.narg('before_time'),sqlc.narg('before_id')::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg('limit');

-- name: RequestRunCancel :one
WITH updated AS (
  UPDATE runs AS target SET cancel_requested_at=COALESCE(target.cancel_requested_at,$2)
  WHERE target.id=$1 AND target.status IN ('queued','running')
  RETURNING target.*
)
SELECT * FROM updated
UNION ALL
SELECT runs.* FROM runs WHERE runs.id=$1 AND NOT EXISTS (SELECT 1 FROM updated)
LIMIT 1;

-- name: RunCancelRequested :one
SELECT (cancel_requested_at IS NOT NULL)::boolean AS requested FROM runs WHERE id=$1;

-- name: FinishRun :one
UPDATE runs SET status=$2,output=$3,error_code=$4,error_message=$5,iterations=$6,input_tokens=$7,
  output_tokens=$8,finished_at=$9
WHERE id=$1 AND status IN ('queued','running')
RETURNING *;

-- name: CreateSpan :exec
INSERT INTO spans (
  id,run_id,parent_span_id,kind,name,status,model,tool_name,input_tokens,output_tokens,started_at,ended_at,
  duration_ms,attributes,error_message
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15);

-- name: ListSpans :many
SELECT * FROM spans
WHERE run_id=sqlc.arg('run_id')
  AND (sqlc.narg('after_time')::timestamptz IS NULL OR (started_at,id) > (sqlc.narg('after_time'),sqlc.narg('after_id')::uuid))
ORDER BY started_at,id LIMIT sqlc.arg('limit');
