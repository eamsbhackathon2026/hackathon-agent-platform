-- name: CountUsers :one
SELECT count(*) FROM users;
-- name: LockIdentity :exec
SELECT pg_advisory_xact_lock(724391820);
-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, name, role, status, must_change_password, last_login_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9);
-- name: GetUser :one
SELECT * FROM users WHERE id=$1;
-- name: GetUserForUpdate :one
SELECT * FROM users WHERE id=$1 FOR UPDATE;
-- name: FindUserByEmail :one
SELECT * FROM users WHERE lower(email)=lower($1);
-- name: ListUsers :many
SELECT * FROM users WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: UpdateMember :one
UPDATE users SET name=coalesce(sqlc.narg(name)::text,name),role=coalesce(sqlc.narg(role)::text,role),status=coalesce(sqlc.narg(status)::text,status)
WHERE id=$1 RETURNING *;
-- name: SetPassword :execrows
UPDATE users SET password_hash=$2,must_change_password=false WHERE id=$1;
-- name: TouchLastLogin :execrows
UPDATE users SET last_login_at=$2 WHERE id=$1;
-- name: CountActiveOwners :one
SELECT count(*) FROM users WHERE role='owner' AND status='active';
-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id,user_id,family_id,token_hash,expires_at,revoked_at,replaced_by,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);
-- name: FindRefreshToken :one
SELECT * FROM refresh_tokens WHERE token_hash=$1;
-- name: GetRefreshTokenForUpdate :one
SELECT * FROM refresh_tokens WHERE id=$1 FOR UPDATE;
-- name: RotateRefreshToken :execrows
UPDATE refresh_tokens SET replaced_by=$2,revoked_at=$3 WHERE id=$1 AND revoked_at IS NULL;
-- name: RevokeRefreshFamily :execrows
UPDATE refresh_tokens SET revoked_at=coalesce(revoked_at,$2) WHERE family_id=$1;
-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at=coalesce(revoked_at,sqlc.arg(revoked_at)) WHERE user_id=sqlc.arg(user_id)
AND (sqlc.arg(except_family)::uuid='00000000-0000-0000-0000-000000000000'::uuid OR family_id<>sqlc.arg(except_family));
-- name: CreateAPIKey :exec
INSERT INTO api_keys (id,name,prefix,key_hash,scopes,webhook_secret_ciphertext,created_by,last_used_at,revoked_at,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id=$1;
-- name: FindAPIKeyByPrefix :one
SELECT * FROM api_keys WHERE prefix=$1;
-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: RevokeAPIKey :execrows
UPDATE api_keys SET revoked_at=coalesce(revoked_at,$2) WHERE id=$1;
-- name: TouchAPIKey :execrows
UPDATE api_keys SET last_used_at=$2
WHERE id=$1 AND (last_used_at IS NULL OR last_used_at < $2::timestamptz - interval '1 minute');
