-- +goose Up
CREATE TABLE tools (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    display_name text NOT NULL,
    description text NOT NULL,
    kind text NOT NULL DEFAULT 'http' CHECK (kind = 'http'),
    method text NOT NULL CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    url_template text NOT NULL,
    params jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(params) = 'array'),
    public_headers jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(public_headers) = 'object'),
    secret_headers_ciphertext bytea,
    secret_header_names text[] NOT NULL DEFAULT '{}',
    timeout_seconds integer NOT NULL DEFAULT 15 CHECK (timeout_seconds BETWEEN 1 AND 60),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX tools_page ON tools (created_at DESC, id DESC);

CREATE TABLE mcp_servers (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    display_name text NOT NULL,
    url text NOT NULL,
    secret_headers_ciphertext bytea,
    secret_header_names text[] NOT NULL DEFAULT '{}',
    allowed_tools text[],
    tools_cache jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tools_cache) = 'array'),
    status text NOT NULL CHECK (status IN ('unchecked', 'ok', 'failing')),
    last_error jsonb,
    last_synced_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0)
);
CREATE INDEX mcp_servers_page ON mcp_servers (created_at DESC, id DESC);

CREATE TABLE agent_tools (
    agent_id uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_id uuid NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, tool_id)
);

CREATE TABLE agent_mcp_servers (
    agent_id uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id uuid NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, mcp_server_id)
);

-- +goose Down
DROP TABLE agent_mcp_servers;
DROP TABLE agent_tools;
DROP TABLE mcp_servers;
DROP TABLE tools;
