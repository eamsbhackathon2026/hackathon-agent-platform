-- +goose Up
-- The uploaded file is kept apart from skills so the many readers of skill rows (the
-- library list, the run-time resolver) never pull package bytes they do not need.
-- Skills imported before this table have no row and download as their stored SKILL.md.
CREATE TABLE skill_source_files (
    skill_id uuid PRIMARY KEY REFERENCES skills(id) ON DELETE CASCADE,
    content bytea NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 2097152)
);

-- +goose Down
DROP TABLE skill_source_files;
