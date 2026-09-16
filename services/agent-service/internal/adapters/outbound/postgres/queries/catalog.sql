-- name: LockCatalog :exec
SELECT pg_advisory_xact_lock(724391821);
-- name: CreateProvider :exec
INSERT INTO llm_providers (id,name,kind,base_url,api_key_ciphertext,api_key_hint,default_model,status,last_error,last_checked_at,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);
-- name: GetProvider :one
SELECT * FROM llm_providers WHERE id=$1;
-- name: GetProviderForUpdate :one
SELECT * FROM llm_providers WHERE id=$1 FOR UPDATE;
-- name: ListProviders :many
SELECT * FROM llm_providers WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: UpdateProvider :one
UPDATE llm_providers SET name=$2,kind=$3,base_url=$4,api_key_ciphertext=$5,api_key_hint=$6,default_model=$7,status=$8,last_error=$9,last_checked_at=$10,updated_at=$11,revision=revision+1
WHERE id=$1 RETURNING *;
-- name: DeleteProvider :execrows
DELETE FROM llm_providers WHERE id=$1;
-- name: RecordProviderCheck :execrows
UPDATE llm_providers SET status=$3,last_error=$4,last_checked_at=$5,updated_at=$5
WHERE id=$1 AND revision=$2;
-- name: CreateAgent :exec
INSERT INTO agents (id,name,description,provider_id,model,system_prompt,temperature,max_output_tokens,context_window_tokens,max_iterations,timeout_seconds,created_by,archived_at,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15);
-- name: GetAgent :one
SELECT * FROM agents WHERE id=$1 AND archived_at IS NULL;
-- name: GetAgentForUpdate :one
SELECT * FROM agents WHERE id=$1 AND archived_at IS NULL FOR UPDATE;
-- name: ListAgents :many
SELECT * FROM agents WHERE archived_at IS NULL AND (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: UpdateAgent :execrows
UPDATE agents SET name=$2,description=$3,provider_id=$4,model=$5,system_prompt=$6,temperature=$7,max_output_tokens=$8,context_window_tokens=$9,max_iterations=$10,timeout_seconds=$11,updated_at=$12
WHERE id=$1 AND archived_at IS NULL;
-- name: ArchiveAgent :execrows
UPDATE agents SET archived_at=$2,updated_at=$2 WHERE id=$1 AND archived_at IS NULL;
-- name: ListAgentsByProvider :many
SELECT id,name FROM agents WHERE provider_id=$1 AND archived_at IS NULL ORDER BY name,id;
