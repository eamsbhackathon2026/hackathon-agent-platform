-- name: LockSkills :exec
SELECT pg_advisory_xact_lock(724391823);

-- name: CreateSkill :exec
INSERT INTO skills (id,name,description,source_type,source_filename,content,checksum,tool_refs,created_by,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);

-- name: CreateSkillSourceFile :exec
INSERT INTO skill_source_files (skill_id,content) VALUES ($1,$2);

-- name: GetSkill :one
SELECT sqlc.embed(skills),
  EXISTS(SELECT 1 FROM skill_source_files f WHERE f.skill_id=skills.id)::boolean AS source_file_available
FROM skills WHERE id=$1;

-- name: GetSkillSourceFile :one
SELECT content FROM skill_source_files WHERE skill_id=$1;

-- name: ListSkills :many
SELECT id,name,description,source_type,source_filename,checksum,tool_refs,created_by,created_at,updated_at,
  EXISTS(SELECT 1 FROM skill_source_files f WHERE f.skill_id=skills.id)::boolean AS source_file_available
FROM skills
WHERE (sqlc.narg(before_time)::timestamptz IS NULL OR (created_at,id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT $1;

-- name: DeleteSkill :execrows
DELETE FROM skills WHERE id=$1;

-- name: ListAgentSkillIDs :many
SELECT b.skill_id FROM agent_skills b JOIN skills s ON s.id=b.skill_id
WHERE b.agent_id=$1 ORDER BY lower(s.name),s.id;

-- name: DeleteAgentSkills :exec
DELETE FROM agent_skills WHERE agent_id=$1;

-- name: AddAgentSkill :exec
INSERT INTO agent_skills (agent_id,skill_id) VALUES ($1,$2);

-- name: ResolveAgentSkills :many
SELECT s.* FROM skills s JOIN agent_skills b ON b.skill_id=s.id
WHERE b.agent_id=sqlc.arg(agent_id) ORDER BY lower(s.name),s.id;
