-- +goose Up
-- Ask the provider for a summary of its thinking and emit it while the run is in
-- flight. Off by default: it spends extra output tokens, and not every product
-- wants to tell an end user what the assistant is thinking. Existing agents keep
-- their current behaviour.
ALTER TABLE agents ADD COLUMN show_thinking boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE agents DROP COLUMN show_thinking;
