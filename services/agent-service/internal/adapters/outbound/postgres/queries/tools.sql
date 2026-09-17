-- name: LockTooling :exec
SELECT pg_advisory_xact_lock(724391822);
-- name: CreateTool :exec
INSERT INTO tools (id,connection_id,slug,display_name,description,method,url_template,params,public_headers,secret_headers_ciphertext,secret_header_names,timeout_seconds,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14);
-- name: GetTool :one
SELECT * FROM tools WHERE id=$1;
-- name: GetToolForUpdate :one
SELECT * FROM tools WHERE id=$1 FOR UPDATE;
-- name: ListTools :many
SELECT * FROM tools WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: ListAllTools :many
SELECT * FROM tools ORDER BY slug;
-- name: UpdateTool :execrows
UPDATE tools SET connection_id=$2,slug=$3,display_name=$4,description=$5,method=$6,url_template=$7,params=$8,public_headers=$9,secret_headers_ciphertext=$10,secret_header_names=$11,timeout_seconds=$12,updated_at=$13
WHERE id=$1;
-- name: DeleteTool :execrows
DELETE FROM tools WHERE id=$1;

-- name: CreateMCPServer :exec
INSERT INTO mcp_servers (id,slug,display_name,url,secret_headers_ciphertext,secret_header_names,allowed_tools,tools_cache,status,last_error,last_synced_at,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13);
-- name: GetMCPServer :one
SELECT * FROM mcp_servers WHERE id=$1;
-- name: GetMCPServerForUpdate :one
SELECT * FROM mcp_servers WHERE id=$1 FOR UPDATE;
-- name: ListMCPServers :many
SELECT * FROM mcp_servers WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;
-- name: UpdateMCPServer :one
UPDATE mcp_servers SET slug=$2,display_name=$3,url=$4,secret_headers_ciphertext=$5,secret_header_names=$6,allowed_tools=$7,tools_cache=$8,status=$9,last_error=$10,last_synced_at=$11,updated_at=$12,revision=revision+1
WHERE id=$1 RETURNING *;
-- name: DeleteMCPServer :execrows
DELETE FROM mcp_servers WHERE id=$1;
-- name: RecordMCPServerSync :execrows
UPDATE mcp_servers SET tools_cache=$3,status=$4,last_error=$5,last_synced_at=$6,updated_at=$6
WHERE id=$1 AND revision=$2;

-- name: ListAgentToolIDs :many
SELECT tool_id FROM agent_tools WHERE agent_id=$1 ORDER BY tool_id;
-- name: ListAgentMCPServerIDs :many
SELECT mcp_server_id FROM agent_mcp_servers WHERE agent_id=$1 ORDER BY mcp_server_id;
-- name: DeleteAgentTools :exec
DELETE FROM agent_tools WHERE agent_id=$1;
-- name: DeleteAgentMCPServers :exec
DELETE FROM agent_mcp_servers WHERE agent_id=$1;
-- name: AddAgentTool :exec
INSERT INTO agent_tools (agent_id,tool_id) VALUES ($1,$2);
-- name: AddAgentMCPServer :exec
INSERT INTO agent_mcp_servers (agent_id,mcp_server_id) VALUES ($1,$2);
-- name: ResolveAgentHTTPTools :many
SELECT
    t.*,
    c.id AS api_connection_id,
    c.slug AS api_connection_slug,
    c.display_name AS api_connection_display_name,
    c.base_url AS api_connection_base_url,
    c.public_headers AS api_connection_public_headers,
    c.secret_headers_ciphertext AS api_connection_secret_headers_ciphertext,
    c.secret_header_names AS api_connection_secret_header_names,
    c.created_at AS api_connection_created_at,
    c.updated_at AS api_connection_updated_at
FROM tools t
JOIN agent_tools b ON b.tool_id=t.id
LEFT JOIN api_connections c ON c.id=t.connection_id
WHERE b.agent_id=sqlc.arg(agent_id)
ORDER BY t.slug,t.id;
-- name: ResolveAgentMCPServers :many
SELECT s.* FROM mcp_servers s JOIN agent_mcp_servers b ON b.mcp_server_id=s.id WHERE b.agent_id=sqlc.arg(agent_id) ORDER BY s.slug,s.id;
