-- name: CreateAPIConnection :exec
INSERT INTO api_connections (id,slug,display_name,base_url,public_headers,secret_headers_ciphertext,secret_header_names,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9);

-- name: GetAPIConnection :one
SELECT * FROM api_connections WHERE id=$1;

-- name: GetAPIConnectionForUpdate :one
SELECT * FROM api_connections WHERE id=$1 FOR UPDATE;

-- name: ListAPIConnections :many
SELECT * FROM api_connections
WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;

-- name: ListAllAPIConnections :many
SELECT * FROM api_connections ORDER BY slug;

-- name: UpdateAPIConnection :execrows
UPDATE api_connections
SET slug=$2,display_name=$3,base_url=$4,public_headers=$5,secret_headers_ciphertext=$6,secret_header_names=$7,updated_at=$8
WHERE id=$1;

-- name: DeleteAPIConnection :execrows
DELETE FROM api_connections WHERE id=$1;

-- name: CountToolsByAPIConnection :one
SELECT count(*) FROM tools WHERE connection_id=$1;
