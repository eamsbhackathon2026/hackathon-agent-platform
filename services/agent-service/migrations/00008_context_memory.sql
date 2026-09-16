-- +goose Up
ALTER TABLE agents
ADD COLUMN context_window_tokens integer;

-- Older releases did not bound the response against a model context window.
-- Repair impossible outliers, then choose the smallest safe window that preserves
-- the configured response cap while leaving 10% safety and 1K input headroom.
UPDATE agents
SET max_output_tokens = 1798976
WHERE max_output_tokens > 1798976;

UPDATE agents
SET context_window_tokens = LEAST(
  2000000,
  GREATEST(32768, ((COALESCE(max_output_tokens, 0) + 1024) * 10 + 8) / 9)
);

ALTER TABLE agents
ALTER COLUMN context_window_tokens SET DEFAULT 32768,
ALTER COLUMN context_window_tokens SET NOT NULL,
ADD CONSTRAINT agents_context_window_bounds
  CHECK (context_window_tokens BETWEEN 8192 AND 2000000),
ADD CONSTRAINT agents_context_budget
  CHECK (
    max_output_tokens IS NULL OR
    max_output_tokens <= context_window_tokens - GREATEST(context_window_tokens / 10, 512) - 1024
  );

ALTER TABLE sessions
ADD COLUMN last_prompt_estimated_tokens bigint,
ADD COLUMN last_prompt_tokens bigint,
ADD CONSTRAINT sessions_prompt_estimate_nonnegative
  CHECK (last_prompt_estimated_tokens IS NULL OR last_prompt_estimated_tokens > 0),
ADD CONSTRAINT sessions_prompt_tokens_nonnegative
  CHECK (last_prompt_tokens IS NULL OR last_prompt_tokens >= 0),
ADD CONSTRAINT sessions_prompt_calibration_pair
  CHECK ((last_prompt_estimated_tokens IS NULL) = (last_prompt_tokens IS NULL));

CREATE TABLE session_context_snapshots (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    source_run_id uuid REFERENCES runs(id) ON DELETE SET NULL,
    version bigint NOT NULL CHECK (version > 0),
    covered_through_seq bigint NOT NULL CHECK (covered_through_seq > 0),
    summary text NOT NULL CHECK (length(summary) > 0),
    input_tokens bigint CHECK (input_tokens IS NULL OR input_tokens >= 0),
    output_tokens bigint CHECK (output_tokens IS NULL OR output_tokens >= 0),
    created_at timestamptz NOT NULL,
    UNIQUE (session_id, version),
    UNIQUE (session_id, covered_through_seq)
);
CREATE INDEX session_context_snapshots_latest
ON session_context_snapshots (session_id, version DESC);

-- +goose Down
DROP TABLE session_context_snapshots;
ALTER TABLE sessions
DROP CONSTRAINT sessions_prompt_calibration_pair,
DROP CONSTRAINT sessions_prompt_tokens_nonnegative,
DROP CONSTRAINT sessions_prompt_estimate_nonnegative,
DROP COLUMN last_prompt_tokens,
DROP COLUMN last_prompt_estimated_tokens;
ALTER TABLE agents
DROP CONSTRAINT IF EXISTS agents_context_budget,
DROP CONSTRAINT IF EXISTS agents_context_window_bounds,
DROP COLUMN context_window_tokens;
