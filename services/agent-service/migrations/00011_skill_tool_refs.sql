-- +goose Up
-- Skills imported before this column keep an empty list rather than being backfilled.
-- Deriving references in SQL would mean a second copy of the reference grammar that
-- nothing keeps in step with the Go parser, and it would read the whole document
-- including YAML frontmatter and fenced examples, so it would invent references the
-- parser never produces. Re-importing a skill fills the column from the one parser.
ALTER TABLE skills ADD COLUMN tool_refs text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE skills DROP COLUMN tool_refs;
