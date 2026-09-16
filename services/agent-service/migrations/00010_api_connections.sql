-- +goose Up
CREATE TABLE api_connections (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    display_name text NOT NULL,
    base_url text NOT NULL,
    public_headers jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(public_headers) = 'object'),
    secret_headers_ciphertext bytea,
    secret_header_names text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX api_connections_page ON api_connections (created_at DESC, id DESC);

ALTER TABLE tools
    ADD COLUMN connection_id uuid REFERENCES api_connections(id) ON DELETE RESTRICT;
CREATE INDEX tools_connection_id ON tools (connection_id) WHERE connection_id IS NOT NULL;

-- +goose Down
ALTER TABLE tools DROP COLUMN connection_id;
DROP TABLE api_connections;
