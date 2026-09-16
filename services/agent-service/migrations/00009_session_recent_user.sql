-- +goose Up
CREATE INDEX sessions_user_source_updated
ON sessions (created_by_user_id, source, updated_at DESC, id DESC)
WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX sessions_user_source_updated;
