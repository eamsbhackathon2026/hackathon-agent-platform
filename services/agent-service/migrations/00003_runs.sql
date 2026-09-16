-- +goose Up
CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    agent_id uuid NOT NULL REFERENCES agents(id),
    source text NOT NULL CHECK (source IN ('playground', 'api')),
    created_by_user_id uuid REFERENCES users(id),
    created_by_api_key_id uuid REFERENCES api_keys(id),
    external_key text,
    title text NOT NULL,
    next_message_seq bigint NOT NULL DEFAULT 1 CHECK (next_message_seq > 0),
    deleted_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((created_by_user_id IS NOT NULL)::integer + (created_by_api_key_id IS NOT NULL)::integer = 1),
    CHECK (external_key IS NULL OR (created_by_api_key_id IS NOT NULL AND length(external_key) > 0))
);
CREATE INDEX sessions_page ON sessions (created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX sessions_agent ON sessions (agent_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX sessions_external_key_active ON sessions (created_by_api_key_id, external_key)
WHERE deleted_at IS NULL;

CREATE TABLE runs (
    id uuid PRIMARY KEY,
    agent_id uuid NOT NULL REFERENCES agents(id),
    session_id uuid NOT NULL REFERENCES sessions(id),
    mode text NOT NULL CHECK (mode IN ('sync', 'stream', 'async')),
    status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    source text NOT NULL CHECK (source IN ('playground', 'api')),
    triggered_by_user_id uuid REFERENCES users(id),
    triggered_by_api_key_id uuid REFERENCES api_keys(id),
    input jsonb NOT NULL,
    output text,
    error_code text,
    error_message text,
    iterations integer NOT NULL DEFAULT 0 CHECK (iterations BETWEEN 0 AND 25),
    input_tokens bigint,
    output_tokens bigint,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    cancel_requested_at timestamptz,
    queued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK ((triggered_by_user_id IS NOT NULL)::integer + (triggered_by_api_key_id IS NOT NULL)::integer = 1),
    CHECK ((error_code IS NULL) = (error_message IS NULL)),
    CHECK (input_tokens IS NULL OR input_tokens >= 0),
    CHECK (output_tokens IS NULL OR output_tokens >= 0)
);
CREATE INDEX runs_page ON runs (created_at DESC, id DESC);
CREATE INDEX runs_agent_status ON runs (agent_id, status);
CREATE INDEX runs_session_status ON runs (session_id, status);
CREATE INDEX runs_user_owner ON runs (triggered_by_user_id, created_at DESC, id DESC);
CREATE INDEX runs_api_key_owner ON runs (triggered_by_api_key_id, created_at DESC, id DESC);

CREATE TABLE messages (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES sessions(id),
    run_id uuid REFERENCES runs(id),
    seq bigint NOT NULL CHECK (seq > 0),
    role text NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content text NOT NULL,
    tool_calls jsonb NOT NULL DEFAULT '[]'::jsonb,
    tool_call_id text,
    tool_name text,
    provider_meta jsonb,
    is_error boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    UNIQUE (session_id, seq),
    CHECK ((role = 'tool') = (tool_call_id IS NOT NULL AND tool_name IS NOT NULL)),
    CHECK (role = 'assistant' OR jsonb_array_length(tool_calls) = 0),
    CHECK (role = 'tool' OR NOT is_error)
);
CREATE INDEX messages_session_seq ON messages (session_id, seq, id);
CREATE INDEX messages_run ON messages (run_id);

CREATE TABLE spans (
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES runs(id),
    parent_span_id uuid,
    kind text NOT NULL CHECK (kind IN ('run', 'llm_call', 'tool_call')),
    name text NOT NULL,
    status text NOT NULL CHECK (status IN ('ok', 'error')),
    model text,
    tool_name text,
    input_tokens bigint,
    output_tokens bigint,
    started_at timestamptz NOT NULL,
    ended_at timestamptz NOT NULL,
    duration_ms bigint NOT NULL CHECK (duration_ms >= 0),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_message text,
    CHECK (ended_at >= started_at),
    CHECK (input_tokens IS NULL OR input_tokens >= 0),
    CHECK (output_tokens IS NULL OR output_tokens >= 0)
);
CREATE INDEX spans_run_started ON spans (run_id, started_at, id);

-- +goose Down
DROP TABLE spans;
DROP TABLE messages;
DROP TABLE runs;
DROP TABLE sessions;
