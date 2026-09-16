-- name: EnqueueRun :exec
INSERT INTO run_jobs (run_id,available_at,created_at) VALUES ($1,$2,$3);

-- name: ClaimRunJobs :many
UPDATE run_jobs SET locked_by=sqlc.arg('worker_id'),
  locked_until=now()+(sqlc.arg('lease_milliseconds')::bigint * interval '1 millisecond'),
  attempts=attempts+1
WHERE run_id IN (
  SELECT run_id FROM run_jobs
  WHERE available_at<=now() AND (locked_until IS NULL OR locked_until<now())
  ORDER BY available_at,run_id FOR UPDATE SKIP LOCKED LIMIT sqlc.arg('job_limit')
)
RETURNING *;

-- name: ExtendRunJob :execrows
UPDATE run_jobs SET locked_until=now()+(sqlc.arg('lease_milliseconds')::bigint * interval '1 millisecond')
WHERE run_id=sqlc.arg('run_id') AND locked_by=sqlc.arg('worker_id') AND locked_until>now();

-- name: CompleteRunJob :execrows
DELETE FROM run_jobs WHERE run_id=$1 AND locked_by=$2;

-- name: InsertWebhookDelivery :exec
INSERT INTO webhook_deliveries (
  id,run_id,api_key_id,url,event,payload,status,attempts,next_attempt_at,locked_until,
  last_status_code,last_error,delivered_at,created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);

-- name: ClaimDueWebhookDeliveries :many
UPDATE webhook_deliveries SET
  attempts=attempts+1,
  locked_until=now()+(sqlc.arg('lease_milliseconds')::bigint * interval '1 millisecond')
WHERE id IN (
  SELECT id FROM webhook_deliveries
  WHERE status='pending' AND next_attempt_at<=now() AND (locked_until IS NULL OR locked_until<now())
  ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT sqlc.arg('delivery_limit')
)
RETURNING *;

-- name: MarkWebhookDelivered :one
UPDATE webhook_deliveries SET status='delivered',locked_until=NULL,next_attempt_at=NULL,
  last_status_code=$2,last_error=NULL,delivered_at=$3
WHERE id=$1 AND status='pending'
RETURNING *;

-- name: MarkWebhookRetry :one
UPDATE webhook_deliveries SET locked_until=NULL,next_attempt_at=$4,last_status_code=$2,last_error=$3
WHERE id=$1 AND status='pending'
RETURNING *;

-- name: MarkWebhookFailed :one
UPDATE webhook_deliveries SET status='failed',locked_until=NULL,next_attempt_at=NULL,
  last_status_code=$2,last_error=$3
WHERE id=$1 AND status='pending'
RETURNING *;

-- name: GetWebhookDelivery :one
SELECT * FROM webhook_deliveries WHERE id=$1;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE run_id=sqlc.arg('run_id')
  AND (sqlc.narg('before_time')::timestamptz IS NULL OR (created_at,id)<(sqlc.narg('before_time'),sqlc.narg('before_id')::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg('delivery_limit');

-- name: ResetWebhookDelivery :one
UPDATE webhook_deliveries SET status='pending',attempts=0,next_attempt_at=$2,locked_until=NULL,
  last_status_code=NULL,last_error=NULL,delivered_at=NULL
WHERE id=$1 AND status='failed'
RETURNING *;

-- name: DeleteExpiredIdempotencyKey :exec
DELETE FROM idempotency_keys
WHERE principal_kind=$1 AND principal_id=$2 AND key=$3 AND expires_at<=$4;

-- name: InsertIdempotencyKey :one
INSERT INTO idempotency_keys (
  principal_kind,principal_id,key,request_hash,run_id,response_status,created_at,expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys WHERE principal_kind=$1 AND principal_id=$2 AND key=$3;

-- name: DeleteExpiredIdempotency :exec
DELETE FROM idempotency_keys WHERE expires_at<=$1;
