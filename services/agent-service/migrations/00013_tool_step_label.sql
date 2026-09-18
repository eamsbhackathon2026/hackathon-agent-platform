-- +goose Up
-- The label an end user reads while the assistant runs this tool. Separate from
-- display_name because the two are read in different places: display_name names
-- the tool in an operator's catalog, while this one passes a customer's eyes in
-- the middle of an answer taking shape, so it stays short and in the present
-- tense. Existing tools keep an empty string and fall back to display_name, so
-- nothing needs backfilling.
ALTER TABLE tools ADD COLUMN step_label text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tools DROP COLUMN step_label;
