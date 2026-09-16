-- +goose Up
CREATE TABLE skills (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    source_type text NOT NULL CHECK (source_type IN ('markdown', 'zip')),
    source_filename text NOT NULL,
    content text NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 102400),
    checksum text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX skills_name_ci ON skills (lower(name));
CREATE INDEX skills_page ON skills (created_at DESC, id DESC);

CREATE TABLE agent_skills (
    agent_id uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id uuid NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, skill_id)
);
CREATE INDEX agent_skills_skill ON agent_skills (skill_id);

-- +goose Down
DROP TABLE agent_skills;
DROP TABLE skills;
