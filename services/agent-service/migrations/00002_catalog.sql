-- +goose Up
CREATE TABLE llm_providers (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    kind text NOT NULL CHECK (kind IN ('gemini', 'openai_compatible')),
    base_url text,
    api_key_ciphertext bytea,
    api_key_hint text,
    default_model text,
    status text NOT NULL CHECK (status IN ('unchecked', 'ok', 'failing')),
    last_error jsonb,
    last_checked_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    CHECK (kind <> 'openai_compatible' OR (base_url IS NOT NULL AND length(base_url) > 0))
);
CREATE INDEX llm_providers_page ON llm_providers (created_at DESC, id DESC);
CREATE TABLE agents (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL,
    provider_id uuid REFERENCES llm_providers(id) ON DELETE SET NULL,
    model text NOT NULL,
    system_prompt text NOT NULL,
    temperature double precision CHECK (temperature >= 0 AND temperature <= 2),
    max_output_tokens integer CHECK (max_output_tokens > 0),
    max_iterations integer NOT NULL CHECK (max_iterations BETWEEN 1 AND 25),
    timeout_seconds integer NOT NULL CHECK (timeout_seconds BETWEEN 10 AND 600),
    created_by uuid NOT NULL REFERENCES users(id),
    archived_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (provider_id IS NOT NULL OR archived_at IS NOT NULL)
);
CREATE UNIQUE INDEX agents_active_name ON agents (name) WHERE archived_at IS NULL;
CREATE INDEX agents_active_page ON agents (created_at DESC, id DESC) WHERE archived_at IS NULL;
CREATE INDEX agents_provider ON agents (provider_id);

-- +goose Down
DROP TABLE agents;
DROP TABLE llm_providers;
